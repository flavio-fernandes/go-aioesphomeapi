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
4. Use synthetic identifiers, test-only keys, and a caller-advanced manual
   clock. Call `Scenario.Validate` for custom scenarios. A non-zero seed is
   required only when the scenario declares randomized actions; zero is valid
   for deterministic raw literals. Scenario time is device-global and starts
   at zero; equal-time events retain declaration order. Bind to loopback unless
   a task requires an isolated test network.
5. Run the documented simulator command and the real client or MGMT adapter
   against it. A reconnect receives the latest state snapshot and only future
   timeline events; past events are not replayed as a burst. When introducing
   that global-store behavior, follow ADR 0013's before/after MGMT re-baseline
   and append-only evidence gate.
6. Assert handshake, discovered capabilities, ordered state events, ordered
   command expectations, cancellation, queue outcomes, cleanup, and final
   simulator-owned goroutine/resource state. Do not rely on exact process-wide
   goroutine counts.
7. For faults, select one named fault at a time before composing them:
   fragmented frame, coalesced segments, malformed length, delayed reply,
   dropped connection, slow subscriber, rejected authentication, unknown
   message, or reconnect storm. Delay and stall use virtual time or context
   cancellation. Fragmentation and coalescing act below framing on the
   simulator side and preserve bytes exactly.
8. Save only sanitized scenario definitions and deterministic textual results. Never save real traffic, credentials, host details, or camera data.
9. Update support evidence only when the scenario is checked into the test suite and passes in CI.

## Queue and cleanup rules

- A blocked state callback is held by a caller-controlled gate. A virtual-time
  state burst fills the finite client queue; saturation must close the session
  with `ErrEventQueueFull`, never silently drop or grow memory.
- Command observation overflow is asserted through
  `DeviceStats.DroppedCommands`. Deterministic acceptance declares
  `Scenario.Commands`, uses `WaitForCommandExpectations(ctx)` after the producer
  is quiescent, and classifies its typed ordered outcomes.
- `Device.Close` must release accepted connections, listeners, virtual waits,
  and simulator-owned goroutines even when a callback remains blocked.
- Keep implementation claims aligned with the ledger in
  `references/scenario-contract.md`; issue #2 accepts the design and issue #10
  owns the remaining simulator machinery.

## Contract-first behavior

When a requested simulator behavior is accepted by the scenario contract but
not yet present in its implementation ledger, implement the smallest reviewed
slice under the owning issue. Do not create an ad hoc socket server or helper
whose behavior bypasses the shared framing layer.
