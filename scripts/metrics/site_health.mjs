// Site runtime health: crawls every same-origin page reachable from the
// sitemap and from links, in headless Chrome over the DevTools protocol (no
// npm dependencies, the same approach as scripts/site_audit.mjs), and prints
// one treemand-measure probe result as JSON on stdout.
//
//   BASE=http://127.0.0.1:PORT/ CHROME=/path/to/chrome node site_health.mjs
//
// Run it through `task measure -- site-health` (scripts/metrics/probes/
// site-health.sh), which builds and serves the site, or targets production
// with SITE_BASE=https://treemand.dev/.
//
// Per page it records the HTTP status, console errors, uncaught exceptions,
// failed subresource requests, mixed content and broken links. Every link
// target is checked once per run: same-origin ones directly, external ones
// with HEAD (GET when HEAD is refused) under a short timeout, at most
// EXTERNAL_CONCURRENCY at a time and one request per host per HOST_GAP_MS.
import { spawn } from "node:child_process";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

const BASE = process.env.BASE;
const CHROME = process.env.CHROME;
if (!BASE || !CHROME) {
  console.error("site_health: set BASE and CHROME");
  process.exit(2);
}
const origin = new URL(BASE).origin;
// Every metric carries the target as its subject (http_errors.production), so
// local and production runs share one history without judging each other.
const TARGET = process.env.TARGET || "local";
const MAX_PAGES = Number(process.env.SITE_HEALTH_MAX_PAGES || 500);
const LINK_TIMEOUT_MS = Number(process.env.SITE_HEALTH_LINK_TIMEOUT_MS || 8000);
const EXTERNAL_CONCURRENCY = 5;
const HOST_GAP_MS = 250;
const UA = "treemand-site-health/1 (+https://treemand.dev/)";
// Sites that refuse automated clients answer these; the link may be fine, so
// it is reported as unverifiable rather than broken.
const UNVERIFIABLE = new Set([401, 403, 429, 999]);

const log = (...a) => console.error("site_health:", ...a);
const strip = (u) => { const x = new URL(u); x.hash = ""; return x.href; };
const sameOrigin = (u) => { try { return new URL(u).origin === origin; } catch { return false; } };

// ── Chrome and the DevTools protocol ────────────────────────────────────────
const profile = mkdtempSync(join(tmpdir(), "site-health-"));
const chrome = spawn(CHROME, [
  "--headless=new", "--no-sandbox", "--disable-gpu", "--remote-debugging-port=0",
  `--user-data-dir=${profile}`, "about:blank",
], { stdio: ["ignore", "ignore", "pipe"] });
const removeProfile = () => {
  try { rmSync(profile, { recursive: true, force: true, maxRetries: 10, retryDelay: 200 }); } catch { /* best effort */ }
};
const chromeExited = new Promise((resolve) => chrome.once("exit", resolve));
async function cleanup() {
  chrome.kill();
  await Promise.race([chromeExited, new Promise((r) => setTimeout(r, 5000))]);
  removeProfile();
}
process.on("exit", () => { chrome.kill("SIGKILL"); removeProfile(); });

