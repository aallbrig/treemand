// Headless Chrome over the DevTools protocol, with no npm dependencies. The
// site drivers share it: scripts/site_audit.mjs, scripts/metrics/site_health.mjs,
// scripts/metrics/site_quality.mjs and scripts/site/design_evidence.mjs. Needs Node 22+ (the global WebSocket).
//
//   import { launchChrome } from "./cdp.mjs";
//   const chrome = await launchChrome(process.env.CHROME, "site-audit-");
//   const page = await chrome.newPage();
//   await page.send("Page.navigate", { url });
//   await page.close();
//   await chrome.close();
import { spawn } from "node:child_process";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

export async function launchChrome(binary, profilePrefix = "cdp-", extraArgs = []) {
  const profile = mkdtempSync(join(tmpdir(), profilePrefix));
  const chrome = spawn(binary, [
    "--headless=new", "--no-sandbox", "--disable-gpu", "--remote-debugging-port=0",
    `--user-data-dir=${profile}`, ...extraArgs, "about:blank",
  ], { stdio: ["ignore", "ignore", "pipe"] });
  // Chrome keeps writing into its profile until it has exited, so deleting the
  // directory right after kill() races with it (ENOTEMPTY). Wait for the exit,
  // retry the delete, and never let cleanup change a caller's verdict.
  const removeProfile = () => {
    try { rmSync(profile, { recursive: true, force: true, maxRetries: 10, retryDelay: 200 }); } catch { /* best effort */ }
  };
  const exited = new Promise((resolve) => chrome.once("exit", resolve));
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

  // One tab. `send` resolves with a command's result; `on` receives every event
  // as (method, params); `id` is the target id, which is also the main frame's id.
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
      } else if (msg.method) for (const l of listeners) l(msg.method, msg.params);
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

  async function close() {
    chrome.kill();
    await Promise.race([exited, new Promise((r) => setTimeout(r, 5000))]);
    removeProfile();
  }

  return { newPage, close };
}
