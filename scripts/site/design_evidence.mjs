// Design evidence: screenshots of a site's pages at desktop and mobile, in light
// and dark, for the treemand-site-product-design skill to read and judge.
//
//   OUT=<dir> CHROME=<chrome> [BASE=<url>] [PEERS=<tsv>] node design_evidence.mjs
//
// BASE: the treemand site to capture; every sitemap.xml path (plus /) is one
// page. PEERS: a TSV of `name<TAB>page<TAB>url` rows (# comments allowed), each
// URL captured once for comparison. Each page is fetched with ONE navigation;
// the four views are re-renders of that load (viewport and colour scheme
// emulation), so a peer site sees one page load per URL.
//
// Each view is captured as viewport-sized tiles from the top: tile 1 is the
// first screen. Files: <OUT>/<site>/<page>--<desktop|mobile>-<light|dark>-<n>.png,
// plus <OUT>/manifest.json (status, load time, bytes, tiles per page) and a
// human-readable <OUT>/index.md.
import { mkdirSync, readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { launchChrome } from "./cdp.mjs";

const OUT = process.env.OUT;
const CHROME = process.env.CHROME;
const BASE = process.env.BASE || "";
const PEERS = process.env.PEERS || "";
if (!OUT || !CHROME || (!BASE && !PEERS)) {
  console.error("design_evidence: set OUT, CHROME and BASE and/or PEERS");
  process.exit(2);
}
const TILES = Number(process.env.DESIGN_TILES || 4);
const PEER_TILES = Number(process.env.DESIGN_PEER_TILES || 3);
// A pause between peer pages, so a run never bursts at one host.
const PEER_GAP_MS = Number(process.env.DESIGN_PEER_GAP_MS || 2000);
const UA_NOTE = "treemand-design-evidence/1 (+https://treemand.dev/)";
const VIEWS = [
  { name: "desktop-light", width: 1280, height: 800, mobile: false, scheme: "light" },
  { name: "desktop-dark", width: 1280, height: 800, mobile: false, scheme: "dark" },
  { name: "mobile-light", width: 390, height: 844, mobile: true, scheme: "light" },
  { name: "mobile-dark", width: 390, height: 844, mobile: true, scheme: "dark" },
];

const log = (...a) => console.error("design_evidence:", ...a);
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));
// "/" -> "home", "/docs/getting-started/" -> "docs_getting-started".
const slug = (path) => path.replace(/^\/+|\/+$/g, "").replace(/[^A-Za-z0-9._-]+/g, "_") || "home";

async function sitemapPages(base) {
  const origin = new URL(base).origin;
  const paths = new Set(["/"]);
  try {
    const res = await fetch(new URL("sitemap.xml", base), { headers: { "user-agent": UA_NOTE } });
    if (res.ok) {
      for (const m of (await res.text()).matchAll(/<loc>\s*([^<\s]+)\s*<\/loc>/g)) {
        // The sitemap names production URLs; capture the same paths on BASE.
        paths.add(new URL(m[1]).pathname);
      }
    } else log(`sitemap.xml returned ${res.status}`);
  } catch (e) { log(`sitemap.xml: ${e.message}`); }
  return [...paths].sort().map((p) => ({ page: slug(p), url: new URL(p, origin).href }));
}

