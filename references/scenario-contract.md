# Simulator scenario contract

This is the accepted design contract for the public `simulator` package. It is
the shared checklist used by the `$run-device-simulator` skill and the fixed
input for Milestone 1 simulator work. The simulator implements the device side
of the real ESPHome Native API path; it is never a mock of the client.

The word **must** below is normative. A behavior is not implementation evidence
until a checked-in test proves it. The implementation ledger at the end keeps
accepted design separate from shipped behavior.

## Safety and wire fidelity

- All built-in scenarios, names, keys, addresses, logs, and payloads must be
  synthetic. The default Noise key is public test data and must never be used
  by a real device.
- Noise is the default. Plaintext requires the explicit test-only option.
- An injected `net.Pipe` dialer must bypass default TCP dialing and mDNS. A TCP
  listener must be caller-owned, ephemeral, and loopback-only.
- The peer must use the same protobuf messages, message registry, framing, and
  Noise implementation as the client. Faults must cross that shared path; a
  mock-client-only shortcut is forbidden.
- The simulator must remain dependency-free beyond modules already required by
  the core library. It must not import MGMT or copy MGMT source.
- Simulator evidence must stay labeled `simulator`; it cannot be promoted to
  firmware, hardware, or production evidence.

## Normative scenario model

The implementation may add fields without breaking callers, but the following
concepts and semantics are stable for Milestone 1:

```go
type Scenario struct {
    Name          string
    Seed          uint64
    Entities      []proto.Message
    InitialStates []proto.Message
    StateTimeline []StateEvent
    Logs          []*pb.SubscribeLogsResponse
    Commands      []CommandExpectation
    Network       []NetworkAction
    Faults        []Fault
}

type StateEvent struct {
    At    time.Duration
    State proto.Message
}
```

The current compatibility field named `States` represents `InitialStates` and
must remain supported while the clearer name is introduced. Scenario creation
must defensively copy mutable protobuf values before another goroutine can
observe them. Invalid entity/state types, duplicate keys within one entity
family, negative times, decreasing timelines, a randomized scenario without a
seed, and impossible expectations must fail with a typed, secret-safe
validation error. They must not silently change the scenario.

### Validation surface

`Scenario.Validate() error` is the explicit preflight. Failures are
`*ValidationError` values that unwrap to `ErrInvalidScenario` and contain only a
field, index, related index, and stable validation code. They never contain
scenario/entity names, keys, values, addresses, or credentials.

`New(Scenario, ...Option) *Device` retains its compatible return signature. It
validates and defensively copies a valid scenario. An invalid scenario creates
an inert device; `DialContext` and `Serve` return the stored typed validation
error before opening a connection, inspecting a listener, or starting a
goroutine. Every future scenario field extends `Validate` before the field can
affect runtime behavior.

Built-in scenarios remain generic device fixtures:

- `BasicIOScenario` corresponds to the device shape required by
  `esphome0.mcl`;
- `BlinkScenario` corresponds to `esphome-blink.mcl`;
- `ConveyorScenario` composes ordinary ESPHome entities for the demonstration
  and does not introduce conveyor types into the core library.

## Virtual time and random seed

- Deterministic acceptance tests must use an injected manual clock. Scenario
  time starts at duration zero; wall-clock dates and the host time zone have no
  meaning.
- Advancing the manual clock is explicit and synchronous: all work due at or
  before the new time must be observable before `Advance` returns. Moving time
  backwards must fail.
- Production-style TCP demos may opt into a real clock, but their timing is not
  deterministic evidence and every wait must still be context-bounded.
- A scenario that declares one or more randomized actions must declare a
  non-zero `uint64` seed. Zero is valid when no randomized action exists, so
  adding the `Seed` field does not break existing raw literals. No
  package-global random source is allowed. A randomized helper must derive all
  choices from that seed and include the seed in a sanitized failure so the run
  is repeatable.
- Default scenarios use named constants for their seed. A seed controls only
  explicitly randomized actions; it must not reorder normal entities, initial
  states, logs, commands, or equal-time events.

## State timelines and reconnects

- Initial states are sent once, in declaration order, after each successful
  state subscription.
- The virtual timeline is device-global. Events use absolute offsets from
  scenario start. Equal-time events preserve declaration order.
- A due event updates the simulator's current state and is pushed to every
  active state subscriber through the real wire path.
- A client that reconnects receives one initial snapshot containing the latest
  state for every known key, then only future events. Past events are not
  replayed as a burst.
- A command that changes an emulated output updates the same current-state
  store before its response is emitted. The scenario must define the ordering
  when a command and timeline event occur at the same virtual instant; for M1,
  an already received command is applied first.
- Timeline events are data, not arbitrary callbacks. They cannot run user code,
  sleep, dial a network, or bypass framing.

Implementing the device-global latest-state store changes the current
per-connection simulator baseline. Issue #10 must record the pre-change MGMT
reconnect command sequence, then rerun the focused reconnect/outage test and
all unchanged baseline/conveyor MCL lanes. A new append-only compatibility
record must pin both revisions, unchanged MCL hashes, latest-state snapshot,
and any corrective-command count delta. Historical records are not rewritten.

## Commands and logs

- `Commands()` continues to expose defensive copies for exploratory consumers.
  Saturation must remain visible through `DeviceStats.DroppedCommands`.
- Deterministic tests use ordered command expectations, not sleeps or channel
  peeking. An expectation identifies the exact protobuf command and count.
  Helpers must accept a context, report missing, unexpected, out-of-order, and
  overflow outcomes distinctly, and compare protobuf values without retaining
  caller-owned mutable messages.
- A successful expectation means that the simulated peer received the command;
  it never claims physical completion.
- Logs use synthetic bytes and declared levels. Subscription filtering must
  follow the requested ESPHome log level. No helper may store an unbounded log
  history.

