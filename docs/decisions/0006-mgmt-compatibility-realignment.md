# ADR 0006: Make the pinned MGMT branch the first compatibility contract

- Status: accepted through PR #27 and the merged MGMT replacement
- Date: 2026-07-17

## Context

MGMT's `feat/esphome` branch now contains a substantial native ESPHome integration using `Richard87/esphome-apiclient`. Abandoning this repository would lose its stronger security, simulation, governance, and long-term protocol design. Ignoring the working MGMT branch would create avoidable integration risk.

## Decision

Keep this repository greenfield and make the MGMT branch at `8eab220` the first external compatibility contract. Use `Richard87/esphome-apiclient` `v1.1.0` at `982fb85860e7214e3384e68cb69bf94b16a6985b` as a behavioral and Go-surface comparison. Use pinned ESPHome protocol definitions as wire truth.

M1 provides the smallest source-compatible facade needed by MGMT plus a preferred typed API over the same internals. Generated `pb` types may be public for this compatibility purpose; this supersedes only ADR 0003's proposed internal-only placement. Existing MCL behavior cannot change silently.

MGMT remains responsible for shared endpoint sessions, persistent/polling policy, convergence, outage interlocks, and MCL resources/functions. This library remains responsible for a bounded, context-aware connection and protocol client.

## Consequences

The migration can be small enough for meaningful review, and MGMT becomes a real cross-repository acceptance test. We accept a deliberately narrow generated compatibility surface. We do not promise full parity with every reference-client feature in M1, and we do not copy implementation code from either repository.
