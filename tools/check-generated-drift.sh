#!/usr/bin/env bash
set -euo pipefail

# Prove that the checked-in generated protocol files are exactly what the
# pinned inputs and the pinned generator toolchain produce. Regeneration runs
# in a throwaway copy of the tracked files, so this check never mutates the
# checkout it verifies, whether it passes or fails.

cd "$(git rev-parse --show-toplevel 2>/dev/null || true)"
if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo "run this from inside the repository checkout" >&2
  exit 1
fi

# The files that tools/generate-protocol.sh rewrites.
generated=(pb/api.pb.go pb/api_options.pb.go protocol/inventory.json)

tmpdir="$(mktemp -d)"
trap 'rm -rf "${tmpdir}"' EXIT

# Copy the tracked tree as it currently stands (including staged or unstaged
# edits), then regenerate inside the copy with the existing pinned command.
git ls-files -z | tar --null --files-from=- -cf - | tar -xf - -C "${tmpdir}"
(cd "${tmpdir}" && ./tools/generate-protocol.sh >/dev/null)

drift=0
for path in "${generated[@]}"; do
  if ! cmp -s "${path}" "${tmpdir}/${path}"; then
    echo "generated drift: ${path} no longer matches its pinned inputs" >&2
    diff -u "${path}" "${tmpdir}/${path}" >&2 || true
    drift=1
  fi
done

if [[ "${drift}" -ne 0 ]]; then
  echo "run ./tools/generate-protocol.sh and commit the result" >&2
  exit 1
fi

echo "generated protocol files match their pinned inputs"
