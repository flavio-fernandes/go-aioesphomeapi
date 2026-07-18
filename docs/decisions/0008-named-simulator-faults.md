# ADR 0008: Simulator faults use named actions at exact protocol triggers

- Status: accepted and implemented for the M1 fault and network-shaping slices
- Date: 2026-07-17

## Context

Happy-path simulation cannot prove that the client fails closed when a peer
drops a connection, sends malformed protobuf, advertises an unknown message,
duplicates entity-list completion, or stops replying. Arbitrary callback hooks would make scenarios difficult to
review and could bypass the shared Native API framing path.

## Decision

Scenarios may declare ordered `Fault` values. Each value combines one exported
`FaultTrigger` with one exported `FaultAction`. The initial vocabulary is:

- triggers after Hello, before entity-list completion, and after initial state;
- actions that drop the connection, send malformed protobuf, send a bounded
  unknown message ID, duplicate entity-list completion, or stall without an
  internal timer.

Fault frames pass through the same plaintext or Noise framer as ordinary
simulator traffic. A malformed payload for a known message remains fatal. A
bounded unknown message ID is skipped and subsequent known traffic must still
work; this is the protocol's forward-compatible path. Duplicate entity-list
completion is harmless and cannot panic the embedding process. A stall is
released by `Device.Close`; the tested client operation supplies the only
real-time deadline. Unknown fault values have no effect, so a newer scenario
cannot cause an older simulator to perform an unintended action.

`Scenario.Network` uses the same exact triggers for deterministic
server-to-client wire shaping. A named action applies only to the next response
frame. `NetworkFragmentFrame` splits that frame into one-byte connection
writes, `NetworkCoalesceSegments` combines its framing segments into one
connection buffer, and `NetworkDelayReply` holds it until the injected manual
clock advances by the declared positive, bounded duration. Shaping occurs
below both the Noise and plaintext framers, so clients consume the real wire
bytes. Unknown network actions have no effect.

The delay wait is owned by the connection and is released by `Device.Close` or
`Device.DropConnections`; it has no hidden timer or sleep. Public counters in
`DeviceStats` show armed frames, raw segments, and currently pending delays
without exposing scenario data. Scenario validation rejects missing,
non-positive, excessive, or inapplicable durations before any connection or
listener work.

Complete server frames pass through one ordered writer per connection. A
delayed timeline frame therefore does not block `ManualClock.Advance` while it
waits for a later virtual deadline, and following frames cannot overtake it.
The queue behind a delayed frame is fixed at 64 protocol-bounded frames; an
overflow fails the connection closed instead of growing memory or dropping a
frame silently. Closing the connection cancels the writer and its virtual wait.

This slice does not add random timing, packet capture, fixed ports, automatic
reconnect, or domain-specific conveyor policy. Conditional seed validation and
broader owned-resource budgets remain tracked by issue #10.

## Consequences

Tests can name hostile protocol and transport behavior at an exact point while
remaining deterministic, synthetic, dependency-free, and safe to run without
hardware. Real encrypted-client tests prove fragmented and coalesced bytes
decode exactly, virtual delays release only at their deadline, subsequent
traffic remains usable, delayed timeline updates retain declaration order
without blocking clock advancement, bounded queue overflow fails closed, and
shutdown cancels a pending delay. A passing scenario is simulator evidence
only; it is not ESPHome firmware, MGMT, hardware, or production evidence.
