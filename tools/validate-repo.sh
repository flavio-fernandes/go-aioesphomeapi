#!/usr/bin/env bash
set -euo pipefail

cd "$(git rev-parse --show-toplevel 2>/dev/null || pwd)"

required=(
  README.md CHEATSHEET.md LICENSE SECURITY.md CONTRIBUTING.md AGENTS.md
  THIRD_PARTY_NOTICES.md docs/architecture.md docs/security-threat-model.md
  docs/documentation-style.md docs/provenance.md docs/support-matrix.md docs/roadmap.md
  docs/mgmt-integration.md docs/dependency-policy.md docs/reference-baseline.md
  docs/m1-implementation-plan.md
  compatibility/mgmt-feat-esphome.json
  protocol/upstream.lock.json protocol/inventory.json
  protocol/upstream/api.proto protocol/upstream/api_options.proto protocol/upstream/LICENSE
  pb/api.pb.go pb/api_options.pb.go tools/sync-protocol.sh tools/generate-protocol.sh
)

for path in "${required[@]}"; do
  if [[ ! -s "$path" ]]; then
    echo "required file is missing or empty: $path" >&2
    exit 1
  fi
done

if ! grep -Fq '[cheatsheet](CHEATSHEET.md)' README.md; then
  echo "README.md must link to the root cheatsheet" >&2
  exit 1
fi

if ! grep -Fq '**Current phase: usable development branch; no tagged release yet.**' CHEATSHEET.md; then
  echo "CHEATSHEET.md must state the current implementation phase" >&2
  exit 1
fi

if ! grep -Fq '8eab220' compatibility/mgmt-feat-esphome.json ||
  ! grep -Fq '982fb85860e7214e3384e68cb69bf94b16a6985b' compatibility/mgmt-feat-esphome.json; then
  echo "compatibility manifest must pin the reviewed MGMT and reference-client revisions" >&2
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
    echo "pinned protocol checksum mismatch: ${path}" >&2
    exit 1
  fi
done

if command -v python3 >/dev/null 2>&1; then
  python3 -m json.tool compatibility/mgmt-feat-esphome.json >/dev/null
  python3 -m json.tool protocol/upstream.lock.json >/dev/null
  python3 -m json.tool protocol/inventory.json >/dev/null
fi

if grep -RInE --exclude-dir=.git --exclude='validate-repo.sh' \
  '(TODO|FIXME|/home/[^ /]+|\.codex/attachments|chatgpt\.com/g/g-p-|BEGIN (RSA |EC |OPENSSH )?PRIVATE KEY|api[_-]?key[[:space:]]*[:=][[:space:]]*[^${][^ ]+)' .; then
  echo "placeholder, private path/URL, or possible secret found" >&2
  exit 1
fi

if grep -RInE --include='*.yml' --include='*.yaml' \
  'uses:[[:space:]]+[^[:space:]]+@(main|master|latest|v[0-9]+([.][0-9]+){0,2})($|[[:space:]#])' .github; then
  echo "GitHub Actions must be pinned to a full commit SHA" >&2
  exit 1
fi

while IFS= read -r skill; do
  name="$(sed -n 's/^name:[[:space:]]*//p' "$skill" | head -n1)"
  description="$(sed -n 's/^description:[[:space:]]*//p' "$skill" | head -n1)"
  if [[ -z "$name" || -z "$description" ]]; then
    echo "invalid skill frontmatter: $skill" >&2
    exit 1
  fi
  metadata="$(dirname "$skill")/agents/openai.yaml"
  if [[ ! -s "$metadata" ]] || ! grep -Fq "\$$name" "$metadata"; then
    echo "skill metadata is missing a matching default prompt: $metadata" >&2
    exit 1
  fi
done < <(find .agents/skills -name SKILL.md -type f -print | sort)

if git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  ignored_tracked="$(git ls-files -ci --exclude-standard)"
  if [[ -n "$ignored_tracked" ]]; then
    echo "ignored sensitive/output paths are tracked:" >&2
    echo "$ignored_tracked" >&2
    exit 1
  fi
fi

echo "repository policy validation passed"
