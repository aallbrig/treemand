// Site audit: drives headless Chrome over the DevTools protocol (no npm
// dependencies) against the built site and fails on the problems Lighthouse's
// SEO, Accessibility and Best Practices categories score, plus layout shift.
//
//   BASE=http://127.0.0.1:PORT/ CHROME=/path/to/chrome node site_audit.mjs
//
// Run it through `task site:audit`, which builds and serves the site. This is
// not Lighthouse and produces no Lighthouse score: it checks the same families
// of rules deterministically, in this repository, with no downloads.
import { spawn } from "node:child_process";
import { mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

const BASE = process.env.BASE;
const CHROME = process.env.CHROME;
if (!BASE || !CHROME) {
  console.error("site_audit: set BASE and CHROME");
  process.exit(2);
}
const origin = new URL(BASE).origin;
// Every page is checked in both colour schemes: the site follows the system
// setting, so a contrast bug in either theme is a real reader's bug.
const VIEWPORTS = [
  { name: "desktop", width: 1280, height: 800, mobile: false, scheme: "light" },
  { name: "mobile", width: 390, height: 844, mobile: true, scheme: "light" },
  { name: "desktop-dark", width: 1280, height: 800, mobile: false, scheme: "dark" },
  { name: "mobile-dark", width: 390, height: 844, mobile: true, scheme: "dark" },
];

// ── Chrome and the DevTools protocol ────────────────────────────────────────
const profile = mkdtempSync(join(tmpdir(), "site-audit-"));
const chrome = spawn(CHROME, [
  "--headless=new", "--no-sandbox", "--disable-gpu", "--remote-debugging-port=0",
  `--user-data-dir=${profile}`, "about:blank",
], { stdio: ["ignore", "ignore", "pipe"] });
// Chrome keeps writing into its profile until it has exited, so deleting the
// directory right after kill() races with it (ENOTEMPTY). Wait for the exit,
// retry the delete, and never let cleanup change the audit's verdict.
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
    send: (method, params = {}) => new Promise((resolve, reject) => {
      const n = ++id;
      pending.set(n, { resolve, reject });
      ws.send(JSON.stringify({ id: n, method, params }));
    }),
    on: (fn) => listeners.push(fn),
    close: () => ws.close(),
  };
}

