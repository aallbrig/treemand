// Runs inline in <head>, before first paint: marks JS as on and applies the
// saved theme and OS tab so the page does not flash or shift. baseof.html inlines
// it minified, so dev and production builds emit the same bytes; the site's
// Content-Security-Policy allows exactly those bytes by hash (infra/site.yaml),
// and scripts/site/csp_test.sh fails when an edit here changes the hash.
(function(d){d.classList.add("js");var k=["macos","linux","windows"],o=location.hash.slice(1),s="";try{var t=localStorage.getItem("treemand.theme");if(t==="light"||t==="dark")d.dataset.theme=t;s=localStorage.getItem("treemand.os")}catch(e){}if(k.indexOf(o)<0)o=s;if(k.indexOf(o)<0){var u=(((navigator.userAgentData||{}).platform||navigator.platform||"")+" "+navigator.userAgent).toLowerCase();o=/win/.test(u)&&!/darwin/.test(u)?"windows":/mac|iphone|ipad|darwin/.test(u)?"macos":"linux"}d.dataset.os=o})(document.documentElement);
