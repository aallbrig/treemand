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
