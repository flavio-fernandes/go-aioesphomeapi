# Dependency policy

Dependencies are an operational and security cost paid by MGMT and every other consumer. A feature is not complete merely because adding a module makes it easy.

## Runtime budget

The M1 core targets at most two direct runtime modules beyond the Go standard library:

1. the official Go protobuf runtime needed for ESPHome messages;
2. one established, security-reviewed Noise implementation.

This is a ceiling, not a quota. ADR 0007 accepts the initial Noise choice for M1 and records its pinned version, license, transitive override, and review gates. Writing cryptographic primitives locally is prohibited.

The initial Go directive remains compatible with MGMT's default branch where practical. A dependency may not force a Go-version increase without an explicit cross-repository decision and passing MGMT build evidence.

## Not core dependencies

The following do not belong in the core runtime graph:

- mDNS or service-discovery libraries (the narrow built-in `.local` resolver uses only `net`);
- CLI frameworks;
- YAML or configuration-file parsers;
- logging or telemetry SDKs;
- test assertion or mocking frameworks;
- simulator frameworks;
- MGMT packages;
- firmware, flashing, camera, or workbench tools.

Use `net`, injected interfaces, small command packages, standard `testing`, and optional integration modules instead. An optional feature must not become a transitive cost for the MGMT client.

## Admission gate

Every proposed runtime module needs an ADR containing:

- exact module path, version, license, checksum, and purpose;
- why the standard library or a small non-security-sensitive implementation is insufficient;
- maintainer and ownership history, release cadence, adoption, and bus-factor evidence;
- current vulnerability and supply-chain review;
- complete direct and transitive module delta;
- minimum Go version and platform impact;
- API surface used and an exit/replacement plan;
- deterministic tests at the boundary;
- approval from a maintainer and an MGMT reviewer.

Version age alone neither proves stability nor proves abandonment. Claims must be evidence-based.

## Change budget

A pull request that changes `go.mod` or generated tooling must include:

```text
Runtime direct modules: before N, after N
Runtime transitive modules: before N, after N
Tool-only modules: before N, after N
MGMT go.mod delta: +X / -Y modules
Go directive: unchanged / changed with approved reason
```

CI will eventually generate and compare this report. Unexpected additions fail closed.

## Continuous vulnerability monitoring

`./tools/run-govulncheck.sh` runs the official `golang.org/x/vuln` scanner at
the exact tool version recorded in that script. The tool is downloaded through
the checksum-verified Go module path and does not enter this module's runtime or
tool dependency graph. Repository policy runs it for every pull request, every
push to `main`, and on a weekly schedule.

A reachable finding exits nonzero and fails closed. Findings in required
modules that the source analysis cannot reach remain visible in the verbose
scan output and, when GitHub has a matching advisory, Dependabot. CI may pass,
but a maintainer must triage them. An available compatible security patch is
upgraded and tested. If no safe patch exists, an
issue records exposure and mitigations, and the finding blocks a public release
until a maintainer accepts a concrete non-exposure or mitigation decision. An
“unreachable” result is never a reason to dismiss a Dependabot alert without
that review. Dependency upgrades remain reviewed pull requests, not automatic
merges.

## Reference-client observation

The pinned reference release brings protobuf and Noise plus mDNS, CLI, YAML, and their transitive modules, and it raises the Go directive used by the current MGMT branch. Some are useful for its bundled CLI but are not required by MGMT's native API driver. Our compatibility goal is behavioral parity for MGMT without inheriting those unrelated costs.
