# ADR 0008: Simulator faults use named actions at exact protocol triggers

- Status: accepted for the initial M1 fault slice
- Date: 2026-07-17

## Context

Happy-path simulation cannot prove that the client fails closed when a peer
drops a connection, sends malformed protobuf, advertises an unknown message,
or stops replying. Arbitrary callback hooks would make scenarios difficult to
review and could bypass the shared Native API framing path.

## Decision

Scenarios may declare ordered `Fault` values. Each value combines one exported
`FaultTrigger` with one exported `FaultAction`. The initial vocabulary is:

- triggers after Hello, before entity-list completion, and after initial state;
- actions that drop the connection, send malformed protobuf, send an unknown
  message ID, or stall without an internal timer.

Fault frames pass through the same plaintext or Noise framer as ordinary
simulator traffic. A stall is released by `Device.Close`; the tested client
operation supplies the only real-time deadline. Unknown values have no effect,
so a newer scenario cannot cause an older simulator to perform an unintended
fault action.

This slice does not add random timing, packet capture, fixed ports, automatic
reconnect, or domain-specific conveyor policy. Fragmentation and transport
corruption remain framing-level tests until a reviewed network-fault seam is
needed.

## Consequences

Tests can name the hostile behavior and the exact point where it happens while
remaining deterministic, synthetic, dependency-free, and safe to run without
hardware. A passing scenario is simulator evidence only; it is not ESPHome
firmware, MGMT, hardware, or production evidence.
