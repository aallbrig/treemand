"""Serve a built site with the response headers production sends.

Usage: python3 -I scripts/site/serve_with_headers.py PORT DIR [SITE_YAML]

CloudFront adds treemand.dev's security headers from the SecurityHeaders
response headers policy in infra/site.yaml. This server reads the
Content-Security-Policy and the custom headers (Permissions-Policy) from that
template and sends them with every response, so `task site:audit` loads each
page under the policy production enforces: anything the policy blocks shows
up as a console error there, before it ships.

`--print` instead of PORT prints the headers it would send and exits.
`--check` instead of PORT checks DIR against the policy: every inline
<script> in every page must be allowed by a 'sha256-...' source in script-src
(the policy has no 'unsafe-inline'). It prints each missing hash and exits 1.
"""

import base64
import functools
import hashlib
import http.server
import pathlib
import re
import sys

ROOT = pathlib.Path(__file__).resolve().parents[2]


def _scalar(raw):
    """A YAML scalar's text: whitespace folded, one pair of enclosing quotes removed."""
    value = " ".join(raw.split())
    if len(value) > 1 and value[0] == value[-1] and value[0] in "'\"":
        value = value[1:-1]
    return value


def policy_headers(site_yaml):
    """Return [(name, value)] for the CSP and custom headers in site_yaml."""
    text = pathlib.Path(site_yaml).read_text()
    start = text.find("SecurityHeaders:\n")
    if start < 0:
        raise SystemExit("serve_with_headers: no SecurityHeaders resource in " + str(site_yaml))
    # The resource ends at the next top-level resource (two-space indent).
    end = re.search(r"\n  [A-Za-z]+:\n", text[start + 1:])
    block = text[start:start + 1 + end.start()] if end else text[start:]
    headers = []
    csp = re.search(r"ContentSecurityPolicy:\s*\n\s+ContentSecurityPolicy:\s*(?:>-)?\s*(.+?)\n\s+Override:", block, re.S)
    if csp:
        headers.append(("Content-Security-Policy", _scalar(csp.group(1))))
    for name, value in re.findall(r"- Header:\s*(\S+)\s*\n\s*Value:\s*(.+?)\n\s*Override:", block, re.S):
        headers.append((name, _scalar(value)))
    return headers


def unhashed_inline_scripts(site_dir, csp):
    """Return [(page, hash)] for inline scripts in site_dir that csp does not allow."""
    allowed = set(re.findall(r"'(sha256-[A-Za-z0-9+/=]+)'", csp))
    missing = []
    for page in sorted(pathlib.Path(site_dir).rglob("*.html")):
        html = page.read_text(errors="replace")
        for m in re.finditer(r"<script(?![^>]*\bsrc=)([^>]*)>(.*?)</script>", html, re.S | re.I):
            kind = re.search(r"\btype=[\"']?([^\"' >]+)", m.group(1))
            if kind and kind.group(1).lower() not in ("module", "text/javascript"):
                continue  # a data block (JSON-LD and the like) never runs
            digest = "sha256-" + base64.b64encode(hashlib.sha256(m.group(2).encode()).digest()).decode()
            if digest not in allowed:
                missing.append((str(page.relative_to(site_dir)), digest))
    return missing


class Handler(http.server.SimpleHTTPRequestHandler):
    extra = []

    def end_headers(self):
        for name, value in self.extra:
            self.send_header(name, value)
        super().end_headers()

    def log_message(self, *args):
        pass


def main(argv):
    if len(argv) < 2:
        raise SystemExit(__doc__)
    site_yaml = argv[2] if len(argv) > 2 else ROOT / "infra" / "site.yaml"
    headers = policy_headers(site_yaml)
    if argv[0] == "--check":
        csp = dict(headers).get("Content-Security-Policy", "")
        missing = unhashed_inline_scripts(argv[1], csp)
        for page, digest in missing:
            print(f"serve_with_headers: {page}: inline script '{digest}' is not in the CSP script-src in {site_yaml}")
        sys.exit(1 if missing else 0)
    if argv[0] == "--print":
        for name, value in headers:
            print(f"{name}: {value}")
        return
    Handler.extra = headers
    handler = functools.partial(Handler, directory=argv[1])
    http.server.ThreadingHTTPServer(("127.0.0.1", int(argv[0])), handler).serve_forever()


if __name__ == "__main__":
    main(sys.argv[1:])
