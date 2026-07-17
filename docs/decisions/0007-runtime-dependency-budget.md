# ADR 0007: Enforce a minimal runtime dependency budget

- Status: accepted for the M1 conveyor slice
- Date: 2026-07-17

## Context

MGMT is dependency-sensitive, and transitive modules increase review, vulnerability, version, and maintenance cost. The reference client includes useful CLI and discovery features that MGMT does not consume. Cryptography, however, must not be reimplemented casually.

## Decision

Use `google.golang.org/protobuf` v1.36.11 and `github.com/flynn/noise` v1.1.0 in the M1 core. Override Noise's old transitive crypto requirement with `golang.org/x/crypto` v0.48.0, the version already selected by the MGMT baseline. Keep mDNS, CLI, YAML, telemetry, assertion, and simulator frameworks out of the core dependency graph. Require an ADR and dependency-delta report for every additional runtime module.

The protobuf module is the official Go runtime and requires Go 1.23. The Noise module's latest tag is v1.1.0 at `4d9f71cd4ba1fe81415efac312664ccc4bc79b46`, uses a BSD-3-Clause license, and has one runtime cryptography dependency. The selected `x/crypto` v0.48.0 boundary passes race tests and `govulncheck` for the exact Noise call paths while avoiding an unrelated MGMT module-family upgrade.

Do not raise the language line merely to follow a reference client. The first source scan found reachable GO-2026-4971 in Go 1.25.7's `net` package, so the minimum is Go 1.25.10, the fixed patch. Verify that patch against the real MGMT build and rerun `govulncheck` before claiming the dependency gate complete.

## Consequences

MGMT pays only for protocol essentials. Some convenience features require separate optional packages or caller-provided interfaces. The Noise choice remains a blocking security review, because custom cryptography is explicitly prohibited.
