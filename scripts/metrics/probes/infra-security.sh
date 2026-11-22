#!/bin/sh
# Probe: AWS and supply-chain security posture (skill: treemand-infra-security).
#   static (always, offline): cfn-lint and checkov counts, GitHub Actions not
#     pinned to a commit SHA, workflow permissions, and template checks for S3
#     public access blocks, CloudFront security headers and the TLS minimum.
#   live (PROBE_LIVE=1, read-only): the site's headers, TLS and certificate,
#     public bucket listing, and with AWS_PROFILE the deployed CloudFront
#     settings the read-only role can see.
set -eu
cd "$(dirname "$0")/../../.."
exec python3 -I scripts/metrics/infra_security.py "$@"
