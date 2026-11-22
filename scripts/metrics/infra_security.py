#!/usr/bin/env python3
"""infra_security.py: the infra-security probe's measurements (AWS and supply chain posture).

    infra_security.py [--root DIR]

Prints one probe result (see scripts/metrics/measure.py). DIR (default: the
repo) holds the .github/workflows and infra/*.yaml being judged.

Static, deterministic, offline (CI gates on these):
- cfn_lint_findings, checkov_failed, checkov_skipped: what `task infra:lint` runs.
- actions_unpinned: `uses:` refs that are not a full commit SHA (docker refs: not a digest).
- workflows_without_permissions: no top-level `permissions:` block.
- workflow_write_scopes_broad: workflow-level write scopes that a job inherits
  in a workflow with more than one job (heuristic: the jobs may not all need them).
- s3_without_public_access_block, cloudfront_without_security_headers,
  cloudfront_tls_below_1_2_2021, iam_full_service_wildcards: the cfn-lint
  rules in infra/security_rules.

Live, read-only, only with PROBE_LIVE=1 (curl and openssl against the public
site; the aws CLI only with AWS_PROFILE set, and only calls the read-only role
allows). Checks the role cannot make are named in details as not checked.
"""

import json
import os
import re
import shutil
import subprocess
import sys
from datetime import UTC, datetime
from pathlib import Path
from typing import Any

type Obj = dict[str, Any]

REPO = Path(__file__).resolve().parents[2]
SECURITY_RULES = REPO / "infra" / "security_rules"
PROJECT_RULES = REPO / "infra" / "rules"
TEMPLATE_RULES = {
    "W9101": "s3_without_public_access_block",
    "W9102": "cloudfront_without_security_headers",
    "W9103": "cloudfront_tls_below_1_2_2021",
    "W9104": "iam_full_service_wildcards",
}
LIVE_METRICS = (
    "live_headers_missing",
    "live_headers_recommended_missing",
    "live_http_not_redirected",
    "live_legacy_tls_accepted",
    "live_cert_invalid",
    "live_cert_days_left",
    "live_buckets_listable",
    "live_bucket_objects_public",
)
AWS_METRICS = ("live_cf_tls_below_policy", "live_cf_without_headers_policy")
# Checks a full audit would make that the read-only role (docs/agents/prod-readonly.md) cannot.
NOT_CHECKED = {
    "s3 PublicAccessBlock as deployed": "needs s3:GetPublicAccessBlock, not granted to the read-only role",
    "response headers policy contents as deployed": "needs cloudfront:GetResponseHeadersPolicy, not granted",
}
DOMAIN = "treemand.dev"
REGION = "us-east-1"
HSTS_MIN_AGE = 31536000
TIMEOUT = 30
HTTP_OK = 200
SHA = re.compile(r"[0-9a-f]{40}")
USES = re.compile(r"""^\s*(?:-\s+)?uses:\s*["']?([^"'\s#]+)""")


def run(*cmd: str, stdin: str = "") -> subprocess.CompletedProcess[str]:
    try:
        return subprocess.run(cmd, check=False, capture_output=True, text=True, input=stdin, timeout=TIMEOUT)
    except subprocess.TimeoutExpired:
        return subprocess.CompletedProcess(cmd, 124, "", "timeout")


def need(tool: str) -> None:
    if shutil.which(tool) is None:
        sys.exit(f"infra-security: {tool} is not installed (task setup)")


# ── workflows ────────────────────────────────────────────────────────────


def _indent(line: str) -> int:
    return len(line) - len(line.lstrip(" "))


def _content(lines: list[str]) -> list[str]:
    return [ln.split(" #", 1)[0].rstrip() for ln in lines if ln.strip() and not ln.lstrip().startswith("#")]


def _top_permissions(lines: list[str]) -> list[str] | None:
    """Workflow-level write scopes, or None when there is no top-level block."""
    for i, line in enumerate(lines):
        m = re.match(r"^permissions:\s*(.*)$", line)
        if not m:
            continue
        inline = m.group(1).strip()
        if inline == "write-all":
            return ["write-all"]
        if inline:
            pairs = re.findall(r"([\w-]+)\s*:\s*(\w+)", inline)
        else:
            body = []
            for ln in lines[i + 1 :]:
                if _indent(ln) == 0:
                    break
                body.append(ln)
            pairs = [p for ln in body for p in re.findall(r"^\s+([\w-]+):\s*(\w+)", ln)]
        return [scope for scope, level in pairs if level == "write"]
    return None


