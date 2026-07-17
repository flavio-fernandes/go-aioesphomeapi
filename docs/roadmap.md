# Controlled roadmap

Milestones are gated. Dates are intentionally absent until maintainers assign capacity and review owners. The pinned MGMT MCL contract stays green at every milestone.

## Gate 0 — freeze compatibility and cost before code

**Goal:** remove ambiguity about what replaces the current client and what MGMT must not lose.

- accept ADR 0006 and the immutable MGMT/reference baselines;
- approve the root compatibility facade, generated `pb` exception, and typed API boundary;
- turn the compatibility manifest into a clean cross-repository checkout/compile design;
- classify every current MGMT MCL behavior as preserve, intentionally strengthen, or defect requiring MGMT review;
- pin upstream `api.proto`, generation inputs, and protocol inventory schema;
- accept simulator API, virtual time, fault vocabulary, and fidelity limits;
- accept finite frame, queue, request, deadline, log, dial, and reconnect budgets;
- approve the protobuf runtime and one Noise implementation under the dependency gate;
- prove the proposed module graph and Go directive against MGMT;
- keep newcomer, privacy, provenance, and GitHub governance controls current.

Exit: accepted contracts and ADRs, machine-readable baseline, zero unresolved license or dependency questions, and no undocumented MCL behavior change.

## Milestone 1 — replace the client without breaking MGMT

**Goal:** MGMT's existing branch runs both MCL examples through this module, then the conveyor adds the first new showcase behavior.

Library slice:

- bounded plaintext framing and Noise transport; secure configuration is the normal path;
- hello, API version, device info, ping, disconnect, and context-bound connection lifetime;
- entity discovery for binary sensor, sensor, text sensor, switch, number, button, and fan;
- state subscription for binary sensor, sensor, text sensor, switch, number, and fan;
- switch, number, button, and fan commands;
- bounded redacted log subscription;
- the exact MGMT-required root and `pb` compatibility symbols;
- deterministic simulated device covering valid, malformed, delayed, dropped, and disconnect paths;
- race, fuzz-smoke, goroutine, allocation, and dependency-delta checks.

Cross-repository acceptance:

- check out MGMT at the pinned baseline and replace only module/import references;
- compile its driver and run its existing session tests;
- run `esphome0.mcl` and `esphome-blink.mcl` unchanged against simulator scenarios;
- prove shared one-connection behavior, persistent and polling modes, name/object-ID lookup, logs, reconnect/outage behavior, command completion, and cleanup;
- run a new conveyor MCL example using the same public contract;
- optionally run explicitly authorized hardware only after simulator acceptance.

Exit: zero MCL source diff, mechanically bounded Go adapter diff, clean MGMT tests, secure simulator demo from a clean clone, no race findings, approved dependency report, and evidence-linked support matrix.

Implementation follows the ordered [M1 implementation sequence](m1-implementation-plan.md). It is a dependency graph and stop-condition guide, not permission to bypass Gate 0.

## Milestone 2 — typed API and useful parity beyond MGMT

**Goal:** be attractive as a standalone Go library without destabilizing MGMT.

- complete the preferred immutable typed state/command API;
- add light and select plus other low-risk reference-client core features selected by demand;
- publish explicit reference-client API parity and migration tables;
- add generic non-conveyor simulator examples;
- automate oldest/current/development ESPHome compatibility reports;
- decide whether optional discovery or CLI modules justify their own dependency budgets.

Exit: a non-MGMT program uses only the typed API; MGMT compatibility remains unchanged; optional packages do not alter the core module graph.

## Milestone 3 — complex entities and services

**Goal:** expand one reviewed surface at a time.

- text, climate, cover, lock, services/actions, and other selected entities;
- version/capability gates and unknown-value conformance;
- authorization guidance for security-sensitive commands;
- bounded streaming surfaces with explicit backpressure.

Exit: current and oldest-supported ESPHome evidence, simulator negative paths, and no unreviewed dependency growth.

## Milestone 4 — factory-scale operations

**Goal:** prove bounded behavior for MGMT-shaped fleets.

- fleet dial scheduling outside individual sessions;
- hundreds, then thousands, of deterministic simulated devices;
- dependency-free observability hooks;
- CPU, memory, goroutine, reconnect, event-latency, and recovery budgets;
- partition, rolling firmware update, key rotation, and thundering-herd scenarios.

Exit: reproducible benchmark report and no unbounded resource path.

## Milestone 5 — evidence-driven ecosystem breadth

Candidate epics include Bluetooth proxy, camera, media, voice assistant, serial proxy, Z-Wave proxy, update, discovery, and CLI tooling. Each needs a consumer, threat-model amendment, resource and dependency budget, simulator plan, and support-matrix evidence. Reference-client or protocol presence alone does not schedule implementation.

## Milestone 6 — first evidence-backed release

- audit all public history for secrets, privacy, license, and provenance;
- enforce default-branch protections and supported security features;
- publish a semantic release with immutable workflow inputs and provenance;
- publish MGMT and reference-client migration notes, compatibility, and limitations;
- document upstream sync, deprecation, and security-response cadence.

Exit: reproducible GPL-3.0-only release, tested reporting path, clean audit, and no claim beyond recorded evidence.
