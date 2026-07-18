# Simulator scenario contract

This file is the shared checklist used by the `$run-device-simulator` skill.
The accepted architectural rule is that the simulator implements the device
side of the real framing and Native API path; it is never a mock client.

## Current contract

- Inputs and identifiers are synthetic and deterministic.
- Noise with a public test-only key is the default; plaintext is explicit.
- Injected `net.Pipe` connections bypass default dialing and mDNS.
- TCP listeners are caller-owned and loopback-only.
- Scenarios declare entities, initial states, logs, and ordered named faults.
- Commands are observable through defensive message copies.
- `Close` releases listeners, connections, and timer-free stalls.
- Simulator evidence is never relabeled as firmware or hardware evidence.

## Accepted current fault vocabulary

Triggers occur after Hello, before entity-list completion, and after initial
states. Actions drop the connection, send malformed protobuf, send an unknown
message, or stall until caller cancellation or simulator close.

## M1 contract gaps

Issue #2 remains open until the contract also decides and tests:

- explicit virtual clock and random-seed semantics;
- pushed-state timelines after the initial snapshot;
- delayed, fragmented, and coalesced transport behavior;
- slow-subscriber saturation and the documented queue outcome;
- command expectation helpers and final goroutine/resource assertions;
- fidelity limits for behavior that is intentionally not ESPHome firmware.

Every added behavior must use the shared wire path, remain dependency-free,
and use no hardware, private network data, or production credential.
