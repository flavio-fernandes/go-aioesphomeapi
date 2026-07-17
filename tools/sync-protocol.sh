#!/usr/bin/env bash
set -euo pipefail

cd "$(git rev-parse --show-toplevel 2>/dev/null || pwd)"

commit="920a8b761b680d9864da2ef4b44b4af95c99dba8"
base="https://raw.githubusercontent.com/esphome/esphome/${commit}"
tmp="$(mktemp -d)"
trap 'rm -rf "${tmp}"' EXIT

curl -fsSL "${base}/esphome/components/api/api.proto" -o "${tmp}/api.proto"
curl -fsSL "${base}/esphome/components/api/api_options.proto" -o "${tmp}/api_options.proto"
curl -fsSL "${base}/LICENSE" -o "${tmp}/LICENSE"

verify() {
  local expected="$1"
  local path="$2"
  local actual
  actual="$(sha256sum "${path}" | cut -d' ' -f1)"
  if [[ "${actual}" != "${expected}" ]]; then
    echo "checksum mismatch for $(basename "${path}")" >&2
    exit 1
  fi
}

verify "9ddd6b66a016cd5ccb216052668d680cb83413e2d4eb3b1cff84a50b30492828" "${tmp}/api.proto"
verify "c4ba32a9d34e8785442112aed5b202a1614a9d74d59a90c992cdb13902bd79f5" "${tmp}/api_options.proto"
verify "1f312390122725ee382f85af00e3df84f2490cd8fa61b9ea172bc6591cc0ac63" "${tmp}/LICENSE"

mkdir -p protocol/upstream
install -m 0644 "${tmp}/api.proto" protocol/upstream/api.proto
install -m 0644 "${tmp}/api_options.proto" protocol/upstream/api_options.proto
install -m 0644 "${tmp}/LICENSE" protocol/upstream/LICENSE

./tools/generate-protocol.sh