// ── In-page checks (run inside the document) ───────────────────────────────
const pageChecks = () => {
  const out = [];
  const bad = (rule, detail) => out.push({ rule, detail });
  const vis = (el) => {
    const s = getComputedStyle(el);
    if (s.display === "none" || s.visibility === "hidden" || Number(s.opacity) === 0) return false;
    const r = el.getBoundingClientRect();
    return r.width > 0 && r.height > 0;
  };
  const parse = (c) => {
    const m = c.match(/rgba?\(([^)]+)\)/);
    if (!m) return null;
    const [r, g, b, a = 1] = m[1].split(",").map(parseFloat);
    return { r, g, b, a };
  };
  const over = (fg, bg) => ({
    r: fg.r * fg.a + bg.r * (1 - fg.a), g: fg.g * fg.a + bg.g * (1 - fg.a),
    b: fg.b * fg.a + bg.b * (1 - fg.a), a: 1,
  });
  const lum = ({ r, g, b }) => {
    const f = (v) => { v /= 255; return v <= 0.03928 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4; };
    return 0.2126 * f(r) + 0.7152 * f(g) + 0.0722 * f(b);
  };
  const contrast = (a, b) => {
    const [hi, lo] = [lum(a), lum(b)].sort((x, y) => y - x);
    return (hi + 0.05) / (lo + 0.05);
  };
  const backdrop = (el) => {
    for (let n = el; n; n = n.parentElement) {
      const c = parse(getComputedStyle(n).backgroundColor);
      if (c && c.a > 0.95) return c;
    }
    return { r: 255, g: 255, b: 255, a: 1 };
  };
  const meta = (sel) => document.querySelector(sel)?.getAttribute("content") || "";

  // SEO
  const title = document.title.trim();
  if (!title) bad("document-title", "missing <title>");
  const desc = meta('meta[name="description"]');
  if (desc.length < 50 || desc.length > 170) bad("meta-description", `length ${desc.length}, want 50 to 170`);
  const canon = document.querySelector('link[rel="canonical"]')?.href || "";
  if (!/^https:\/\//.test(canon)) bad("canonical", `not an absolute https URL: ${canon || "missing"}`);
  else if (new URL(canon).pathname !== location.pathname) bad("canonical", `path ${new URL(canon).pathname} != ${location.pathname}`);
  for (const p of ["og:title", "og:description", "og:image", "og:url", "og:type"])
    if (!meta(`meta[property="${p}"]`)) bad("open-graph", `missing ${p}`);
  for (const n of ["twitter:card", "twitter:image", "twitter:title"])
    if (!meta(`meta[name="${n}"]`)) bad("twitter-card", `missing ${n}`);
  if (!document.querySelector('link[rel~="icon"]')) bad("favicon", "no <link rel=icon>");
  if (!document.querySelector('link[rel="apple-touch-icon"]')) bad("favicon", "no apple-touch-icon");
  if (!document.documentElement.lang) bad("html-has-lang", "missing lang");
  const vp = meta('meta[name="viewport"]');
  if (!vp) bad("viewport", "missing viewport meta");
  if (/user-scalable\s*=\s*(no|0)|maximum-scale\s*=\s*[01](\.\d+)?\b/.test(vp)) bad("meta-viewport", "zoom is disabled");

  // Accessibility
  const h = [...document.querySelectorAll("h1,h2,h3,h4,h5,h6")].filter(vis);
  if (h.filter((x) => x.tagName === "H1").length !== 1) bad("one-h1", `${h.filter((x) => x.tagName === "H1").length} h1 elements`);
  let prev = 0;
  for (const el of h) {
    const lvl = Number(el.tagName[1]);
    if (prev && lvl > prev + 1) bad("heading-order", `h${prev} jumps to h${lvl}: "${el.textContent.trim().slice(0, 40)}"`);
    prev = lvl;
  }
  for (const img of document.querySelectorAll("img")) {
    if (!img.hasAttribute("alt")) bad("image-alt", img.currentSrc || img.src);
    if (!img.getAttribute("width") || !img.getAttribute("height")) bad("unsized-image", img.src);
  }
  for (const a of document.querySelectorAll("a[href]")) {
    if (vis(a) && !(a.textContent.trim() || a.getAttribute("aria-label") || a.querySelector("img[alt]")))
      bad("link-name", a.getAttribute("href"));
  }
  for (const b of document.querySelectorAll("button")) {
    if (vis(b) && !(b.textContent.trim() || b.getAttribute("aria-label") || b.getAttribute("title")))
      bad("button-name", b.outerHTML.slice(0, 80));
  }
  for (const el of document.querySelectorAll("input,select,textarea")) {
    if (vis(el) && el.type !== "hidden" && !(el.labels?.length || el.getAttribute("aria-label") || el.getAttribute("aria-labelledby")))
      bad("label", el.outerHTML.slice(0, 80));
  }
  const ids = new Map();
  for (const el of document.querySelectorAll("[id]")) ids.set(el.id, (ids.get(el.id) || 0) + 1);
  for (const [id, n] of ids) if (n > 1) bad("duplicate-id", id);

  // Contrast and legible text
  const seen = new Set();
  const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
  for (let n = walker.nextNode(); n; n = walker.nextNode()) {
    const el = n.parentElement;
    if (!n.textContent.trim() || seen.has(el) || !vis(el) || el.closest("script,style,.xterm")) continue;
    seen.add(el);
    const s = getComputedStyle(el);
    const size = parseFloat(s.fontSize);
    if (size < 12) bad("font-size", `${size}px: "${n.textContent.trim().slice(0, 30)}"`);
    if (s.webkitTextFillColor === "rgba(0, 0, 0, 0)" || s.backgroundClip === "text" || s.webkitBackgroundClip === "text") continue; // gradient text
    const bg = backdrop(el);
    let fg = parse(s.color);
    if (!fg) continue;
    fg = over(fg, bg);
    const large = size >= 24 || (size >= 18.66 && Number(s.fontWeight) >= 700);
    const ratio = contrast(fg, bg);
    if (ratio < (large ? 3 : 4.5)) bad("color-contrast", `${ratio.toFixed(2)}:1 for "${n.textContent.trim().slice(0, 30)}" (${s.color} on rgb(${bg.r | 0},${bg.g | 0},${bg.b | 0}))`);
  }

  // Layout and best practices
  if (document.documentElement.scrollWidth > window.innerWidth + 1)
    bad("horizontal-overflow", `scrollWidth ${document.documentElement.scrollWidth} > ${window.innerWidth}`);
  for (const s of document.querySelectorAll("head script[src]:not([defer]):not([async]):not([type=module])"))
    bad("render-blocking-script", s.src);
  for (const el of document.querySelectorAll("button, nav a")) {
    if (!vis(el)) continue;
    const r = el.getBoundingClientRect();
    if (Math.min(r.width, r.height) < 24) bad("target-size", `${r.width.toFixed(0)}x${r.height.toFixed(0)}: "${(el.textContent || "").trim().slice(0, 20)}"`);
  }
  return out;
};

