# ADR 0007: Enforce a minimal runtime dependency budget

- Status: proposed for Gate 0 acceptance
- Date: 2026-07-17

## Context

MGMT is dependency-sensitive, and transitive modules increase review, vulnerability, version, and maintenance cost. The reference client includes useful CLI and discovery features that MGMT does not consume. Cryptography, however, must not be reimplemented casually.

## Decision

Target the standard library plus at most the official Go protobuf runtime and one independently accepted Noise implementation in the M1 core. Keep mDNS, CLI, YAML, telemetry, assertion, and simulator frameworks out of the core dependency graph. Require an ADR and dependency-delta report for every new runtime module.

Do not raise the Go directive merely to follow a reference client. Select the lowest version compatible with current MGMT and required security support, then verify the real MGMT build.

## Consequences

MGMT pays only for protocol essentials. Some convenience features require separate optional packages or caller-provided interfaces. The Noise choice remains a blocking security review, because custom cryptography is explicitly prohibited.
