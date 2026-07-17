#!/usr/bin/env bash
set -euo pipefail

cd "$(git rev-parse --show-toplevel 2>/dev/null || pwd)"

protoc_bin="${PROTOC:-$(command -v protoc || true)}"
plugin_bin="${PROTOC_GEN_GO:-$(command -v protoc-gen-go || true)}"

if [[ -z "${protoc_bin}" ]] || [[ "$(${protoc_bin} --version)" != "libprotoc 31.1" ]]; then
  echo "protoc v31.1 is required; set PROTOC to its executable" >&2
  exit 1
fi
if [[ -z "${plugin_bin}" ]] || [[ "$(${plugin_bin} --version)" != "protoc-gen-go v1.36.11" ]]; then
  echo "protoc-gen-go v1.36.11 is required; set PROTOC_GEN_GO to its executable" >&2
  exit 1
fi

for record in \
  "9ddd6b66a016cd5ccb216052668d680cb83413e2d4eb3b1cff84a50b30492828 protocol/upstream/api.proto" \
  "c4ba32a9d34e8785442112aed5b202a1614a9d74d59a90c992cdb13902bd79f5 protocol/upstream/api_options.proto" \
  "1f312390122725ee382f85af00e3df84f2490cd8fa61b9ea172bc6591cc0ac63 protocol/upstream/LICENSE"; do
  expected="${record%% *}"
  path="${record#* }"
  actual="$(sha256sum "${path}" | cut -d' ' -f1)"
  if [[ "${actual}" != "${expected}" ]]; then
    echo "checksum mismatch for ${path}" >&2
    exit 1
  fi
done

mkdir -p pb
PATH="$(dirname "${plugin_bin}"):${PATH}" "${protoc_bin}" \
  -I protocol/upstream \
  -I "$(dirname "${protoc_bin}")/../include" \
  --go_out=. \
  --go_opt=module=github.com/flavio-fernandes/go-aioesphomeapi \
  --go_opt=Mapi.proto=github.com/flavio-fernandes/go-aioesphomeapi/pb \
  --go_opt=Mapi_options.proto=github.com/flavio-fernandes/go-aioesphomeapi/pb \
  protocol/upstream/api_options.proto \
  protocol/upstream/api.proto

go run ./cmd/protocol-inventory > protocol/inventory.json
