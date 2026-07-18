#!/usr/bin/env bash
set -euo pipefail

cd "$(git rev-parse --show-toplevel 2>/dev/null || pwd)"

readonly govulncheck_version="v1.6.0"

exec go run "golang.org/x/vuln/cmd/govulncheck@${govulncheck_version}" \
  -show version,verbose ./...
