#!/bin/sh
# Lint every CloudFormation template in infra/: cfn-lint (including the
# Project-tag rule in infra/rules) and checkov's CloudFormation policies.
set -eu
cd "$(dirname "$0")/.."

# "$@" becomes the templates; an unmatched glob stays literal, so test it.
set -- infra/*.yaml
if [ ! -e "$1" ]; then
  echo "[infra:lint] no templates in infra/"
  exit 0
fi
count=$#

cfn-lint --append-rules infra/rules --regions us-east-1 -- "$@"
# Rebuild "$@" as "-f template" pairs for checkov.
for t in "$@"; do set -- "$@" -f "$t"; shift; done
checkov --quiet --compact --framework cloudformation "$@"
echo "[infra:lint] $count template(s) clean"
