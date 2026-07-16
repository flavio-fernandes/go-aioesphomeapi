# Controlled roadmap

Milestones are gated. Starting a later milestone does not waive an incomplete earlier exit criterion. Dates are intentionally omitted until maintainers assign capacity and review owners.

## Gate 0 — public contract before implementation

**Goal:** remove architectural and legal ambiguity.

Tasks:

- approve scope, layer boundaries, naming disclaimer, and MGMT licensing boundary;
- choose protobuf generation toolchain and pin every version;
- design `protocol/upstream.lock.json` and first clean protocol sync;
- inventory all current upstream messages and entity families;
- accept stable public API conventions for contexts, errors, subscriptions, and capabilities;
- accept simulator API, deterministic clock/randomness, and fault vocabulary;
- establish privacy, security, dependency, release, and branch-protection policy;
- create a minimal MGMT adapter design in the MGMT repository, not here.

Exit criteria: accepted ADRs, reviewed threat model, reproducible protocol sync design, zero unresolved license questions, and GitHub controls active.

## Milestone 1 — secure vertical slice and conveyor acceptance demo

**Goal:** a fun end-to-end demonstration built on production-shaped foundations.

Library tasks:

- secure transport, framing, handshake, device info, keepalive, and controlled disconnect;
- one concurrency-safe device session with bounded subscriptions;
- discovery/state/command support for binary sensor, sensor, switch, and fan;
- deterministic simulator with scenario timeline, assertions, and network faults;
- typed errors, redacted logs, metrics hooks, race tests, fuzz smoke tests, and reconnect tests;
- Go example that works unchanged against simulator or explicitly selected hardware.

MGMT/demo tasks:

- MGMT resource/provider maps generic ESPHome entities into desired state;
- ESPHome firmware locally owns safe boot, communications timeout, maximum run time, invalid sensor-state stop, and motor output;
- conveyor profile uses an H-bridge fan entity for DRV8833 direction/speed and two binary-sensor paths for position sensing;
- interactive demo shows normal routing, sensor-driven transitions, disconnect recovery, and a safe-stop fault;
- a human-readable dashboard explains which behavior is MGMT, library, ESPHome, and physical hardware.

Exit criteria: simulator demo is reproducible in CI; hardware demo passes a signed checklist without repository secrets; no race findings; secure-by-default connection verified; support matrix has evidence links.

## Milestone 2 — reusable entity foundation

**Goal:** prove the design generalizes beyond the conveyor.

- add number, button, light, select, and text-sensor families;
- capability-aware command validation and immutable state cache;
- simulator scenario library for generic devices, not demo-specific fixtures;
- reconnect and subscription semantics documented as public contract;
- compatibility CI for supported Go and ESPHome versions;
- MGMT integration examples for read-only telemetry and generic desired state.

Exit criteria: a non-conveyor example uses every M2 entity family; simulator load tests meet accepted budgets.

## Milestone 3 — complex entities and services

**Goal:** expand without destabilizing the session core.

- add text, climate, cover, lock, and selected service/action support;
- implement device-log streaming with redaction and backpressure;
- create conformance suites for optional fields, unknown enums, and version gates;
- document application authorization responsibilities for sensitive commands.

Exit criteria: compatibility report covers current stable ESPHome and the oldest supported line; sensitive entities have negative authorization examples.

## Milestone 4 — factory-scale operations

**Goal:** validate many-device behavior for MGMT deployments.

- fleet connection scheduler with jitter and concurrency caps outside single-device sessions;
- load and soak harness for hundreds, then thousands, of simulated devices;
- OpenTelemetry-compatible hooks without a mandatory telemetry dependency;
- CPU, memory, goroutine, reconnect, and event-latency budgets;
- network partition, rolling firmware update, key rotation, and thundering-herd scenarios.

Exit criteria: published reproducible benchmark report and no unbounded resource path.

## Milestone 5 — ecosystem breadth

**Goal:** prioritize additional upstream surfaces from real demand.

Candidate epics include Bluetooth proxy, camera, media, voice assistant, serial proxy, Z-Wave proxy, and update entities. Each requires a threat-model amendment, resource budget, simulator design, and support-matrix evidence. Protocol presence alone does not schedule implementation.

## Milestone 6 — public release

**Goal:** publish an honest, maintainable GPL-3.0-only project.

- run the public-release skill and close every finding;
- enable private vulnerability reporting, rulesets, secret scanning where available, dependency review, and signed release provenance;
- publish compatibility and limitations with the first semantic version;
- document deprecation and upstream-sync cadence;
- change visibility only after a maintainer reviews the complete repository history.

Exit criteria: reproducible release, clean history/privacy audit, SPDX/license audit, security response path tested, and no support claim beyond the evidence matrix.
