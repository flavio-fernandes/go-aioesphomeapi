# ADR 0001: Greenfield repository, not a fork

- Status: accepted for bootstrap
- Date: 2026-07-16

## Context

Existing Go clients provide useful history but carry protocol, API, dependency, or product assumptions that do not match a secure MGMT-first library. The project also aspires to broad current ESPHome compatibility without presenting itself as official.

## Decision

Create `flavio-fernandes/go-aioesphomeapi` as a new repository. Implement from official protocol definitions and public documentation. Use existing clients for comparative research and interoperability questions, not as a code base.

## Consequences

Provenance is simpler and legacy compatibility is opt-in. More foundational work is required, and compatibility must be earned through conformance tests and the support matrix.
