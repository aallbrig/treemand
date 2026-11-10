// Installation page: pick the visitor's OS, let them override it, and add copy
// buttons. Everything works without this script (all panels are visible); this
// only narrows the page to one OS and adds the buttons.
(function () {
  "use strict";
  var picker = document.getElementById("install-picker");
  if (!picker) return;

  var panels = Array.prototype.slice.call(picker.querySelectorAll(".os-panel"));
  var buttons = Array.prototype.slice.call(picker.querySelectorAll(".os-btn"));
  var detectedNote = document.getElementById("os-detected");
  var names = { macos: "macOS", linux: "Linux", windows: "Windows" };
  var KEY = "treemand.os";

  function detect() {
    var platform = (navigator.userAgentData && navigator.userAgentData.platform) || navigator.platform || "";
    var s = (platform + " " + (navigator.userAgent || "")).toLowerCase();
    if (/win/.test(s) && !/darwin/.test(s)) return "windows";
    if (/mac|iphone|ipad|darwin/.test(s)) return "macos";
    if (/linux|x11|chrome os|cros|android|bsd/.test(s)) return "linux";
    return null;
  }

  function stored() {
    try { return window.localStorage.getItem(KEY); } catch (e) { return null; }
  }
  function remember(os) {
    try { window.localStorage.setItem(KEY, os); } catch (e) { /* storage may be blocked */ }
  }
  function known(os) { return panels.some(function (p) { return p.dataset.os === os; }); }

  function show(os, byUser) {
    panels.forEach(function (p) { p.hidden = p.dataset.os !== os; });
    buttons.forEach(function (b) { b.setAttribute("aria-pressed", String(b.dataset.os === os)); });
    if (byUser) remember(os);
  }

  var detected = detect();
  var fromHash = (window.location.hash || "").replace("#", "");
  var initial = known(fromHash) ? fromHash : known(stored()) ? stored() : known(detected) ? detected : "linux";

  document.getElementById("os-switch").hidden = false;
  buttons.forEach(function (b) {
    b.addEventListener("click", function () {
      show(b.dataset.os, true);
      if (detectedNote) detectedNote.hidden = true;
    });
  });
  show(initial, false);
  if (detectedNote && known(detected) && initial === detected && !known(fromHash)) {
    detectedNote.textContent = "We think you are on " + names[detected] + ". Not right? Pick your system above.";
    detectedNote.hidden = false;
  }

  // Copy buttons.
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
      ok ? resolve() : reject(new Error("copy failed"));
    });
  }

  // One polite live region announces copy results to screen readers.
  var status = document.createElement("div");
  status.className = "sr-only";
  status.setAttribute("role", "status");
  picker.appendChild(status);

  Array.prototype.forEach.call(picker.querySelectorAll(".cmd"), function (box) {
    var code = box.querySelector("code");
    var channel = box.closest(".channel");
    var title = channel && channel.querySelector("h3") ? channel.querySelector("h3").firstChild.textContent.trim() : "";
    var button = document.createElement("button");
    button.type = "button";
    button.className = "copy-btn";
    button.textContent = "Copy";
    button.setAttribute("aria-label", "Copy the " + title + " commands");
    var label = button.textContent;
    var timer = null;
    button.addEventListener("click", function () {
      copyText(code.textContent).then(function () {
        button.textContent = "Copied";
        button.classList.add("done");
        status.textContent = "Copied the " + title + " commands to the clipboard";
      }, function () {
        button.textContent = "Press Ctrl+C";
        status.textContent = "Copy failed; the commands are selected, press Ctrl+C";
        var range = document.createRange();
        range.selectNodeContents(code);
        var sel = window.getSelection();
        sel.removeAllRanges();
        sel.addRange(range);
      });
      clearTimeout(timer);
      timer = setTimeout(function () { button.textContent = label; button.classList.remove("done"); }, 2000);
    });
    box.appendChild(button);
  });
})();