## Slow subscribers and queue outcome

The client callback queue is finite. Callbacks run serially off the network read
loop. A blocked callback may therefore fill the queue while the read loop
continues. When the queue is full, the client must close the ambiguous session
with `ErrEventQueueFull`; it must not block the read loop forever, discard an
unreported state, grow memory, or invoke callbacks concurrently.

The simulator proves this with a virtual-time state burst and a caller-controlled
callback gate. `Device.Close` and context cancellation must release the test
even while the callback is blocked. Tests must assert the close reason and the
final callback count instead of relying on scheduler timing.

## Network shaping and hostile peers

Network shaping belongs on the simulator side of the actual `net.Conn`, below
plaintext or Noise framing. It changes read/write boundaries or availability;
it does not decode and re-encode a frame after encryption.

Each action is named, parameter-bounded, attached to an exact protocol trigger,
and executed in declaration order:

- **delay** gates the next read or write until a specified virtual time;
- **fragment** caps the bytes in each underlying read or write segment;
- **coalesce** combines a bounded number of adjacent segments before delivery;
- **drop** closes the connection at the declared trigger;
- **stall** waits for clock advance, context cancellation, or simulator close;
- **malformed** emits one explicitly bounded invalid frame;
- **unknown message** emits one bounded unrecognized message ID and then
  continues normally.

Fragmentation and coalescing must preserve bytes exactly. Delay and stall use
the injected clock, never an unbounded `time.Sleep`. Shape parameters have
finite defaults and maxima so a scenario cannot request unbounded buffering or
allocation. A newer unknown action must have no effect in an older simulator.

The accepted current fault vocabulary also includes duplicate entity-list
completion. Duplicate completion must be harmless; malformed payload for a
known message remains fatal; a bounded unknown message ID is skipped and later
known traffic continues.

## Cleanup and resource assertions

- `Device.Close` is idempotent and owns shutdown of accepted connections,
  simulator goroutines, virtual waits, and registered listeners. It must not
  wait for user callbacks to return.
- Tests must use a context-bounded `WaitIdle`-style helper and assert zero active
  connections, zero owned goroutines, resolved command expectations, and the
  expected overflow counters. Raw `runtime.NumGoroutine` equality is not a
  stable ownership assertion.
- Every blocking helper accepts `context.Context`. Cancellation preserves
  `errors.Is` access to `context.Canceled` or `context.DeadlineExceeded`.
- Allocation and queue limits are tested separately under issues #6, #10, and
  #12. No cleanup helper may hide a non-zero counter by resetting it.

## Planned acceptance scenarios

| Scenario | Required proof |
|---|---|
| persistent MGMT | One endpoint connection, initial snapshot, ordered commands/logs, clean close. |
| polling MGMT | Caller-owned repeated connections with no simulator-owned reconnect. |
| reconnect/outage | Named drop, MGMT-owned retry, latest-state snapshot, no command replay, and reviewed before/after MGMT command evidence. |
| slow callback | Virtual state burst, `ErrEventQueueFull`, bounded cleanup. |
| malformed peer | One named malformed action, typed close reason, no panic. |
| transport shaping | Deterministic delay, fragmentation, and coalescing through plaintext and Noise paths. |
| clean shutdown | All listeners, connections, virtual waits, and owned goroutines end within a context deadline. |

## Fidelity limits

The simulator is a protocol peer, not ESPHome firmware. It intentionally does
not emulate ESP-IDF/Arduino scheduling, Wi-Fi association, DHCP, radio loss,
flash persistence, boot timing, Home Assistant behavior, YAML automations,
component drivers, OTA, web servers, or physical sensors and actuators. It does
not prove electrical safety, motor interlocks, real-time deadlines, or firmware
compatibility. Those claims require their separate firmware, workbench, and
hardware evidence levels.

The standalone mDNS responder used by isolated MGMT acceptance is test
infrastructure, not simulated ESPHome service discovery. The in-process dialer
correctly bypasses it.

## Implementation ledger

| Contract part | Current evidence | Remaining implementation owner |
|---|---|---|
| real Noise/framing/session peer; secure default; injected and loopback dialing | simulator unit tests and reviewed MGMT baseline/conveyor lanes | complete for M1 contract |
| generic basic-I/O, blink, and conveyor fixtures; initial states and logs | `simulator/basic.go`, `simulator/conveyor.go`, and acceptance scripts | complete for M1 contract |
| command observation and visible overflow | `Commands`, `DeviceStats.DroppedCommands`, `Scenario.Commands`, typed context-bounded expectation outcomes, and race tests | complete for M1 contract |
| named drop, malformed, unknown, duplicate-completion, and stall faults | ADR 0008 and real-wire fault tests | delay/fragment/coalesce in #10 |
| validation surface and defensive scenario copy | `Scenario.Validate`, deferred `DialContext`/`Serve` rejection, and simulator tests cover initial states, timeline events, and command expectations | future randomized/network fields extend validation in #10 |
| manual clock and pushed latest-state timeline | `ManualClock`, `StateTimeline`, device-global snapshots, `DropConnections`, and repeated race tests | final pinned MGMT re-baseline required by ADR 0013 |
| conditional non-zero seed and slow-subscriber proof | zero remains valid because no randomized action exists; semantics accepted above | randomized actions and queue-saturation scenario in #10 |
| device information, keepalive, callback isolation, connection-state cleanup | lifecycle tests and MGMT acceptance are partial | #11, with budgets in #6/#12 |

Closing architecture issue #2 accepts this contract; it does not claim the
open #10 or #11 implementation rows are complete. ADR 0013 resolves validation,
conditional seed, and reconnect re-baselining details. Any later change to the
normative rules requires another reviewed ADR update before implementation.
