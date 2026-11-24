// Site-wide behaviour: the light/dark toggle and copy buttons on code blocks.
// Everything works without this script: the theme follows the system and code
// can be selected by hand.
(function () {
  "use strict";
  var root = document.documentElement;
  var KEY = "treemand.theme";
  var media = window.matchMedia ? window.matchMedia("(prefers-color-scheme: dark)") : null;

  // ── Theme ───────────────────────────────────────────────────────────────
  function current() {
    var t = root.dataset.theme;
    if (t === "light" || t === "dark") return t;
    return media && media.matches ? "dark" : "light";
  }
  var toggle = document.getElementById("theme-toggle");
  if (toggle) {
    var sync = function () {
      var dark = current() === "dark";
      toggle.setAttribute("aria-pressed", String(dark));
      toggle.title = dark ? "Switch to light theme" : "Switch to dark theme";
    };
    toggle.hidden = false;
    sync();
    toggle.addEventListener("click", function () {
      var next = current() === "dark" ? "light" : "dark";
      root.dataset.theme = next;
      try { localStorage.setItem(KEY, next); } catch (e) { /* storage may be blocked */ }
      sync();
    });
    if (media && media.addEventListener) media.addEventListener("change", sync);
  }

  // ── OS tabs (home install one-liner) ────────────────────────────────────
  // Without JS every OS's command shows with its label; with JS one shows,
  // the OS the head script picked (shared with the Installation page).
  Array.prototype.forEach.call(document.querySelectorAll("[data-tabs]"), function (box) {
    var tabs = box.querySelectorAll(".qi-tab");
    var panels = box.querySelectorAll(".qi-panel");
    if (!tabs.length) return;
    var os = root.dataset.os || "linux";
    function show(id) {
      root.dataset.os = id;
      Array.prototype.forEach.call(panels, function (p) { p.hidden = p.dataset.tab !== id; });
      Array.prototype.forEach.call(tabs, function (t) { t.setAttribute("aria-pressed", String(t.dataset.tab === id)); });
    }
    Array.prototype.forEach.call(tabs, function (t) {
      t.addEventListener("click", function () {
        show(t.dataset.tab);
        try { localStorage.setItem("treemand.os", t.dataset.tab); } catch (e) { /* storage may be blocked */ }
      });
    });
    box.classList.add("js-tabs");
    show(box.querySelector('.qi-panel[data-tab="' + os + '"]') ? os : tabs[0].dataset.tab);
  });

  // ── Copy buttons ────────────────────────────────────────────────────────
  // The Installation page has its own named buttons (install.js); the Try
  // page's terminal is not a code block.
  function copyText(text) {
    if (navigator.clipboard && window.isSecureContext) return navigator.clipboard.writeText(text);
    return new Promise(function (resolve, reject) {
      var ta = document.createElement("textarea");
      ta.value = text;
      ta.setAttribute("readonly", "");
      ta.style.position = "fixed";
      ta.style.opacity = "0";
      document.body.appendChild(ta);
      ta.select();
      var ok = false;
      try { ok = document.execCommand("copy"); } catch (e) { ok = false; }
      document.body.removeChild(ta);
      if (ok) resolve(); else reject(new Error("copy failed"));
    });
  }
  var blocks = document.querySelectorAll("main pre");
  if (!blocks.length) return;
  var status = document.createElement("div");
  status.className = "visually-hidden";
  status.setAttribute("role", "status");
  document.body.appendChild(status);
  Array.prototype.forEach.call(blocks, function (pre) {
    if (pre.closest(".cmd, .try-stage, .no-copy")) return;
    var code = pre.querySelector("code") || pre;
    var box = pre.parentElement && pre.parentElement.classList.contains("highlight") ? pre.parentElement : pre;
    box.classList.add("copyable");
    var button = document.createElement("button");
    button.type = "button";
    button.className = "copy-btn";
    button.textContent = "Copy";
    button.setAttribute("aria-label", "Copy code");
    var timer = null;
    button.addEventListener("click", function () {
      copyText(code.textContent.replace(/\n$/, "")).then(function () {
        button.textContent = "Copied";
        button.classList.add("done");
        status.textContent = "Copied to the clipboard";
      }, function () {
        button.textContent = "Press Ctrl+C";
        var range = document.createRange();
        range.selectNodeContents(code);
        var sel = window.getSelection();
        sel.removeAllRanges();
        sel.addRange(range);
      });
      clearTimeout(timer);
      timer = setTimeout(function () { button.textContent = "Copy"; button.classList.remove("done"); }, 2000);
    });
    box.appendChild(button);
  });
})();

// ── Search (Pagefind) ─────────────────────────────────────────────────────
// The index is built after hugo (scripts/site_search_index.sh). The UI loads on
// first open; Ctrl+K, Cmd+K or "/" opens it and Esc closes it.
(function () {
  "use strict";
  var open = document.getElementById("search-open");
  var dialog = document.getElementById("search-dialog");
  var box = document.getElementById("search");
  if (!open || !dialog || !box || typeof dialog.showModal !== "function") return;
  var base = box.dataset.base;
  var mac = /mac|iphone|ipad/i.test(navigator.platform || "");
  var kbd = open.querySelector(".search-kbd");
  if (kbd && mac) kbd.textContent = "⌘K";
  open.hidden = false;
  var loaded = null;

  function load() {
    if (loaded) return loaded;
    loaded = new Promise(function (resolve, reject) {
      var css = document.createElement("link");
      css.rel = "stylesheet";
      css.href = base + "pagefind-ui.css";
      document.head.appendChild(css);
      var js = document.createElement("script");
      js.src = base + "pagefind-ui.js";
      js.onload = function () {
        /* global PagefindUI */
        new PagefindUI({ element: "#search", showSubResults: true, showImages: false, resetStyles: false, autofocus: true });
        resolve();
      };
      js.onerror = function () {
        box.innerHTML = '<p class="search-missing">Search is not available in this build (run <code>task site:build</code>).</p>';
        reject(new Error("pagefind missing"));
      };
      document.head.appendChild(js);
    });
    return loaded;
  }
  function show() {
    if (dialog.open) return;
    dialog.showModal();
    load().then(function () {
      var input = box.querySelector("input");
      if (input) input.focus();
    }, function () { /* message already shown */ });
  }
  open.addEventListener("click", show);
  document.getElementById("search-close").addEventListener("click", function () { dialog.close(); });
  dialog.addEventListener("click", function (e) { if (e.target === dialog) dialog.close(); });
  document.addEventListener("keydown", function (e) {
    if (e.target && e.target.closest && e.target.closest(".try-stage")) return; // keys belong to the TUI
    var typing = /^(INPUT|TEXTAREA|SELECT)$/.test((e.target && e.target.tagName) || "") || (e.target && e.target.isContentEditable);
    if ((e.key === "k" || e.key === "K") && (e.metaKey || e.ctrlKey)) { e.preventDefault(); show(); }
    else if (e.key === "/" && !typing && !e.metaKey && !e.ctrlKey && !e.altKey) { e.preventDefault(); show(); }
  });
})();
