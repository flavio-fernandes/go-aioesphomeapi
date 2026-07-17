---
name: run-device-simulator
description: Design, run, and diagnose deterministic simulated ESPHome device scenarios through the real Native API client path. Use for entity behavior, reconnects, malformed peers, timeouts, command assertions, MGMT integration tests, load tests, or conveyor demonstrations without hardware.
---

# Run Device Simulator

Use the simulator as a real device-side protocol peer, not as a mock of the public client.

## Workflow

1. Read ADR 0004 and `references/scenario-contract.md`.
2. For documented demos, acceptance scripts, or external-app examples, also read `references/field-notes.md`.
3. Choose the smallest existing scenario that proves the requested behavior. Prefer generic entity fixtures; the conveyor is a composed example.
4. Use synthetic identifiers, test-only keys, virtual time, and an explicit random seed. Bind to loopback unless a task requires an isolated test network.
5. Run the documented simulator command and the real client or MGMT adapter against it.
6. Assert handshake, discovered capabilities, ordered state events, received commands, cancellation, cleanup, and final goroutine/resource state.
7. For faults, select one named fault at a time before composing them: fragmented frame, malformed length, delayed reply, dropped connection, slow subscriber, rejected authentication, unknown message, or reconnect storm.
8. Save only sanitized scenario definitions and deterministic textual results. Never save real traffic, credentials, host details, or camera data.
9. Update support evidence only when the scenario is checked into the test suite and passes in CI.

## Pre-implementation behavior

Until the simulator package and command exist, use this skill to produce or refine the M1 scenario contract and acceptance test. Do not create an ad hoc socket server whose behavior bypasses the planned shared framing layer.