function readPeers(file) {
  return readFileSync(file, "utf8").split("\n")
    .map((l) => l.replace(/#.*/, "").trim()).filter(Boolean)
    .map((l) => { const [site, page, url] = l.split(/\t+/); return { site, page, url }; })
    .filter((p) => p.site && p.page && /^https?:/.test(p.url || ""));
}

const chrome = await launchChrome(CHROME, "design-evidence-");

async function capture(site, page, url, maxTiles) {
  const dir = join(OUT, site);
  mkdirSync(dir, { recursive: true });
  const rec = { site, page, url, status: 0, views: {} };
  const tab = await chrome.newPage();
  let loaded;
  const loadFired = new Promise((r) => { loaded = r; });
  const inflight = new Set();
  let lastActivity = Date.now();
  tab.on((method, p) => {
    if (method === "Network.requestWillBeSent") { inflight.add(p.requestId); lastActivity = Date.now(); }
    if (method === "Network.loadingFinished" || method === "Network.loadingFailed") { inflight.delete(p.requestId); lastActivity = Date.now(); }
    if (method === "Network.responseReceived" && p.type === "Document" && p.frameId === tab.id) rec.status = p.response.status;
    if (method === "Page.loadEventFired") loaded();
  });
  await tab.send("Network.enable");
  await tab.send("Page.enable");
  const first = VIEWS[0];
  await tab.send("Emulation.setDeviceMetricsOverride", { width: first.width, height: first.height, deviceScaleFactor: 1, mobile: first.mobile });
  await tab.send("Emulation.setEmulatedMedia", { features: [{ name: "prefers-color-scheme", value: first.scheme }] });
  const nav = await tab.send("Page.navigate", { url });
  if (nav.errorText) rec.error = nav.errorText;
  await Promise.race([loadFired, sleep(20000)]);
  const settle = async (ms) => {
    const deadline = Date.now() + ms;
    while (Date.now() < deadline && (inflight.size > 0 || Date.now() - lastActivity < 500)) await sleep(100);
  };
  await settle(8000);
  const ev = async (expr) => (await tab.send("Runtime.evaluate", { expression: expr, returnByValue: true })).result.value;
  // Speed evidence from the one real load (before any re-render).
  Object.assign(rec, await ev(`(() => {
    const n = performance.getEntriesByType("navigation")[0] || {};
    const res = performance.getEntriesByType("resource");
    return {
      title: document.title,
      final_url: location.href,
      load_ms: Math.round(n.loadEventEnd || 0),
      dom_content_loaded_ms: Math.round(n.domContentLoadedEventEnd || 0),
      transfer_kb: Math.round(((n.transferSize || 0) + res.reduce((s, r) => s + (r.transferSize || 0), 0)) / 1024),
      requests: res.length + 1,
    };
  })()`) || {});
  for (const v of VIEWS) {
    await tab.send("Emulation.setDeviceMetricsOverride", { width: v.width, height: v.height, deviceScaleFactor: 1, mobile: v.mobile });
    await tab.send("Emulation.setEmulatedMedia", { features: [{ name: "prefers-color-scheme", value: v.scheme }] });
    // A phone has no hover: touch emulation makes (hover: none) and (pointer:
    // coarse) match, so touch-only styles (always-visible buttons) render.
    await tab.send("Emulation.setTouchEmulationEnabled", v.mobile ? { enabled: true, maxTouchPoints: 5 } : { enabled: false });
    await ev("window.scrollTo(0, 0)");
    await sleep(400);
    await settle(3000);
    const height = (await ev("Math.max(document.documentElement.scrollHeight, document.body ? document.body.scrollHeight : 0)")) || v.height;
    const tiles = Math.max(1, Math.min(maxTiles, Math.ceil(height / v.height)));
    const files = [];
    for (let i = 0; i < tiles; i++) {
      // Scrolling (rather than one tall capture) loads lazy images and shows
      // sticky headers the way a reader sees them.
      await ev(`window.scrollTo(0, ${i * v.height})`);
      await sleep(i === 0 ? 100 : 350);
      const shot = await tab.send("Page.captureScreenshot", { format: "png" });
      const file = join(site, `${page}--${v.name}-${i + 1}.png`);
      writeFileSync(join(OUT, file), Buffer.from(shot.data, "base64"));
      files.push(file);
    }
    rec.views[v.name] = { page_height: height, hover_none: await ev("matchMedia('(hover: none)').matches"), tiles: files };
  }
  await tab.close();
  return rec;
}

const records = [];
if (BASE) {
  const pages = await sitemapPages(BASE);
  log(`${pages.length} pages on ${BASE}`);
  for (const p of pages) {
    log(`treemand ${p.page}`);
    records.push(await capture("treemand", p.page, p.url, TILES));
  }
}
if (PEERS) {
  const peers = readPeers(PEERS);
  for (const [i, p] of peers.entries()) {
    if (i > 0) await sleep(PEER_GAP_MS);
    log(`peer ${p.site} ${p.page}`);
    try { records.push(await capture(p.site, p.page, p.url, PEER_TILES)); }
    catch (e) { log(`${p.url}: ${e.message}`); records.push({ site: p.site, page: p.page, url: p.url, error: e.message, views: {} }); }
  }
}

writeFileSync(join(OUT, "manifest.json"), JSON.stringify({ base: BASE || null, views: VIEWS, pages: records }, null, 1) + "\n");
const md = ["# Design evidence", "", `Target: ${BASE || "(peers only)"}`, "",
  "| site | page | status | load ms | KB | first screen (desktop light) |", "| --- | --- | --- | --- | --- | --- |",
  ...records.map((r) => `| ${r.site} | ${r.page} | ${r.status || r.error || ""} | ${r.load_ms ?? ""} | ${r.transfer_kb ?? ""} | ${r.views["desktop-light"]?.tiles[0] ?? ""} |`), ""];
writeFileSync(join(OUT, "index.md"), md.join("\n"));
await chrome.close();
const failed = records.filter((r) => r.site === "treemand" && (r.error || r.status >= 400 || !r.status));
if (failed.length) { log(`${failed.length} treemand page(s) failed to load`); process.exit(1); }
process.exit(0);
