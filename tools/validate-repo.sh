#!/usr/bin/env bash
set -euo pipefail

cd "$(git rev-parse --show-toplevel 2>/dev/null || pwd)"

required=(
  README.md CHEATSHEET.md LICENSE SECURITY.md CONTRIBUTING.md AGENTS.md
  THIRD_PARTY_NOTICES.md docs/architecture.md docs/security-threat-model.md
  docs/documentation-style.md docs/provenance.md docs/support-matrix.md docs/roadmap.md
  docs/mgmt-integration.md docs/dependency-policy.md docs/reference-baseline.md
  docs/m1-implementation-plan.md
  compatibility/mgmt-feat-esphome.json compatibility/mgmt-feat-esphome2.json
  compatibility/mgmt-feat-esphome2-runtime.json compatibility/mgmt-feat-esphome2-baselines.json
  compatibility/mgmt-feat-esphome-mdns.json compatibility/mgmt-feat-esphome-diagnostics.json
  compatibility/mgmt-feat-esphome-hardware-blink.json docs/mgmt-hardware-blink.md
  compatibility/mgmt-feat-esphome-security.json
  compatibility/mgmt-feat-esphome-simulator-timeline-candidate.json
  compatibility/mgmt-feat-esphome-simulator-timeline-postmerge.json
  protocol/upstream.lock.json protocol/inventory.annotations.json protocol/inventory.json
  protocol/upstream/api.proto protocol/upstream/api_options.proto protocol/upstream/LICENSE
  pb/api.pb.go pb/api_options.pb.go tools/sync-protocol.sh tools/generate-protocol.sh
  tools/run-govulncheck.sh
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

if [[ ! -x tools/run-govulncheck.sh ]] ||
  ! grep -Fq 'govulncheck_version="v1.6.0"' tools/run-govulncheck.sh ||
  ! grep -Fq 'run: ./tools/run-govulncheck.sh' .github/workflows/policy.yml ||
  ! grep -Fq 'cron: "17 9 * * 1"' .github/workflows/policy.yml; then
  echo "vulnerability monitoring must be executable, version-pinned, and scheduled in policy CI" >&2
  exit 1
fi

if ! grep -Fq '8eab220' compatibility/mgmt-feat-esphome.json ||
  ! grep -Fq '982fb85860e7214e3384e68cb69bf94b16a6985b' compatibility/mgmt-feat-esphome.json; then
  echo "compatibility manifest must pin the reviewed MGMT and reference-client revisions" >&2
  exit 1
fi

if ! grep -Fq '5bf41f505bc601e6d2c4da8ecb3050b7c01ff34a' compatibility/mgmt-feat-esphome2.json ||
  ! grep -Fq '398a8e9296fc79513756964304f16fdf7c1a1da0' compatibility/mgmt-feat-esphome2.json ||
  ! grep -Fq '238f06dc564ec3b4a16473ef5225447c4303166c' compatibility/mgmt-feat-esphome2.json; then
  echo "replacement manifest must pin the rebased baseline, candidate, and library revisions" >&2
  exit 1
fi

if ! grep -Fq 'acddc3f1804dd3ae3e29f077996b7845e768ae29' compatibility/mgmt-feat-esphome2-runtime.json ||
  ! grep -Fq '352513396edf30b098e545e05ebc22659a9e3674' compatibility/mgmt-feat-esphome2-runtime.json; then
  echo "runtime manifest must pin the verified MGMT and library revisions" >&2
  exit 1
fi

if ! grep -Fq 'a29ebe1e8d052e450bdd92536629114f15baa401' compatibility/mgmt-feat-esphome2-baselines.json ||
  ! grep -Fq 'ef8386820978611d313f976e68bd2aaf9009e8b8' compatibility/mgmt-feat-esphome2-baselines.json; then
  echo "baseline runtime manifest must pin the verified MGMT and library revisions" >&2
  exit 1
fi

if ! grep -Fq 'c60c22ebf06e19dfbe6766136736d6f29e16bea7' compatibility/mgmt-feat-esphome-mdns.json ||
  ! grep -Fq '55602f044300bcf516c620aae88bb45d494f21b5' compatibility/mgmt-feat-esphome-mdns.json; then
  echo "mDNS compatibility manifest must pin the verified MGMT and library revisions" >&2
  exit 1
fi

if ! grep -Fq 'd625919912cb5b5791fe41d9c58326e79a0efcff' compatibility/mgmt-feat-esphome-diagnostics.json ||
  ! grep -Fq '73b5d58e5dd39d6dce0df024c3a792f668824b3b' compatibility/mgmt-feat-esphome-diagnostics.json; then
  echo "diagnostic compatibility manifest must pin the verified MGMT and library revisions" >&2
  exit 1
fi

if ! grep -Fq 'd625919912cb5b5791fe41d9c58326e79a0efcff' compatibility/mgmt-feat-esphome-hardware-blink.json ||
  ! grep -Fq '73b5d58e5dd39d6dce0df024c3a792f668824b3b' compatibility/mgmt-feat-esphome-hardware-blink.json ||
  ! grep -Fq 'cc57833875290b60e7e7f1004b93d00fb17249ad2f31267ac20ff91c1052c7ad' compatibility/mgmt-feat-esphome-hardware-blink.json; then
  echo "hardware blink manifest must pin MGMT, library, and immutable MCL revisions" >&2
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
  python3 -m json.tool compatibility/mgmt-feat-esphome2.json >/dev/null
  python3 -m json.tool compatibility/mgmt-feat-esphome2-runtime.json >/dev/null
  python3 -m json.tool compatibility/mgmt-feat-esphome2-baselines.json >/dev/null
  python3 -m json.tool compatibility/mgmt-feat-esphome-mdns.json >/dev/null
  python3 -m json.tool compatibility/mgmt-feat-esphome-diagnostics.json >/dev/null
  python3 -m json.tool compatibility/mgmt-feat-esphome-hardware-blink.json >/dev/null
  python3 -m json.tool compatibility/mgmt-feat-esphome-security.json >/dev/null
  python3 -m json.tool compatibility/mgmt-feat-esphome-simulator-timeline-candidate.json >/dev/null
  python3 -m json.tool compatibility/mgmt-feat-esphome-simulator-timeline-postmerge.json >/dev/null
  python3 -m json.tool protocol/upstream.lock.json >/dev/null
  python3 -m json.tool protocol/inventory.annotations.json >/dev/null
  python3 -m json.tool protocol/inventory.json >/dev/null
fi

if ! grep -Fq 'f1f9e3ef9b5efca161aa97cbe0040d278fdb4038' compatibility/mgmt-feat-esphome-security.json ||
  ! grep -Fq 'ede1737219be106e2c5e06bb497af9a1ec9e17c8' compatibility/mgmt-feat-esphome-security.json; then
  echo "security compatibility manifest must pin the verified library and MGMT revisions" >&2
  exit 1
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