def _jobs(lines: list[str]) -> dict[str, bool]:
    """Job name -> whether the job declares its own permissions."""
    jobs: dict[str, bool] = {}
    start = next((i for i, ln in enumerate(lines) if re.match(r"^jobs:\s*$", ln)), None)
    if start is None:
        return jobs
    body = []
    for ln in lines[start + 1 :]:
        if _indent(ln) == 0:
            break
        body.append(ln)
    if not body:
        return jobs
    job_indent = _indent(body[0])
    current, key_indent = "", 0
    for ln in body:
        if _indent(ln) == job_indent:
            current, key_indent = ln.strip().rstrip(":"), 0
            jobs[current] = False
            continue
        key_indent = key_indent or _indent(ln)  # the job's own keys sit at its first child's indent
        if _indent(ln) == key_indent and ln.strip().startswith("permissions:"):
            jobs[current] = True
    return jobs


def _unpinned(ref: str) -> bool:
    if ref.startswith("./"):
        return False
    if ref.startswith("docker://"):
        return "@sha256:" not in ref
    return SHA.fullmatch(ref.rsplit("@", 1)[-1]) is None


def workflows(root: Path) -> tuple[Obj, list[Obj]]:
    unpinned: list[str] = []
    no_permissions: list[str] = []
    broad: list[str] = []
    files = sorted([*root.glob(".github/workflows/*.yml"), *root.glob(".github/workflows/*.yaml")])
    for path in files:
        lines = _content(path.read_text().splitlines())
        rel = path.relative_to(root).as_posix()
        for ln in lines:
            m = USES.match(ln)
            if m and _unpinned(m.group(1)):
                unpinned.append(f"{rel}: {m.group(1)}")
        writes = _top_permissions(lines)
        if writes is None:
            no_permissions.append(rel)
            continue
        jobs = _jobs(lines)
        inheriting = sorted(j for j, own in jobs.items() if not own)
        if len(jobs) > 1 and inheriting:
            broad += [f"{rel}: {scope} (inherited by {', '.join(inheriting)})" for scope in writes]
    metrics = {
        "actions_unpinned": len(unpinned),
        "workflows_without_permissions": len(no_permissions),
        "workflow_write_scopes_broad": len(broad),
    }
    details: list[Obj] = [
        {"actions_unpinned": sorted(set(unpinned))},
        {"workflows_without_permissions": no_permissions},
        {"workflow_write_scopes_broad": broad},
    ]
    return metrics, details


# ── templates ────────────────────────────────────────────────────────────


def templates(root: Path) -> tuple[Obj, list[Obj]]:
    files = sorted(str(p) for p in root.glob("infra/*.yaml"))
    metrics: Obj = {"cfn_lint_findings": 0, "checkov_failed": 0, "checkov_skipped": 0}
    metrics.update(dict.fromkeys(TEMPLATE_RULES.values(), 0))
    if not files:
        return metrics, [{"templates": "none in infra/"}]
    need("cfn-lint")
    need("checkov")
    rules = [str(PROJECT_RULES), str(SECURITY_RULES)]
    p = run("cfn-lint", "--format", "json", "--append-rules", *rules, "--regions", REGION, "--", *files)
    try:
        matches = json.loads(p.stdout or "[]")
    except json.JSONDecodeError:
        sys.exit(f"infra-security: cfn-lint output is not JSON: {p.stderr.strip()[:300]}")
    lint: list[str] = []
    posture: dict[str, list[str]] = {name: [] for name in TEMPLATE_RULES.values()}
    for m in matches:
        where = f"{Path(m['Filename']).name}:{m['Location']['Start']['LineNumber']}"
        rid = m["Rule"]["Id"]
        if rid in TEMPLATE_RULES:
            posture[TEMPLATE_RULES[rid]].append(f"{where} {m['Message']}")
        else:
            lint.append(f"{where} {rid} {m['Message']}")
    metrics["cfn_lint_findings"] = len(lint)
    for name, found in posture.items():
        metrics[name] = len(found)

    # Without --quiet, so the report lists the skipped checks too.
    args = ["checkov", "--compact", "--framework", "cloudformation", "-o", "json"]
    for f in files:
        args += ["-f", f]
    c = run(*args)
    try:
        report = json.loads(c.stdout or "{}")
    except json.JSONDecodeError:
        sys.exit(f"infra-security: checkov output is not JSON: {c.stderr.strip()[:300]}")
    failed: list[str] = []
    skipped: list[str] = []
    for r in report if isinstance(report, list) else [report]:
        results = r.get("results") or {}
        failed += [
            f"{Path(x['file_path']).name} {x['check_id']} {x['resource']}" for x in results.get("failed_checks", [])
        ]
        skipped += [
            f"{Path(x['file_path']).name} {x['check_id']} {x['resource']}" for x in results.get("skipped_checks", [])
        ]
        failed += [f"{Path(f).name} parsing error" for f in results.get("parsing_errors", [])]
    metrics["checkov_failed"] = len(failed)
    metrics["checkov_skipped"] = len(skipped)
    details: list[Obj] = [
        {"cfn_lint_findings": lint},
        {"template_checks": posture},
        {"checkov_failed": failed},
        {"checkov_skipped": skipped},
    ]
    return metrics, details


