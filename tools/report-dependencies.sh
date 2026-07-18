#!/usr/bin/env bash
set -euo pipefail

# Report and check the module graph: Go directive, direct and transitive
# runtime modules, tool-only helpers, versions, checksums, detected licenses,
# and the accepted dependency budget from docs/dependency-policy.md. Any
# unexpected runtime module, version drift, or unrecognized license fails
# closed. Standard Go tooling and shell only; no external scanner.

cd "$(git rev-parse --show-toplevel 2>/dev/null || pwd)"

# The accepted budget. Updating these lines is a deliberate policy change and
# needs the docs/dependency-policy.md admission evidence in the same review.
expected_go_directive="1.25.12"
expected_direct_budget=2
expected_modules=(
  "github.com/flynn/noise@v1.1.0 direct BSD-3-Clause"
  "google.golang.org/protobuf@v1.36.11 direct BSD-3-Clause"
  "golang.org/x/crypto@v0.52.0 indirect BSD-3-Clause"
  "golang.org/x/sys@v0.45.0 indirect BSD-3-Clause"
)

fail=0
problem() {
  echo "PROBLEM: $*" >&2
  fail=1
}

detect_license() {
  # Classify a module's license file by its distinctive wording. Unknown or
  # missing license text is reported as UNKNOWN and fails the check for any
  # module that builds are linked against.
  local dir="$1" file text
  for file in LICENSE LICENSE.txt LICENSE.md COPYING; do
    if [[ -s "${dir}/${file}" ]]; then
      text="$(<"${dir}/${file}")"
      if [[ "${text}" == *"Redistribution and use in source and binary forms"* ]]; then
        if [[ "${text}" == *"endorse or promote"* ]]; then
          echo "BSD-3-Clause"
        else
          echo "BSD-2-Clause"
        fi
        return
      fi
      if [[ "${text}" == *"Permission is hereby granted, free of charge"* ]]; then
        echo "MIT"
        return
      fi
      if [[ "${text}" == *"Apache License"* && "${text}" == *"Version 2.0"* ]]; then
        echo "Apache-2.0"
        return
      fi
      echo "UNKNOWN"
      return
    fi
  done
  echo "UNKNOWN"
}

echo "== Go directive and toolchain"
go_directive="$(go mod edit -json | sed -n 's/^\t"Go": "\(.*\)",\{0,1\}$/\1/p' | head -n1)"
toolchain_directive="$(go mod edit -json | sed -n 's/^\t"Toolchain": "\(.*\)",\{0,1\}$/\1/p' | head -n1)"
echo "go directive:        ${go_directive}"
echo "toolchain directive: ${toolchain_directive:-none}"
echo "local go:            $(go version)"
if [[ "${go_directive}" != "${expected_go_directive}" ]]; then
  problem "go directive is ${go_directive}, accepted budget pins ${expected_go_directive}"
fi
if [[ -n "${toolchain_directive}" ]]; then
  problem "unexpected toolchain directive: ${toolchain_directive}"
fi

