// Site quality: Lighthouse category scores, axe-core accessibility violations,
// cumulative layout shift and transferred page weight for a few key pages of
// treemand.dev, at mobile and desktop sizes. Prints one treemand-measure probe
// result as JSON on stdout; progress goes to stderr.
//
//   BASE=http://127.0.0.1:PORT/ CHROME=/path/to/chrome node site_quality.mjs
//
// Run it through `task measure -- site-quality` (scripts/metrics/probes/
// site-quality.sh), which builds and serves the site, or targets production
// with SITE_BASE=https://treemand.dev/.
//
// Two passes, one after the other so they never compete for the CPU:
// 1. Chrome over the DevTools protocol (scripts/site/cdp.mjs), one cold load
//    per page and form factor: bytes transferred, CLS (the largest session
//    window, as Core Web Vitals defines it), then axe-core injected into the
//    loaded page. axe-core is fetched once from a pinned npm version and
//    checked against a pinned SHA-256; the site itself gains no dependency.
// 2. Lighthouse, a pinned version run with npx against the same Chrome
//    binary, RUNS times per page and form factor; each category's median
//    score (0-100) is the metric.
//
// Environment:
//   TARGET          metric subject prefix (local, production); default local
//   PAGES           "name=/path/ ..." ; default home, installation and try
//   RUNS            Lighthouse runs per page and form factor; default 3
//   LIGHTHOUSE      the command that runs Lighthouse; "off" skips it
//   AXE_JS          an axe.min.js to use instead of the pinned download
import { spawn, execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import { existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { homedir, tmpdir } from "node:os";
import { join } from "node:path";
import { launchChrome } from "../site/cdp.mjs";

const LIGHTHOUSE_VERSION = "13.6.0";
const AXE_VERSION = "4.14.0";
const AXE_SHA256 = "20c09fe157a8a34a30e241aaa1fcdade657734f08ab379ecfbeb7d45cc46e878";

const BASE = process.env.BASE;
const CHROME = process.env.CHROME;
if (!BASE || !CHROME) {
  console.error("site_quality: set BASE and CHROME");
  process.exit(2);
}
const TARGET = process.env.TARGET || "local";
const RUNS = Math.max(1, Number(process.env.RUNS || 3));
const LIGHTHOUSE = process.env.LIGHTHOUSE || `npx --yes lighthouse@${LIGHTHOUSE_VERSION}`;
const PAGES = (process.env.PAGES || "home=/ installation=/installation/ try=/try/")
  .trim().split(/\s+/).map((p) => { const [name, path] = p.split("="); return { name, url: new URL(path, BASE).href }; });
// Lighthouse's own emulation for each form factor (a Moto G Power, and its
// desktop preset), so both passes see the same viewport.
const FORM_FACTORS = [
  { name: "mobile", width: 412, height: 823, deviceScaleFactor: 1.75, mobile: true, lighthouse: [] },
  { name: "desktop", width: 1350, height: 940, deviceScaleFactor: 1, mobile: false, lighthouse: ["--preset=desktop"] },
];
const CATEGORIES = { performance: "lh_performance", accessibility: "lh_accessibility", "best-practices": "lh_best_practices", seo: "lh_seo" };
const IMPACTS = ["critical", "serious", "moderate", "minor"];

const log = (...a) => console.error("site_quality:", ...a);
const median = (xs) => { const s = [...xs].sort((a, b) => a - b); const m = s.length >> 1; return s.length % 2 ? s[m] : (s[m - 1] + s[m]) / 2; };
const pathOf = (u) => { const x = new URL(u); return x.origin === new URL(BASE).origin ? x.pathname + x.search : u; };

// ── axe-core, pinned ────────────────────────────────────────────────────────
function axeSource() {
  if (process.env.AXE_JS) return readFileSync(process.env.AXE_JS, "utf8");
  const cache = join(process.env.XDG_CACHE_HOME || join(homedir(), ".cache"), "treemand");
  const file = join(cache, `axe-core-${AXE_VERSION}.min.js`);
  const ok = (buf) => createHash("sha256").update(buf).digest("hex") === AXE_SHA256;
  if (existsSync(file) && ok(readFileSync(file))) return readFileSync(file, "utf8");
  log(`fetching axe-core@${AXE_VERSION}`);
  const work = mkdtempSync(join(tmpdir(), "site-quality-axe-"));
  try {
    // npm verifies the tarball against the registry's integrity hash.
    const tgz = execFileSync("npm", ["pack", `axe-core@${AXE_VERSION}`, "--silent", "--pack-destination", work], { encoding: "utf8" }).trim().split("\n").pop();
    execFileSync("tar", ["-xzf", join(work, tgz), "-C", work, "package/axe.min.js"]);
    const buf = readFileSync(join(work, "package", "axe.min.js"));
    if (!ok(buf)) throw new Error(`axe-core@${AXE_VERSION} does not match its pinned SHA-256`);
    mkdirSync(cache, { recursive: true });
    writeFileSync(file, buf);
    return buf.toString("utf8");
  } finally {
    rmSync(work, { recursive: true, force: true });
  }
}

// ── Pass 1: weight, CLS and axe in Chrome ───────────────────────────────────
// Layout shifts grouped into session windows (gaps under 1s, windows at most
// 5s); CLS is the largest window. Each shift keeps its largest source node.
// hadRecentInput is ignored: the probe never interacts, and under mobile
// emulation Chrome marks every early shift as following input (Lighthouse
// works around the same thing).
const CLS_OBSERVER = `(() => {
  window.__shifts = [];
  const describe = (n) => {
    if (!n || !n.tagName) return "";
    return n.tagName.toLowerCase() + (n.id ? "#" + n.id : "") + (n.classList && n.classList.length ? "." + [...n.classList].slice(0, 2).join(".") : "");
  };
  new PerformanceObserver((l) => {
    for (const e of l.getEntries()) {
      const src = [...(e.sources || [])].sort((a, b) => (b.currentRect.width * b.currentRect.height) - (a.currentRect.width * a.currentRect.height))[0];
      window.__shifts.push({ t: e.startTime, value: e.value, source: describe(src && src.node) });
    }
  }).observe({ type: "layout-shift", buffered: true });
})()`;

function clsOf(shifts) {
  let best = { value: 0, shifts: [] };
  let cur = { value: 0, shifts: [] };
  for (const s of shifts) {
    const first = cur.shifts[0];
    const last = cur.shifts[cur.shifts.length - 1];
    if (first && (s.t - last.t >= 1000 || s.t - first.t >= 5000)) cur = { value: 0, shifts: [] };
    cur.value += s.value;
    cur.shifts.push(s);
    if (cur.value > best.value) best = { value: cur.value, shifts: [...cur.shifts] };
  }
  return best;
}

async function inspect(chrome, axe, page, ff) {
  const tab = await chrome.newPage();
  const sizes = new Map(); // requestId -> {url, bytes}
  const inflight = new Set();
  let lastActivity = Date.now();
  let loaded;
  const loadFired = new Promise((r) => { loaded = r; });
  tab.on((method, p) => {
    if (method === "Network.requestWillBeSent") { inflight.add(p.requestId); sizes.set(p.requestId, { url: p.request.url, bytes: 0 }); lastActivity = Date.now(); }
    if (method === "Network.loadingFinished") {
      inflight.delete(p.requestId);
      lastActivity = Date.now();
      const r = sizes.get(p.requestId);
      if (r) r.bytes = p.encodedDataLength;
    }
    if (method === "Network.loadingFailed") { inflight.delete(p.requestId); lastActivity = Date.now(); }
    if (method === "Page.loadEventFired") loaded();
  });
  await tab.send("Network.enable");
  await tab.send("Network.setCacheDisabled", { cacheDisabled: true });
  await tab.send("Page.enable");
  await tab.send("Runtime.enable");
  await tab.send("Emulation.setDeviceMetricsOverride", { width: ff.width, height: ff.height, deviceScaleFactor: ff.deviceScaleFactor, mobile: ff.mobile });
  await tab.send("Page.addScriptToEvaluateOnNewDocument", { source: CLS_OBSERVER });
  const nav = await tab.send("Page.navigate", { url: page.url });
  await Promise.race([loadFired, new Promise((r) => setTimeout(r, 30000))]);
  // Settle: the network goes quiet (the Try page's wasm, late scripts), and a
  // further second lets late layout shifts land.
  const deadline = Date.now() + 20000;
  while (Date.now() < deadline && (inflight.size > 0 || Date.now() - lastActivity < 1000))
    await new Promise((r) => setTimeout(r, 100));
  await new Promise((r) => setTimeout(r, 1000));
  const evaluate = async (expression) => {
    const res = await tab.send("Runtime.evaluate", { expression, returnByValue: true, awaitPromise: true });
    if (res.exceptionDetails) throw new Error(res.exceptionDetails.exception?.description || res.exceptionDetails.text);
    return res.result.value;
  };
  const cls = clsOf((await evaluate("window.__shifts || []")) || []);
  await evaluate(`${axe}\n;0`);
  const violations = await evaluate(`axe.run(document, { resultTypes: ["violations"] }).then((r) => r.violations.map((v) => ({
    id: v.id, impact: v.impact, help: v.help, nodes: v.nodes.length, targets: v.nodes.slice(0, 3).map((n) => n.target.join(" ")),
  })))`);
  await tab.close();
  const resources = [...sizes.values()].filter((r) => r.bytes > 0);
  return {
    ...(nav.errorText ? { navigation_error: nav.errorText } : {}),
    weight_bytes: resources.reduce((n, r) => n + r.bytes, 0),
    requests: resources.length,
    largest: resources.sort((a, b) => b.bytes - a.bytes).slice(0, 5).map((r) => ({ url: pathOf(r.url), bytes: r.bytes })),
    cls: Number(cls.value.toFixed(4)),
    cls_shifts: cls.shifts.map((s) => ({ t_ms: Math.round(s.t), value: Number(s.value.toFixed(4)), source: s.source })),
    axe: violations,
  };
}

// ── Pass 2: Lighthouse ──────────────────────────────────────────────────────
function lighthouseOnce(url, ff, outFile) {
  const [cmd, ...pre] = LIGHTHOUSE.split(/\s+/);
  const args = [...pre, url, "--output=json", `--output-path=${outFile}`, "--quiet",
    "--chrome-flags=--headless=new --no-sandbox", `--only-categories=${Object.keys(CATEGORIES).join(",")}`, ...ff.lighthouse];
  return new Promise((resolve) => {
    // npm's script runner takes the last variable named PATH in any case; a
    // stray "Path" or "PAth" in a login environment would replace the Node
    // that .nvmrc selected, so only PATH itself is passed on.
    const env = Object.fromEntries(Object.entries(process.env).filter(([k]) => k === "PATH" || k.toLowerCase() !== "path"));
    const child = spawn(cmd, args, { stdio: ["ignore", "ignore", "pipe"], env: { ...env, CHROME_PATH: CHROME } });
    let err = "";
    child.stderr.on("data", (d) => { err += d; });
    const timer = setTimeout(() => child.kill("SIGKILL"), 180000);
    child.on("error", (e) => { clearTimeout(timer); resolve({ error: e.message }); });
    child.on("exit", (code) => {
      clearTimeout(timer);
      if (code !== 0 || !existsSync(outFile)) return resolve({ error: `lighthouse exited ${code}: ${err.trim().split("\n").slice(-4).join(" | ")}` });
      try { resolve({ lhr: JSON.parse(readFileSync(outFile, "utf8")) }); } catch (e) { resolve({ error: `unreadable report: ${e.message}` }); }
    });
  });
}

// Audits that cost a category points: scored below 0.9 with weight in it.
function failingAudits(lhr, category) {
  return (lhr.categories[category]?.auditRefs || [])
    .filter((ref) => ref.weight > 0 && lhr.audits[ref.id] && lhr.audits[ref.id].score !== null && lhr.audits[ref.id].score < 0.9)
    .map((ref) => ({ id: ref.id, score: lhr.audits[ref.id].score, title: lhr.audits[ref.id].title, ...(lhr.audits[ref.id].displayValue ? { value: lhr.audits[ref.id].displayValue } : {}) }));
}

async function lighthouse(page, ff, work) {
  const runs = [];
  for (let i = 0; i < RUNS; i++) {
    const r = await lighthouseOnce(page.url, ff, join(work, `${page.name}-${ff.name}-${i}.json`));
    if (r.error) return { error: r.error };
    if (r.lhr.runtimeError?.code) return { error: `${r.lhr.runtimeError.code}: ${r.lhr.runtimeError.message}` };
    runs.push(r.lhr);
  }
  const scores = {};
  for (const c of Object.keys(CATEGORIES)) {
    const xs = runs.map((l) => l.categories[c]?.score);
    if (xs.some((x) => typeof x !== "number")) return { error: `no ${c} score` };
    scores[c] = Math.round(median(xs) * 100);
  }
  const last = runs[runs.length - 1];
  const metric = (id) => last.audits[id]?.numericValue;
  return {
    version: last.lighthouseVersion,
    benchmark_index: last.environment?.benchmarkIndex,
    scores,
    performance_runs: runs.map((l) => Math.round(l.categories.performance.score * 100)),
    lab: { fcp_ms: metric("first-contentful-paint"), lcp_ms: metric("largest-contentful-paint"), tbt_ms: metric("total-blocking-time"),
      si_ms: metric("speed-index"), cls: metric("cumulative-layout-shift"), total_byte_weight: metric("total-byte-weight") },
    failing: Object.fromEntries(Object.keys(CATEGORIES).map((c) => [c, failingAudits(last, c)]).filter(([, a]) => a.length)),
  };
}

// ── Run ─────────────────────────────────────────────────────────────────────
const axe = axeSource();
const metrics = {};
const details = [];
const skipped = {};
const chrome = await launchChrome(CHROME, "site-quality-");
const results = [];
for (const page of PAGES) for (const ff of FORM_FACTORS) {
  const subject = `${TARGET}.${page.name}.${ff.name}`;
  log(`inspect ${page.name} (${ff.name})`);
  const r = await inspect(chrome, axe, page, ff);
  results.push({ page, ff, subject, r });
  metrics[`weight_bytes.${subject}`] = r.weight_bytes;
  metrics[`cls.${subject}`] = r.cls;
  for (const impact of IMPACTS)
    metrics[`axe_${impact}.${subject}`] = r.axe.filter((v) => v.impact === impact).length;
}
await chrome.close();

const work = mkdtempSync(join(tmpdir(), "site-quality-lh-"));
for (const res of results) {
  if (LIGHTHOUSE === "off") { res.lh = { error: "LIGHTHOUSE=off" }; continue; }
  log(`lighthouse ${res.page.name} (${res.ff.name}) x${RUNS}`);
  res.lh = await lighthouse(res.page, res.ff, work);
  if (res.lh.error) { skipped[`lighthouse.${res.subject}`] = res.lh.error; continue; }
  for (const [c, name] of Object.entries(CATEGORIES)) metrics[`${name}.${res.subject}`] = res.lh.scores[c];
}
rmSync(work, { recursive: true, force: true });

for (const { page, ff, r, lh } of results)
  details.push({ page: page.name, path: pathOf(page.url), form_factor: ff.name, ...r, lighthouse: lh });
details.unshift({ target: TARGET, base: BASE, lighthouse: LIGHTHOUSE === "off" ? "off" : LIGHTHOUSE_VERSION, axe_core: AXE_VERSION, lighthouse_runs: RUNS });
console.log(JSON.stringify({ probe: "site-quality", metrics, details, ...(Object.keys(skipped).length ? { skipped } : {}) }, null, 1));
process.exit(0);