# ── live, read-only ──────────────────────────────────────────────────────


def _headers(raw: str) -> tuple[int, dict[str, str]]:
    """Status and headers of the last response in curl -I output."""
    blocks = [b for b in re.split(r"\r?\n\r?\n", raw.strip()) if b.strip()]
    if not blocks:
        return 0, {}
    lines = blocks[-1].splitlines()
    status = re.match(r"HTTP/\S+\s+(\d+)", lines[0] if lines else "")
    headers = {}
    for ln in lines[1:]:
        if ":" in ln:
            k, v = ln.split(":", 1)
            headers[k.strip().lower()] = v.strip()
    return int(status.group(1)) if status else 0, headers


def _missing_required(h: dict[str, str]) -> list[str]:
    missing = []
    age = re.search(r"max-age=(\d+)", h.get("strict-transport-security", ""))
    if not age or int(age.group(1)) < HSTS_MIN_AGE:
        missing.append(f"strict-transport-security (max-age >= {HSTS_MIN_AGE})")
    if h.get("x-content-type-options", "").lower() != "nosniff":
        missing.append("x-content-type-options: nosniff")
    if h.get("x-frame-options", "").upper() not in ("DENY", "SAMEORIGIN"):
        missing.append("x-frame-options")
    if not h.get("referrer-policy"):
        missing.append("referrer-policy")
    return missing


def _http_code(url: str) -> int:
    p = run("curl", "-sS", "-o", "/dev/null", "-w", "%{http_code}", "--max-time", "20", url)
    return int(p.stdout.strip()) if p.stdout.strip().isdigit() else 0


def _bucket_names(root: Path) -> list[str]:
    site = root / "infra" / "site.yaml"
    text = site.read_text() if site.exists() else ""
    return re.findall(r"\w+BucketName:\s*\n\s+Type:\s*String\s*\n\s+Default:\s*(\S+)", text)


def _tls(details: list[Obj]) -> Obj:
    target = f"{DOMAIN}:443"
    p = run("openssl", "s_client", "-connect", target, "-servername", DOMAIN, "-showcerts")
    proto = re.search(r"Protocol\s*:\s*(\S+)", p.stdout)
    verified = re.search(r"Verify return code:\s*0\b", p.stdout) is not None
    pem = re.search(r"-----BEGIN CERTIFICATE-----.*?(?:-----END CERTIFICATE-----|\Z)", p.stdout, re.DOTALL)
    end = run("openssl", "x509", "-noout", "-enddate", stdin=pem.group(0) if pem else "")
    days = -1
    m = re.search(r"notAfter=(.+)", end.stdout)
    if m:
        expires = datetime.strptime(m.group(1).strip(), "%b %d %H:%M:%S %Y %Z").replace(tzinfo=UTC)
        days = (expires - datetime.now(UTC)).days
    legacy = []
    for flag in ("-tls1", "-tls1_1"):
        q = run("openssl", "s_client", flag, "-cipher", "DEFAULT@SECLEVEL=0", "-connect", target, "-servername", DOMAIN)
        # A refused handshake still prints the attempted protocol: the exit
        # status and a real cipher are what show it was accepted.
        if q.returncode == 0 and "Cipher is (NONE)" not in q.stdout:
            legacy.append(flag.lstrip("-"))
    details.append(
        {
            "tls": {
                "negotiated": proto.group(1) if proto else "unknown",
                "verified": verified,
                "cert_days_left": days,
                "legacy_accepted": legacy,
            }
        }
    )
    return {
        "live_legacy_tls_accepted": len(legacy),
        "live_cert_invalid": int(not verified or days < 0),
        "live_cert_days_left": days,
    }