echo
echo "== Runtime modules (linked into builds of ./...)"
mapfile -t runtime_modules < <(
  go list -deps -f '{{if and (not .Standard) .Module}}{{if not .Module.Main}}{{.Module.Path}}@{{.Module.Version}}{{end}}{{end}}' ./... |
    sed '/^$/d' | sort -u
)
direct_count=0
for module in "${runtime_modules[@]}"; do
  role="$(go list -m -f '{{if .Indirect}}indirect{{else}}direct{{end}}' "${module%%@*}")"
  dir="$(go mod download -json "${module}" | sed -n 's/^\t"Dir": "\(.*\)",\{0,1\}$/\1/p')"
  license="UNKNOWN"
  if [[ -n "${dir}" ]]; then
    license="$(detect_license "${dir}")"
  fi
  sums="$(grep -cF "${module%%@*} ${module##*@}" go.sum || true)"
  printf '%-40s %-8s %-12s go.sum entries: %s\n' "${module}" "${role}" "${license}" "${sums}"
  if [[ "${role}" == "direct" ]]; then
    direct_count=$((direct_count + 1))
  fi
  [[ "${sums}" -ge 1 ]] || problem "${module} has no go.sum checksum entry"
  expected_line=""
  base_found=""
  for candidate in "${expected_modules[@]}"; do
    if [[ "${candidate%% *}" == "${module}" ]]; then
      expected_line="${candidate}"
    fi
    if [[ "${candidate%% *}" == "${module%%@*}"@* ]]; then
      base_found="${candidate}"
    fi
  done
  if [[ -z "${expected_line}" ]]; then
    if [[ -n "${base_found}" ]]; then
      problem "version drift: have ${module}, accepted budget pins ${base_found%% *}"
    else
      problem "unexpected runtime module addition: ${module}"
    fi
    continue
  fi
  expected_role="$(cut -d' ' -f2 <<<"${expected_line}")"
  expected_license="$(cut -d' ' -f3 <<<"${expected_line}")"
  [[ "${role}" == "${expected_role}" ]] ||
    problem "${module} is ${role}, accepted budget records ${expected_role}"
  if [[ "${license}" == "UNKNOWN" ]]; then
    problem "${module} has an unrecognized license; classify it before accepting"
  elif [[ "${license}" != "${expected_license}" ]]; then
    problem "${module} license detected as ${license}, accepted budget records ${expected_license}"
  fi
done
for candidate in "${expected_modules[@]}"; do
  present=""
  for module in "${runtime_modules[@]}"; do
    if [[ "${module}" == "${candidate%% *}" ]]; then
      present=yes
    fi
  done
  [[ -n "${present}" ]] || problem "accepted module is missing from the runtime graph: ${candidate%% *}"
done
if [[ "${direct_count}" -gt "${expected_direct_budget}" ]]; then
  problem "direct runtime modules: ${direct_count}, budget ceiling is ${expected_direct_budget}"
fi

echo
echo "== Module graph extras (test scope of dependencies; never linked)"
extras=0
while IFS= read -r entry; do
  [[ -z "${entry}" ]] && continue
  linked=""
  for module in "${runtime_modules[@]}"; do
    if [[ "${module}" == "${entry}" ]]; then
      linked=yes
    fi
  done
  if [[ -z "${linked}" ]]; then
    echo "${entry}"
    extras=$((extras + 1))
  fi
done < <(go list -m -f '{{if not .Main}}{{.Path}}@{{.Version}}{{end}}' all)
echo "(${extras} entries; exact versions are held by go.mod, go.sum, and the CI tidy check)"

echo
echo "== Module cache checksums"
if go mod verify >/dev/null; then
  echo "go mod verify: module cache matches go.sum"
else
  problem "go mod verify failed"
fi

echo
echo "== Tool-only modules (pinned in scripts, outside the module graph)"
govulncheck_version="$(sed -n 's/^readonly govulncheck_version="\(.*\)"$/\1/p' tools/run-govulncheck.sh)"
echo "golang.org/x/vuln/cmd/govulncheck ${govulncheck_version:-UNPINNED} (tools/run-govulncheck.sh)"
echo "protoc v31.1 and protoc-gen-go v1.36.11 (tools/generate-protocol.sh)"
[[ -n "${govulncheck_version}" ]] || problem "govulncheck version pin not found"

echo
echo "== Change budget (docs/dependency-policy.md format)"
indirect_count=$(( ${#runtime_modules[@]} - direct_count ))
echo "Runtime direct modules: before 2, after ${direct_count}"
echo "Runtime transitive modules: before 2, after ${indirect_count}"
echo "Tool-only modules: before 0, after 0"
echo "MGMT go.mod delta: +0 / -0 modules (recompute in the MGMT PR when pinning)"
echo "Go directive: $([[ "${go_directive}" == "${expected_go_directive}" ]] && echo unchanged || echo "changed to ${go_directive}")"

echo
if [[ "${fail}" -ne 0 ]]; then
  echo "dependency report found problems; see PROBLEM lines above" >&2
  exit 1
fi
echo "dependency report matches the accepted budget"