const port = await new Promise((resolve, reject) => {
  let buf = "";
  chrome.stderr.on("data", (d) => {
    buf += d;
    const m = buf.match(/DevTools listening on ws:\/\/127\.0\.0\.1:(\d+)\//);
    if (m) resolve(m[1]);
  });
  chrome.on("exit", () => reject(new Error("chrome exited early")));
  setTimeout(() => reject(new Error("chrome did not start")), 20000);
});

async function newPage() {
  const res = await fetch(`http://127.0.0.1:${port}/json/new?about:blank`, { method: "PUT" });
  const target = await res.json();
  const ws = new WebSocket(target.webSocketDebuggerUrl);
  await new Promise((r) => ws.addEventListener("open", r, { once: true }));
  let id = 0;
  const pending = new Map();
  const listeners = [];
  ws.addEventListener("message", (e) => {
    const msg = JSON.parse(e.data);
    if (msg.id && pending.has(msg.id)) {
      const { resolve, reject } = pending.get(msg.id);
      pending.delete(msg.id);
      msg.error ? reject(new Error(msg.error.message)) : resolve(msg.result);
    } else if (msg.method) listeners.forEach((l) => l(msg.method, msg.params));
  });
  return {
    id: target.id,
    send: (method, params = {}) => new Promise((resolve, reject) => {
      const n = ++id;
      pending.set(n, { resolve, reject });
      ws.send(JSON.stringify({ id: n, method, params }));
    }),
    on: (fn) => listeners.push(fn),
    close: async () => {
      ws.close();
      await fetch(`http://127.0.0.1:${port}/json/close/${target.id}`).catch(() => {});
    },
  };
}

// ── Link checking (cached for the run) ──────────────────────────────────────
const linkCache = new Map(); // url without hash -> Promise<{status, ok, unverifiable, type, final, error}>
const hostNext = new Map(); // host -> earliest time of its next request
let active = 0;
const waiting = [];
const slot = async () => {
  while (active >= EXTERNAL_CONCURRENCY) await new Promise((r) => waiting.push(r));
  active++;
};
const release = () => { active--; waiting.shift()?.(); };

async function request(url, method) {
  const ctl = new AbortController();
  const t = setTimeout(() => ctl.abort(), LINK_TIMEOUT_MS);
  try {
    const res = await fetch(url, { method, redirect: "follow", signal: ctl.signal, headers: { "user-agent": UA } });
    if (method === "GET") res.body?.cancel().catch(() => {});
    return { status: res.status, type: res.headers.get("content-type") || "", final: res.url };
  } catch (e) {
    return { status: 0, error: ctl.signal.aborted ? `timeout after ${LINK_TIMEOUT_MS}ms` : (e.cause?.code || e.message) };
  } finally {
    clearTimeout(t);
  }
}

async function probeLink(url) {
  let r = await request(url, "HEAD");
  // Many servers refuse or mishandle HEAD; GET is the real reader's request.
  if (r.status === 0 || r.status >= 400) r = await request(url, "GET");
  const unverifiable = UNVERIFIABLE.has(r.status);
  return { ...r, ok: r.status >= 200 && r.status < 400, unverifiable };
}

function checkLink(url) {
  const key = strip(url);
  if (!linkCache.has(key)) {
    if (sameOrigin(key)) linkCache.set(key, probeLink(key));
    else linkCache.set(key, (async () => {
      await slot();
      try {
        const host = new URL(key).host;
        const now = Date.now();
        const at = Math.max(now, hostNext.get(host) || 0);
        hostNext.set(host, at + HOST_GAP_MS);
        if (at > now) await new Promise((r) => setTimeout(r, at - now));
        return await probeLink(key);
      } finally { release(); }
    })());
  }
  return linkCache.get(key);
}

// ── One page in Chrome ──────────────────────────────────────────────────────
const collect = () => ({
  links: [...document.querySelectorAll("a[href]")].map((a) => a.href),
  ids: [...document.querySelectorAll("[id], a[name]")].map((e) => e.id || e.getAttribute("name")),
  resources: [...document.querySelectorAll("img[src], script[src], link[href], iframe[src], source[src], video[src], audio[src]")]
    .filter((e) => e.tagName !== "LINK" || /stylesheet|icon|preload|modulepreload|manifest/.test(e.rel))
    .map((e) => e.src || e.href),
});

async function visit(url) {
  const page = await newPage();
  const rec = { url, status: 0, console_errors: [], js_exceptions: [], failed_requests: [], mixed_content: [], broken_links: [] };
  const reqUrl = new Map();
  const inflight = new Set();
  let lastActivity = Date.now();
  let mainRequest = "";
  let loaded;
  const loadFired = new Promise((r) => { loaded = r; });
  const mixed = new Set();
  const failed = new Set();
  const isHttps = url.startsWith("https:");
  page.on((method, p) => {
    if (method === "Network.requestWillBeSent") {
      reqUrl.set(p.requestId, p.request.url);
      inflight.add(p.requestId);
      lastActivity = Date.now();
      if ((p.request.mixedContentType && p.request.mixedContentType !== "none") || (isHttps && p.request.url.startsWith("http:")))
        mixed.add(p.request.url);
    }
    if (method === "Network.loadingFinished") { inflight.delete(p.requestId); lastActivity = Date.now(); }
    if (method === "Network.responseReceived") {
      // The main frame's id is the target's id; iframes are subresources.
      if (p.type === "Document" && p.frameId === page.id) { rec.status = p.response.status; mainRequest = p.requestId; }
      else if (p.response.status >= 400) failed.add(`${p.response.status} ${p.response.url}`);
    }
    if (method === "Network.loadingFailed") {
      inflight.delete(p.requestId);
      lastActivity = Date.now();
      if (!p.canceled && p.requestId !== mainRequest) failed.add(`failed ${p.errorText} ${reqUrl.get(p.requestId) || ""}`);
    }
    if (method === "Runtime.exceptionThrown") {
      const d = p.exceptionDetails;
      rec.js_exceptions.push(`${d.exception?.description || d.text}`.split("\n")[0] + (d.url ? ` (${d.url}:${d.lineNumber + 1})` : ""));
    }
    if (method === "Runtime.consoleAPICalled" && (p.type === "error" || p.type === "assert"))
      rec.console_errors.push(p.args.map((a) => a.value ?? a.description ?? "").join(" ").slice(0, 300));
    // Network failures are logged here too ("Failed to load resource"); they
    // are counted once, as failed requests, not again as console errors.
    if (method === "Log.entryAdded" && p.entry.level === "error" && p.entry.source !== "network" && p.entry.source !== "console-api")
      rec.console_errors.push(`${p.entry.text}`.slice(0, 300));
    if (method === "Page.loadEventFired") loaded();
  });
  await page.send("Network.enable");
  await page.send("Log.enable");
  await page.send("Runtime.enable");
  await page.send("Page.enable");
  const nav = await page.send("Page.navigate", { url });
  if (nav.errorText) rec.navigation_error = nav.errorText;
  await Promise.race([loadFired, new Promise((r) => setTimeout(r, 20000))]);
  // Settle: wait for the network to go quiet (wasm, search index, late scripts).
  const deadline = Date.now() + 10000;
  while (Date.now() < deadline && (inflight.size > 0 || Date.now() - lastActivity < 750))
    await new Promise((r) => setTimeout(r, 100));
  const found = (await page.send("Runtime.evaluate", { expression: `(${collect.toString()})()`, returnByValue: true })).result.value
    || { links: [], ids: [], resources: [] };
  await page.close();
  // A cross-origin http: resource is mixed content wherever the site is served
  // over https, so it is caught on a local http run too.
  for (const u of found.resources) if (u.startsWith("http:") && !sameOrigin(u)) mixed.add(u);
  rec.failed_requests = [...failed];
  rec.mixed_content = [...mixed];
  rec.ids = found.ids;
  rec.links = [...new Set(found.links)].filter((u) => /^https?:/.test(u));
  return rec;
}

// ── Crawl ───────────────────────────────────────────────────────────────────
const seeds = [new URL(BASE).href];
try {
  const sm = await fetch(new URL("sitemap.xml", BASE), { headers: { "user-agent": UA } });
  if (sm.ok) for (const m of (await sm.text()).matchAll(/<loc>\s*([^<\s]+)\s*<\/loc>/g)) {
    // The sitemap names production URLs; crawl the same paths on BASE.
    const u = new URL(m[1]);
    seeds.push(new URL(u.pathname + u.search, origin).href);
  }
  else log(`sitemap.xml returned ${sm.status}`);
} catch (e) { log(`sitemap.xml: ${e.message}`); }

const pages = new Map(); // final url -> record
const queued = new Set();
const queue = [];
const enqueue = (u) => { const k = strip(u); if (sameOrigin(k) && !queued.has(k)) { queued.add(k); queue.push(k); } };
seeds.forEach(enqueue);
const skippedPages = [];

while (queue.length) {
  const u = queue.shift();
  const head = await checkLink(u);
  if (!head.ok || !/text\/html/.test(head.type)) continue; // broken targets are reported by the linking page
  const final = strip(head.final || u);
  if (pages.has(final)) continue;
  if (pages.size >= MAX_PAGES) { skippedPages.push(final); continue; }
  log(`page ${pages.size + 1}: ${new URL(final).pathname}`);
  const rec = await visit(final);
  pages.set(final, rec);
  for (const l of rec.links) enqueue(l);
}

// Seeds that are not pages (a sitemap entry that 404s) are http errors too.
const seedErrors = [];
for (const s of new Set(seeds)) {
  const r = await checkLink(s);
  if (!r.ok) seedErrors.push({ url: s, status: r.status, error: r.error });
}

// ── Links ───────────────────────────────────────────────────────────────────
const idsOf = new Map([...pages.values()].map((p) => [p.url, new Set(p.ids)]));
let brokenInternal = 0;
let brokenExternal = 0;
const unverifiable = new Map();
await Promise.all([...pages.values()].map(async (p) => {
  for (const l of p.links) {
    const r = await checkLink(l);
    const internal = sameOrigin(l);
    if (r.unverifiable && !internal) { unverifiable.set(strip(l), r.status); continue; }
    if (!r.ok) {
      p.broken_links.push({ href: l, internal, status: r.status, ...(r.error ? { error: r.error } : {}) });
      internal ? brokenInternal++ : brokenExternal++;
      continue;
    }
    const hash = decodeURIComponent(new URL(l).hash.slice(1));
    if (internal && hash) {
      const ids = idsOf.get(strip(r.final || l));
      if (ids && !ids.has(hash)) {
        p.broken_links.push({ href: l, internal, status: r.status, error: `no element with id "${hash}"` });
        brokenInternal++;
      }
    }
  }
}));

// ── Result ──────────────────────────────────────────────────────────────────
const recs = [...pages.values()];
const httpPageErrors = recs.filter((p) => p.status >= 400 || p.status === 0).length + seedErrors.length;
const sum = (f) => recs.reduce((n, p) => n + f(p), 0);
const counts = {
  pages: recs.length,
  http_errors: httpPageErrors + sum((p) => p.failed_requests.length),
  console_errors: sum((p) => p.console_errors.length),
  js_exceptions: sum((p) => p.js_exceptions.length),
  broken_internal_links: brokenInternal,
  broken_external_links: brokenExternal,
  mixed_content: sum((p) => p.mixed_content.length),
};
const metrics = Object.fromEntries(Object.entries(counts).map(([k, v]) => [`${k}.${TARGET}`, v]));
const path = (u) => { const x = new URL(u); return x.origin === origin ? x.pathname + x.search + x.hash : u; };
const problemPages = recs
  .filter((p) => p.status >= 400 || p.console_errors.length || p.js_exceptions.length || p.failed_requests.length || p.mixed_content.length || p.broken_links.length)
  .map(({ ids, links, url, ...rest }) => ({ page: path(url), ...rest, broken_links: rest.broken_links.map((b) => ({ ...b, href: path(b.href) })) }));
const external = {};
for (const u of [...linkCache.keys()].filter((k) => !sameOrigin(k)).sort()) external[u] = (await linkCache.get(u)).status;
const details = [
  {
    target: TARGET,
    base: BASE,
    pages: recs.map((p) => path(p.url)),
    links_checked: linkCache.size,
    external_links: external, // url -> final HTTP status (0: network error)
    ...(seedErrors.length ? { sitemap_errors: seedErrors.map((s) => ({ ...s, url: path(s.url) })) } : {}),
    ...(unverifiable.size ? { unverifiable_external: Object.fromEntries(unverifiable) } : {}),
    ...(skippedPages.length ? { not_crawled_over_max_pages: skippedPages.length } : {}),
  },
  ...problemPages,
];
console.log(JSON.stringify({ probe: "site-health", metrics, details }, null, 1));
await cleanup();
process.exit(0);
