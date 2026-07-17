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

if ! grep -Fq '**Current phase: compatibility architecture only.**' CHEATSHEET.md; then
  echo "CHEATSHEET.md must state the current implementation phase" >&2
  exit 1
fi

if ! grep -Fq '8eab220' compatibility/mgmt-feat-esphome.json ||
  ! grep -Fq '982fb85860e7214e3384e68cb69bf94b16a6985b' compatibility/mgmt-feat-esphome.json; then
  echo "compatibility manifest must pin the reviewed MGMT and reference-client revisions" >&2
  exit 1
fi

if command -v python3 >/dev/null 2>&1; then
  python3 -m json.tool compatibility/mgmt-feat-esphome.json >/dev/null
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