// ── Run ─────────────────────────────────────────────────────────────────────
const sitemap = await (await fetch(new URL("sitemap.xml", BASE))).text();
const pages = [...sitemap.matchAll(/<loc>([^<]+)<\/loc>/g)]
  .map((m) => new URL(m[1], origin).pathname)
  .map((p) => new URL(p.replace(/^\/treemand\//, ""), BASE).href);
if (pages.length === 0) { console.error("site_audit: sitemap lists no pages"); process.exit(2); }

const findings = [];
const warnings = new Set();
const add = (page, vp, rule, detail) => findings.push({ page: new URL(page).pathname, vp, rule, detail });

// Social image and icons must exist and be real images.
async function checkAsset(page, path, kind, minWidth) {
  const url = new URL(path, BASE);
  const res = await fetch(url);
  if (!res.ok) return add(page, "-", kind, `${path} returned ${res.status}`);
  if (path.endsWith(".png")) {
    const b = Buffer.from(await res.arrayBuffer());
    const w = b.readUInt32BE(16);
    if (b.toString("latin1", 1, 4) !== "PNG" || w < minWidth) add(page, "-", kind, `${path} is not a PNG at least ${minWidth}px wide`);
  }
}
{
  await checkAsset(BASE, "social-card.png", "social-card", 1200);
  await checkAsset(BASE, "favicon-32.png", "favicon", 32);
  await checkAsset(BASE, "apple-touch-icon.png", "favicon", 180);
  await checkAsset(BASE, "icon.svg", "favicon", 0);
}

// ── Installation page behaviour: OS detection, override, copy buttons ──────
async function installPageScenarios() {
  const url = new URL("installation/", BASE).href;
  const UA = {
    macos: { ua: "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36", platform: "MacIntel", md: "macOS" },
    windows: { ua: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36", platform: "Win32", md: "Windows" },
    linux: { ua: "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0 Safari/537.36", platform: "Linux x86_64", md: "Linux" },
    other: { ua: "Mozilla/5.0 (Unknown) Gecko/1.0", platform: "", md: "" },
  };
  const state = (p) => p.send("Runtime.evaluate", { returnByValue: true, expression: `({
    visible: [...document.querySelectorAll(".os-panel")].filter((x) => !x.hidden).map((x) => x.dataset.os),
    pressed: [...document.querySelectorAll(".os-btn")].filter((b) => b.getAttribute("aria-pressed") === "true").map((b) => b.dataset.os),
    switchShown: !document.getElementById("os-switch").hidden,
    note: document.getElementById("os-detected").hidden ? "" : document.getElementById("os-detected").textContent,
  })` }).then((r) => r.result.value);
  async function open(ua, { hash = "", jsOff = false, keepStorage = false } = {}) {
    const p = await newPage();
    await p.send("Page.enable");
    await p.send("Emulation.setDeviceMetricsOverride", { width: 1280, height: 800, deviceScaleFactor: 1, mobile: false });
    if (!keepStorage) await p.send("Storage.clearDataForOrigin", { origin, storageTypes: "local_storage" });
    await p.send("Emulation.setScriptExecutionDisabled", { value: jsOff });
    const u = UA[ua];
    await p.send("Network.setUserAgentOverride", {
      userAgent: u.ua, platform: u.platform,
      userAgentMetadata: { platform: u.md, platformVersion: "1.0", architecture: "x86", model: "", mobile: false },
    });
    await p.send("Page.navigate", { url: url + hash });
    await new Promise((r) => setTimeout(r, 700));
    return p;
  }
  const expect = (name, got, want) => {
    if (JSON.stringify(got) !== JSON.stringify(want)) add(url, "-", "install-page", `${name}: got ${JSON.stringify(got)}, want ${JSON.stringify(want)}`);
  };

  for (const [os, name] of [["macos", "macOS"], ["windows", "Windows"], ["linux", "Linux"]]) {
    const p = await open(os);
    const st = await state(p);
    expect(`${name} browser shows its own panel`, st.visible, [os]);
    expect(`${name} browser marks its button pressed`, st.pressed, [os]);
    if (!st.switchShown) add(url, "-", "install-page", "the OS switch is hidden although JavaScript ran");
    if (!st.note.includes(name)) add(url, "-", "install-page", `no 'we think you are on ${name}' note: "${st.note}"`);
    p.close();
  }
  let p = await open("other");
  expect("an unknown system falls back to Linux", (await state(p)).visible, ["linux"]);
  p.close();

  // Manual override wins, and is remembered across a reload.
  p = await open("macos");
  await p.send("Runtime.evaluate", { expression: 'document.querySelector(\'.os-btn[data-os="windows"]\').click()' });
  let st = await state(p);
  expect("clicking Windows shows only Windows", st.visible, ["windows"]);
  expect("clicking Windows presses only its button", st.pressed, ["windows"]);
  if (st.note) add(url, "-", "install-page", "the detection note stays after a manual choice");
  p.close();
  p = await open("macos", { keepStorage: true });
  expect("a manual choice is remembered on the next visit", (await state(p)).visible, ["windows"]);
  p.close();

  // The URL hash is shareable and beats detection.
  p = await open("linux", { hash: "#macos" });
  expect("#macos selects macOS on a Linux browser", (await state(p)).visible, ["macos"]);
  p.close();

  // Without JavaScript every panel is visible and the switch stays hidden.
  p = await open("macos", { jsOff: true });
  st = await state(p);
  expect("with JavaScript off all three panels show", st.visible.slice().sort(), ["linux", "macos", "windows"]);
  if (st.switchShown) add(url, "-", "install-page", "the OS switch is shown although JavaScript is off");
  p.close();

  // Copy buttons: named, copy the exact commands, and announce the result.
  p = await open("linux");
  await p.send("Runtime.evaluate", { expression: `
    window.__copied = null;
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText: (t) => { window.__copied = t; return Promise.resolve(); } } });` });
  const info = (await p.send("Runtime.evaluate", { returnByValue: true, expression: `(() => {
    const panel = document.querySelector('.os-panel[data-os="linux"]');
    const boxes = [...panel.querySelectorAll(".cmd")];
    const btn = boxes[0].querySelector(".copy-btn");
    const before = { count: boxes.length, buttons: boxes.filter((b) => b.querySelector(".copy-btn")).length, name: btn.getAttribute("aria-label"), text: btn.textContent };
    btn.click();
    return { before, code: boxes[0].querySelector("code").textContent };
  })()` })).result.value;
  await new Promise((r) => setTimeout(r, 200));
  const after = (await p.send("Runtime.evaluate", { returnByValue: true, expression: `({
    copied: window.__copied, label: document.querySelector('.os-panel[data-os="linux"] .copy-btn').textContent,
    status: document.querySelector('#install-picker [role="status"].sr-only').textContent })` })).result.value;
  if (info.before.count < 3 || info.before.buttons !== info.before.count) add(url, "-", "install-page", `${info.before.buttons} copy buttons for ${info.before.count} command blocks`);
  if (!/^Copy .*commands$/.test(info.before.name || "")) add(url, "-", "install-page", `copy button accessible name: "${info.before.name}"`);
  expect("the copy button copies exactly the commands shown", after.copied, info.code);
  expect("the copy button confirms", after.label, "Copied");
  if (!/Copied/.test(after.status)) add(url, "-", "install-page", `no screen-reader confirmation: "${after.status}"`);
  if (!/curl -fsSL https:\/\/treemand\.dev\/install\.sh \| sh/.test(info.code)) add(url, "-", "install-page", "the first Linux command is not the install script one-liner");
  p.close();
}
await installPageScenarios();

// ── Theme toggle and code-block copy buttons ────────────────────────────────
async function siteScenarios() {
  const url = new URL("getting-started/", BASE).href;
  const fail = (detail) => add(url, "-", "site-js", detail);
  const evalv = (p, expression) => p.send("Runtime.evaluate", { expression, returnByValue: true, awaitPromise: true }).then((r) => r.result.value);
  async function open(scheme, keepStorage = false) {
    const p = await newPage();
    await p.send("Page.enable");
    await p.send("Emulation.setDeviceMetricsOverride", { width: 1280, height: 800, deviceScaleFactor: 1, mobile: false });
    await p.send("Emulation.setEmulatedMedia", { features: [{ name: "prefers-color-scheme", value: scheme }] });
    if (!keepStorage) await p.send("Storage.clearDataForOrigin", { origin, storageTypes: "local_storage" });
    await p.send("Page.navigate", { url });
    await new Promise((r) => setTimeout(r, 700));
    return p;
  }
  const bg = 'getComputedStyle(document.body).backgroundColor';
  const pressed = 'document.getElementById("theme-toggle").getAttribute("aria-pressed")';

  let p = await open("light");
  if (await evalv(p, bg) !== "rgb(255, 255, 255)") fail(`light system: body is ${await evalv(p, bg)}, want white`);
  if (await evalv(p, pressed) !== "false") fail("light system: the toggle reads as pressed");
  await evalv(p, 'document.getElementById("theme-toggle").click()');
  if (await evalv(p, bg) !== "rgb(13, 17, 23)") fail("clicking the toggle does not switch to the dark theme");
  if (await evalv(p, pressed) !== "true") fail("after switching to dark the toggle is not pressed");
  p.close();
  p = await open("light", true);
  if (await evalv(p, bg) !== "rgb(13, 17, 23)") fail("the chosen dark theme is not remembered on the next visit");
  p.close();
  p = await open("dark");
  if (await evalv(p, bg) !== "rgb(13, 17, 23)") fail("dark system: the page is not dark");
  p.close();

  // Every code block outside the Installation picker gets a working copy button.
  p = await open("light");
  const r = await evalv(p, `(async () => {
    window.__copied = null;
    Object.defineProperty(navigator, "clipboard", { configurable: true, value: { writeText: (t) => { window.__copied = t; return Promise.resolve(); } } });
    const pres = [...document.querySelectorAll("main pre")];
    const btn = document.querySelector("main .copy-btn");
    if (!btn) return { pres: pres.length, buttons: 0 };
    btn.click();
    await new Promise((res) => setTimeout(res, 100));
    const code = btn.parentElement.querySelector("code").textContent.replace(/\\n$/, "");
    return { pres: pres.length, buttons: document.querySelectorAll("main .copy-btn").length, copied: window.__copied, code, label: btn.textContent };
  })()`);
  if (r.pres === 0) fail("no code blocks on the page used for the copy check");
  else if (r.buttons !== r.pres) fail(`${r.buttons} copy buttons for ${r.pres} code blocks`);
  else {
    if (r.copied !== r.code) fail("the copy button does not copy the block's text exactly");
    if (r.label !== "Copied") fail(`the copy button says "${r.label}" after a click`);
  }
  p.close();
}
await siteScenarios();

// ── Home page: the proof count is the corpus's, the tree expands, agent files exist
async function homeScenarios() {
  const url = BASE;
  const fail = (detail) => add(url, "-", "home", detail);
  const corpus = JSON.parse(readFileSync(new URL("../cli/treemand/discovery/testdata/corpus/expect.json", import.meta.url), "utf8"));
  const want = Object.keys(corpus).filter((k) => !k.startsWith("_")).length;
  const p = await newPage();
  await p.send("Page.enable");
  await p.send("Emulation.setDeviceMetricsOverride", { width: 1280, height: 800, deviceScaleFactor: 1, mobile: false });
  await p.send("Page.navigate", { url });
  await new Promise((r) => setTimeout(r, 700));
  const r = (await p.send("Runtime.evaluate", { returnByValue: true, expression: `(() => {
    const proof = document.querySelector(".proof strong")?.textContent || "";
    const s = [...document.querySelectorAll(".hero2-demo details")].find((d) => !d.open && d.querySelector("ul li"));
    const before = s ? s.getBoundingClientRect().height : 0;
    if (s) s.querySelector("summary").click();
    return { proof, expandable: !!s, grew: s ? s.getBoundingClientRect().height > before : false };
  })()` })).result.value;
  p.close();
  if (!r.proof.includes(`Tested against ${want} real CLIs`)) fail(`proof says "${r.proof}", want "Tested against ${want} real CLIs"`);
  if (!r.expandable) fail("the hero tree has no collapsed command to expand");
  else if (!r.grew) fail("clicking a command in the hero tree does not expand it");
  for (const f of ["llms.txt", "llms-full.txt"]) {
    const res = await fetch(new URL(f, BASE));
    const text = res.ok ? await res.text() : "";
    if (!res.ok || !text.startsWith("# treemand")) fail(`/${f} is missing or does not start with "# treemand"`);
  }
}
await homeScenarios();

// ── Docs layout: sidebar marks the page, prev/next follow the sidebar, anchors resolve
async function docsScenarios() {
  const url = new URL("commands/cache/", BASE).href;
  const fail = (detail) => add(url, "-", "docs-layout", detail);
  const p = await newPage();
  await p.send("Page.enable");
  await p.send("Emulation.setDeviceMetricsOverride", { width: 1280, height: 800, deviceScaleFactor: 1, mobile: false });
  await p.send("Page.navigate", { url });
  await new Promise((r) => setTimeout(r, 700));
  const r = (await p.send("Runtime.evaluate", { returnByValue: true, expression: `(() => {
    const wide = document.querySelector(".docs-nav-wide");
    const links = wide ? [...wide.querySelectorAll("ul a")].map((a) => a.getAttribute("href")) : [];
    const here = wide?.querySelector('a[aria-current="page"]')?.getAttribute("href") || "";
    const i = links.indexOf(here);
    const anchors = [...document.querySelectorAll("a.anchor")];
    return {
      here, path: location.pathname, prevWant: links[i - 1] || null, nextWant: links[i + 1] || null,
      prev: document.querySelector(".pager-prev")?.getAttribute("href") || null,
      next: document.querySelector(".pager-next")?.getAttribute("href") || null,
      toc: document.querySelectorAll(".docs-toc a").length,
      anchors: anchors.length,
      broken: anchors.filter((a) => !document.getElementById(decodeURIComponent(a.hash.slice(1)))).map((a) => a.hash),
      edit: document.querySelector(".edit-link a")?.href || "",
    };
  })()` })).result.value;
  p.close();
  if (r.here !== r.path) fail(`sidebar marks ${r.here || "nothing"} as current, want ${r.path}`);
  if (r.prev !== r.prevWant || r.next !== r.nextWant) fail(`pager ${r.prev} / ${r.next}, want ${r.prevWant} / ${r.nextWant} (sidebar order)`);
  if (r.toc === 0) fail("no 'On this page' outline");
  if (r.anchors === 0) fail("no heading anchors");
  if (r.broken.length) fail(`heading anchors with no target: ${r.broken.join(" ")}`);
  if (!r.edit.startsWith("https://github.com/aallbrig/treemand/edit/main/www/treemand/content/commands/cache.md")) fail(`edit link: ${r.edit}`);
}
await docsScenarios();

// ── Search: Ctrl+K opens the dialog, and a query finds the page about it in the top 3
async function searchScenarios() {
  const url = new URL("getting-started/", BASE).href;
  const fail = (detail) => add(url, "-", "search", detail);
  const p = await newPage();
  await p.send("Page.enable");
  await p.send("Emulation.setDeviceMetricsOverride", { width: 1280, height: 800, deviceScaleFactor: 1, mobile: false });
  await p.send("Page.navigate", { url });
  await new Promise((r) => setTimeout(r, 700));
  const ev = (expression) => p.send("Runtime.evaluate", { expression, returnByValue: true, awaitPromise: true }).then((r) => r.result.value);
  await p.send("Input.dispatchKeyEvent", { type: "keyDown", key: "k", code: "KeyK", modifiers: 2, windowsVirtualKeyCode: 75 });
  await p.send("Input.dispatchKeyEvent", { type: "keyUp", key: "k", code: "KeyK", modifiers: 2, windowsVirtualKeyCode: 75 });
  await new Promise((r) => setTimeout(r, 800));
  if (!(await ev('document.getElementById("search-dialog").open'))) { fail("Ctrl+K does not open the search dialog"); p.close(); return; }
  // The UI renders results for a query typed into it.
  const shown = await ev(`(async () => {
    const input = document.querySelector("#search input");
    if (!input) return 0;
    input.value = "cache";
    input.dispatchEvent(new Event("input", { bubbles: true }));
    for (let i = 0; i < 40; i++) {
      await new Promise((r) => setTimeout(r, 100));
      const n = document.querySelectorAll("#search .pagefind-ui__result-link").length;
      if (n) return n;
    }
    return 0;
  })()`);
  if (!shown) fail("typing in the search box shows no results");
  // Ranking, through Pagefind's own API (what the UI calls): the page about a
  // topic must be among the top 3 pages for it.
  for (const [q, want] of [["cache clear", "/commands/cache/"], ["completion zsh", "/commands/completion/"], ["json yaml", "/commands/output/"]]) {
    const top = await ev(`(async () => {
      const pf = await import(document.getElementById("search").dataset.base + "pagefind.js");
      const s = await pf.search(${JSON.stringify(q)});
      return Promise.all(s.results.slice(0, 3).map(async (x) => new URL((await x.data()).url, location.href).pathname));
    })()`);
    if (!Array.isArray(top) || !top.includes(want)) fail(`"${q}": top results ${top}, want ${want} among them`);
  }
  await p.send("Input.dispatchKeyEvent", { type: "keyDown", key: "Escape", code: "Escape", windowsVirtualKeyCode: 27 });
  await new Promise((r) => setTimeout(r, 200));
  if (await ev('document.getElementById("search-dialog").open')) fail("Esc does not close the search dialog");
  p.close();
}
await searchScenarios();

const titles = new Map();
let audited = 0;
for (const vp of VIEWPORTS) {
  for (const url of pages) {
    const page = await newPage();
    const problems = [];
    const urls = new Map();
    // A failure fetching someone else's resource (a badge service, a CDN) is
    // reported as a warning: it must not fail the build, but it is worth seeing.
    const thirdParty = (u) => { try { return new URL(u).origin !== origin; } catch { return false; } };
    const report = (rule, detail, url) => {
      if (url && thirdParty(url)) warnings.add(`${new URL(page_url).pathname} [${vp.name}] ${rule}: ${detail}`);
      else problems.push([rule, detail]);
    };
    const page_url = url;
    page.on((method, p) => {
      if (method === "Network.requestWillBeSent") urls.set(p.requestId, p.request.url);
      if (method === "Runtime.exceptionThrown") problems.push(["console-error", p.exceptionDetails.text + " " + (p.exceptionDetails.exception?.description || "")]);
      if (method === "Log.entryAdded" && p.entry.level === "error") {
        const u = p.entry.url || (p.entry.text.match(/https?:\/\/\S+/) || [])[0];
        report("console-error", `${p.entry.text} ${p.entry.url || ""}`, u);
      }
      if (method === "Network.responseReceived" && p.response.status >= 400) report("network-error", `${p.response.status} ${p.response.url}`, p.response.url);
      if (method === "Network.loadingFailed" && !p.canceled) report("network-error", `failed ${p.errorText} ${urls.get(p.requestId) || ""}`, urls.get(p.requestId));
    });
    await page.send("Network.enable");
    await page.send("Log.enable");
    await page.send("Runtime.enable");
    await page.send("Page.enable");
    await page.send("Emulation.setDeviceMetricsOverride", { width: vp.width, height: vp.height, deviceScaleFactor: vp.mobile ? 2 : 1, mobile: vp.mobile });
    await page.send("Emulation.setEmulatedMedia", { features: [{ name: "prefers-color-scheme", value: vp.scheme }] });
    await page.send("Page.addScriptToEvaluateOnNewDocument", { source: `
      window.__cls = 0;
      new PerformanceObserver((l) => { for (const e of l.getEntries()) if (!e.hadRecentInput) window.__cls += e.value; })
        .observe({ type: "layout-shift", buffered: true });` });
    await page.send("Page.navigate", { url });
    await new Promise((r) => setTimeout(r, url.endsWith("/try/") ? 4000 : 1200));
    const res = await page.send("Runtime.evaluate", { expression: `(${pageChecks.toString()})()`, returnByValue: true });
    for (const f of res.result.value) add(url, vp.name, f.rule, f.detail);
    const cls = (await page.send("Runtime.evaluate", { expression: "window.__cls", returnByValue: true })).result.value;
    if (cls > 0.1) add(url, vp.name, "cumulative-layout-shift", `CLS ${cls.toFixed(3)}, want 0.1 or less`);
    for (const [rule, detail] of problems) add(url, vp.name, rule, detail);
    const t = (await page.send("Runtime.evaluate", { expression: "document.title", returnByValue: true })).result.value;
    if (vp.name === "desktop") titles.set(url, t);
    page.close();
    audited++;
  }
}
const seenTitle = new Map();
for (const [url, t] of titles) {
  if (seenTitle.has(t)) add(url, "-", "unique-title", `same title as ${new URL(seenTitle.get(t)).pathname}: "${t}"`);
  else seenTitle.set(t, url);
}

// De-duplicate identical findings across viewports.
const key = (f) => `${f.page}|${f.rule}|${f.detail}`;
const merged = new Map();
for (const f of findings) {
  const k = key(f);
  if (merged.has(k)) merged.get(k).vp += `+${f.vp}`; else merged.set(k, { ...f });
}
console.log(`site audit: ${pages.length} pages x ${VIEWPORTS.length} viewports (${audited} page loads)`);
for (const f of merged.values()) console.log(`FAIL ${f.rule.padEnd(24)} ${f.page}  [${f.vp}]  ${f.detail}`);
for (const w of warnings) console.log(`WARN ${w}`);
if (merged.size === 0) console.log("PASS: no findings");
await cleanup();
process.exit(merged.size === 0 ? 0 : 1);