def _aws(details: list[Obj], skipped: dict[str, str]) -> Obj:
    if not os.environ.get("AWS_PROFILE") or shutil.which("aws") is None:
        skipped.update(dict.fromkeys(AWS_METRICS, "AWS_PROFILE not set or aws CLI missing"))
        return {}
    query = (
        f"DistributionList.Items[?Aliases.Items && contains(Aliases.Items, '{DOMAIN}')]"
        ".[ViewerCertificate.MinimumProtocolVersion, DefaultCacheBehavior.ResponseHeadersPolicyId]"
    )
    p = run("aws", "cloudfront", "list-distributions", "--region", REGION, "--output", "text", "--query", query)
    row = p.stdout.split()
    if p.returncode != 0 or len(row) < 2:  # noqa: PLR2004 - two queried fields
        # stderr is never echoed: AWS errors name the caller's ARN.
        skipped.update(dict.fromkeys(AWS_METRICS, f"cloudfront list-distributions failed (exit {p.returncode})"))
        return {}
    tls, policy = row[0], row[1]
    details.append(
        {"cloudfront_deployed": {"minimum_protocol_version": tls, "headers_policy_attached": policy != "None"}}
    )
    return {
        "live_cf_tls_below_policy": int(tls not in ("TLSv1.2_2021", "TLSv1.2_2025", "TLSv1.3_2025")),
        "live_cf_without_headers_policy": int(policy == "None"),
    }


def live(root: Path) -> tuple[Obj, list[Obj], dict[str, str]]:
    need("curl")
    need("openssl")
    details: list[Obj] = []
    skipped: dict[str, str] = {}
    _, h = _headers(run("curl", "-sS", "-I", "--max-time", "20", f"https://{DOMAIN}/").stdout)
    missing = _missing_required(h)
    recommended = [k for k in ("content-security-policy", "permissions-policy") if not h.get(k)]
    status, plain = _headers(run("curl", "-sS", "-I", "--max-time", "20", f"http://{DOMAIN}/").stdout)
    redirected = status in (301, 308) and plain.get("location", "").startswith(f"https://{DOMAIN}")
    buckets = _bucket_names(root)
    # Anonymous GETs on the regional endpoints: 403 is the healthy answer.
    codes = {b: _http_code(f"https://{b}.s3.{REGION}.amazonaws.com/") for b in buckets}
    codes.update(
        {f"{b}/index.html": _http_code(f"https://{b}.s3.{REGION}.amazonaws.com/index.html") for b in buckets[:1]}
    )
    listable = [b for b in buckets if codes[b] == HTTP_OK]
    public = [k for k in codes if "/" in k and codes[k] == HTTP_OK]
    details += [
        {"headers_missing": missing, "headers_recommended_missing": recommended},
        {"http_redirect": {"status": status, "to_https": redirected}},
        {"bucket_anonymous_get_status": codes},
    ]
    metrics: Obj = {
        "live_headers_missing": len(missing),
        "live_headers_recommended_missing": len(recommended),
        "live_http_not_redirected": int(not redirected),
        "live_buckets_listable": len(listable),
        "live_bucket_objects_public": len(public),
    }
    metrics.update(_tls(details))
    metrics.update(_aws(details, skipped))
    details.append({"not_checked": NOT_CHECKED})
    return metrics, details, skipped


def main(argv: list[str]) -> int:
    root = Path(argv[argv.index("--root") + 1]).resolve() if "--root" in argv else REPO
    metrics, details = workflows(root)
    t_metrics, t_details = templates(root)
    metrics.update(t_metrics)
    details += t_details
    skipped: dict[str, str] = {}
    if os.environ.get("PROBE_LIVE") == "1":
        l_metrics, l_details, skipped = live(root)
        metrics.update(l_metrics)
        details += l_details
    else:
        skipped = dict.fromkeys((*LIVE_METRICS, *AWS_METRICS), "PROBE_LIVE not set")
    result: Obj = {"probe": "infra-security", "metrics": metrics, "details": details}
    if skipped:
        result["skipped"] = skipped
    print(json.dumps(result))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
