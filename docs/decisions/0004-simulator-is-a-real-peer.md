# ADR 0004: Simulator implements the device side of the wire protocol

- Status: accepted
- Date: 2026-07-18

## Context

A mock client would test application call patterns but not framing, handshake, subscriptions, reconnect behavior, or protocol evolution.

## Decision

Build an in-process simulated ESPHome peer that shares generated wire
definitions, framing, and Noise with the client. Its deterministic scenario API
covers entities, current and scheduled states, logs, expected commands, virtual
time, an explicit seed, bounded network shaping, and named faults.

The normative semantics, cleanup rules, planned acceptance scenarios, and
fidelity limits are defined in
[`references/scenario-contract.md`](../../references/scenario-contract.md).
Architecture issue #2 accepts that document as the fixed input for simulator
implementation issue #10 and lifecycle issue #11.

Network shaping occurs below framing on the simulator side of the real
connection. Deterministic tests use a caller-advanced clock; no fault depends
on hidden sleeps or package-global randomness. MGMT continues to own polling,
reconnect, convergence, and outage policy.

## Consequences

Application and MGMT tests can use real client paths without hardware. The
simulator is not claimed to be ESPHome firmware and must state its fidelity
limits. Accepted design is not support evidence until checked-in tests exercise
it. Real-firmware and physical behavior remain separate evidence levels.
