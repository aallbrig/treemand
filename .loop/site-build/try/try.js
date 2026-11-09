// In-browser treemand: xterm.js <-> Go/wasm Bubble Tea program.
(() => {
  const BASE = window.TREEMAND_BASE || 'try/';
  const IDEAS = {
    git: ['Press <kbd>Ctrl+F</kbd> and type <code>force push</code> — finds flags by description',
          'Search <code>stash pop</code> or <code>worktree</code>', 'Press <kbd>?</kbd> for every key binding'],
    docker: ['<kbd>Ctrl+F</kbd> <code>privileged</code> — jump straight to the flag', 'Search <code>volume prune</code>', 'Press <kbd>T</kbd> to cycle tree styles'],
    kubectl: ['<kbd>Ctrl+F</kbd> <code>namespace</code> — every command and flag that mentions it', 'Search <code>rollout undo</code>', 'Press <kbd>f</kbd> on a command to pick flags'],
    gh: ['<kbd>Ctrl+F</kbd> <code>merge</code> — all merge-related commands and flags', 'Search <code>pr create draft</code>', 'Arrow into <code>pr</code> and press Enter to build a command'],
    stripe: ['<kbd>Ctrl+F</kbd> <code>refund</code> — ~1000 API operations, one search', 'Search <code>idempotency</code> — a flag on every request', 'Search <code>webhook listen</code>'],
    aws: ['<kbd>Ctrl+F</kbd> <code>s3 bucket</code> — 400+ services, fuzzy-matched', 'Search <code>lambda invoke</code>', 'Press <kbd>S</kbd> to toggle section headers'],
    treemand: ['Explore the tool that\'s running right now', '<kbd>Ctrl+F</kbd> <code>fuzzy</code>', 'Press <kbd>t</kbd> to cycle tree styles'],
  };
  const KEYS = {
    up: '\x1b[A', down: '\x1b[B', right: '\x1b[C', left: '\x1b[D', enter: '\r', esc: '\x1b',
    tab: '\t', 'ctrl-f': '\x06', slash: '/', help: '?',
  };

  const $ = (id) => document.getElementById(id);
  const statusEl = $('try-status');
  const endedEl = $('try-ended');
  const picker = $('try-picker');
  const params = new URLSearchParams(location.search);
  let current = IDEAS[params.get('cli')] ? params.get('cli') : 'git';
  let wasmReady = false;
  let expectExit = false;
  let started = false;

  const term = new Terminal({
    fontFamily: '"Fira Code", Menlo, Consolas, "DejaVu Sans Mono", monospace',
    fontSize: 14,
    cursorBlink: false,
    cursorStyle: 'bar',
    scrollback: 0,
    theme: { background: '#0d1117', foreground: '#e6edf3', cursor: '#0d1117' },
  });
  const fit = new FitAddon.FitAddon();
  term.loadAddon(fit);
  term.open($('try-term'));

  // FitAddon over-counts by a few pixels; keep one row spare so the TUI's
  // bottom status line is never clipped.
  function fitTerm() {
    fit.fit();
    if (term.rows > 6) term.resize(term.cols, term.rows - 1);
  }

  window.treemandOutput = (s) => term.write(s);
  window.treemandExited = () => {
    if (expectExit) { expectExit = false; return; }
    term.write('\x1b[?1049l\x1b[?25h');
    endedEl.hidden = false;
  };
  term.onData((d) => { if (wasmReady && started) treemandWrite(d); });

  function renderIdeas() {
    $('try-ideas').innerHTML = IDEAS[current].map((i) => `<li>${i}</li>`).join('');
  }

  function setActive() {
    picker.querySelectorAll('button').forEach((b) => b.classList.toggle('active', b.dataset.cli === current));
  }

  async function startCli(name) {
    current = name;
    setActive();
    renderIdeas();
    endedEl.hidden = true;
    statusEl.hidden = false;
    statusEl.textContent = `Loading ${name}…`;
    if (started) { expectExit = true; treemandStop(); }
    term.reset();
    let json;
    try {
      const r = await fetch(`${BASE}trees/${name}.json`);
      if (!r.ok) throw new Error(r.status);
      json = await r.text();
    } catch (e) {
      statusEl.textContent = `Could not load the ${name} snapshot.`;
      return;
    }
    fitTerm();
    const err = treemandStart(json, term.cols, term.rows);
    if (err) { statusEl.textContent = 'Error: ' + err; return; }
    started = true;
    statusEl.hidden = true;
    term.focus();
    const u = new URL(location.href);
    u.searchParams.set('cli', name);
    history.replaceState(null, '', u);
  }

  async function loadWasm() {
    if (!window.WebAssembly) { statusEl.textContent = 'This page needs WebAssembly, which your browser lacks.'; return; }
    const go = new Go();
    const ready = new Promise((res) => { window.treemandReady = res; });
    let inst;
    try {
      const url = BASE + 'treemand.wasm';
      if (WebAssembly.instantiateStreaming) {
        try { inst = (await WebAssembly.instantiateStreaming(fetch(url), go.importObject)).instance; }
        catch (_) { /* wrong MIME type: fall back */ }
      }
      if (!inst) {
        const buf = await (await fetch(url)).arrayBuffer();
        inst = (await WebAssembly.instantiate(buf, go.importObject)).instance;
      }
    } catch (e) {
      statusEl.textContent = 'Could not load treemand.wasm: ' + e.message;
      return;
    }
    go.run(inst);
    await ready;
    wasmReady = true;
    startCli(current);
  }

  picker.addEventListener('click', (e) => {
    const b = e.target.closest('button[data-cli]');
    if (b && wasmReady) startCli(b.dataset.cli);
  });
  $('try-keys').addEventListener('click', (e) => {
    const b = e.target.closest('button[data-key]');
    if (b && wasmReady && started) { treemandWrite(KEYS[b.dataset.key]); term.focus(); }
  });
  $('try-restart').addEventListener('click', () => startCli(current));

  let t;
  window.addEventListener('resize', () => {
    clearTimeout(t);
    t = setTimeout(() => {
      fitTerm();
      if (wasmReady && started) treemandResize(term.cols, term.rows);
    }, 120);
  });

  setActive();
  renderIdeas();
  loadWasm();
})();
