#!/usr/bin/env bash
# Lint every CloudFormation template in infra/: cfn-lint (including the
# Project-tag rule in infra/rules) and checkov's CloudFormation policies.
set -euo pipefail
cd "$(dirname "$0")/.."

shopt -s nullglob
templates=(infra/*.yaml)
if [ ${#templates[@]} -eq 0 ]; then
  echo "[infra:lint] no templates in infra/"
  exit 0
fi

cfn-lint --append-rules infra/rules --regions us-east-1 -- "${templates[@]}"
files=()
for t in "${templates[@]}"; do files+=(-f "$t"); done
checkov --quiet --compact --framework cloudformation "${files[@]}"
echo "[infra:lint] ${#templates[@]} template(s) clean"
