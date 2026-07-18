# ADR 0007: Enforce a minimal runtime dependency budget

- Status: accepted for the M1 conveyor slice
- Date: 2026-07-17
- Updated: 2026-07-18

## Context

MGMT is dependency-sensitive, and transitive modules increase review, vulnerability, version, and maintenance cost. The reference client includes useful CLI and discovery features that MGMT does not consume. Cryptography, however, must not be reimplemented casually.

## Decision

Use `google.golang.org/protobuf` v1.36.11 and `github.com/flynn/noise` v1.1.0 in the M1 core. Override Noise's old transitive crypto requirement with `golang.org/x/crypto` v0.52.0. Keep mDNS, CLI, YAML, telemetry, assertion, and simulator frameworks out of the core dependency graph. Require an ADR and dependency-delta report for every additional runtime module.

The protobuf module is the official Go runtime and requires Go 1.23. The Noise module's latest tag is v1.1.0 at `4d9f71cd4ba1fe81415efac312664ccc4bc79b46`, uses a BSD-3-Clause license, and has one runtime cryptography dependency. `x/crypto` v0.52.0 is the upstream security release at commit `a1c0d9929856c8aba2b31f079340f00578eda803`, uses a BSD-3-Clause license, requires Go 1.25, and has checksum `h1:RMs7fP2rXdep0CftQlK8Uf+kibLm7qkCcradZWYz988=`. It selects BSD-3-Clause `x/sys` v0.45.0 with checksum `h1:dO4czNzziLiiXplLQgBCEpCvXQ3dnkn0SdaZSYdQ+FY=` for the Noise build.

The prior v0.48.0 selection accumulated 13 GitHub advisories: seven critical,
two high, and four medium. Every alert identifies v0.52.0 as the first patched
version. Even though the library uses the Noise-relevant subset rather than the
affected SSH APIs, the compatible patch is available, so the alerts are fixed
instead of dismissed as unreachable.

Do not raise the language line merely to follow a reference client. The first
source scan found reachable GO-2026-4971 in Go 1.25.7's `net` package. The
continuous scan subsequently found GO-2026-4970 in imported package `os` plus
four unreachable standard-library findings in Go 1.25.10. Go 1.25.12 is the
first patch that fixes that complete set, so the module and CI floor advance to
1.25.12 without changing the language version.

GO-2026-5932 marks `x/crypto/openpgp` unmaintained and has no fixed module
version. This project does not import that package, it is not reachable from
the module, and the Noise boundary uses only the separately maintained crypto
packages selected by `flynn/noise`. The verbose scheduled scan keeps the notice
visible. A future reachability change fails review; replacing the Noise
dependency remains a separate cryptographic design decision.

Dependency delta for the security update:

```text
Runtime direct modules: before 2, after 2
Runtime transitive modules: before 2, after 2
Tool-only modules: before 0, after 0
MGMT go.mod delta: +0 / -0 modules; selected x/crypto and x/sys versions advance
Go directive: 1.25.10 to security-fixed 1.25.12
```

## Consequences

MGMT pays only for protocol essentials. Some convenience features require separate optional packages or caller-provided interfaces. The Noise choice remains a blocking security review, because custom cryptography is explicitly prohibited. The pinned vulnerability scanner is a CI command, not a library dependency.
