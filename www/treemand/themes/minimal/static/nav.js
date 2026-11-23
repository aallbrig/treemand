// The narrow-screen menu toggle. Loaded without defer right after the header,
// so the toggle appears before first paint, as the inline script it replaced did.
(function () {
  var nav = document.querySelector('header nav');
  var btn = nav.querySelector('.nav-toggle');
  btn.hidden = false;
  nav.classList.add('js-nav');
  function setOpen(open) {
    btn.setAttribute('aria-expanded', String(open));
    nav.classList.toggle('open', open);
  }
  btn.addEventListener('click', function () { setOpen(btn.getAttribute('aria-expanded') !== 'true'); });
  document.addEventListener('keydown', function (e) {
    if (e.key === 'Escape' && nav.classList.contains('open')) { setOpen(false); btn.focus(); }
  });
})();
