# Simulator field notes

Use these notes when writing or checking user-facing simulator docs, MGMT
acceptance scripts, or external-app examples.

## Standalone application demo

- Prefer `cmd/conveyor-sim` for the first non-MGMT executable. It creates an
  in-process simulator, opens a secure client session, lists entities,
  subscribes to state, and sends Fan plus RGB Light commands.
- For a throwaway external module before a release exists, avoid
  `go get github.com/flavio-fernandes/go-aioesphomeapi` by itself. Use:
  `go mod edit -require github.com/flavio-fernandes/go-aioesphomeapi@v0.0.0`,
  then `go mod edit -replace ...="$repo_root"`, then `go mod tidy`.
- `go mod tidy` may still need network access for public third-party module
  checksums. That is expected; the local `replace` keeps this library coming
  from the checkout.

## MGMT simulator acceptance

- Prefer the checked-in scripts over ad hoc command sequences:
  `tools/test-mgmt-conveyor.sh` for the conveyor MCL and
  `tools/test-mgmt-baselines.sh` for the two original MGMT examples.
- These scripts intentionally use Linux user and network namespaces. Their
  synthetic `.local` names are answered by a multicast responder inside the
  namespace; they do not edit or bind-mount `/etc/hosts`.
  Sandbox runs can fail with netlink or DNS permission errors even when the
  host is capable. Rerun with the proper execution permission rather than
  weakening the script.
- The MGMT checkout may require a newer Go toolchain than this library. Check
  MGMT's `go.mod` before documenting build prerequisites.
- The conveyor cleanup assertion observes a second Fan stop during shutdown.
  A single transient miss can be timing-sensitive; rerun once before treating
  it as a regression, then inspect MGMT cleanup ordering and simulator logs.

## Virtual state and reconnect acceptance

- Adding a `Scenario` field must make
  `TestScenarioFieldInventoryRequiresValidationAndCloneReview` fail. Extend
  `Scenario.Validate`, `cloneScenario`, and that exact field inventory together;
  do not silence the guard until the new field is validated and defensively
  copied.
- Initial snapshots must contain at most one state per entity family/key. The
  same numeric key may be reused by a different family. Call `Scenario.Validate`
  before constructing the device; duplicate `States` or `InitialStates` entries
  return `ValidationDuplicateKey` with safe indexes and never select a winner.
- Create a clock with `simulator.NewManualClock()` and pass it through
  `simulator.WithManualClock(clock)`. `Device.Clock()` returns the same clock
  when a one-device test prefers the default.
- Put the first snapshot in `InitialStates`; legacy `States` remains supported,
  but setting both is a validation error. Put absolute updates in
  `StateTimeline` and call `Advance` or `AdvanceTo`; equal-time updates arrive
  in declaration order.
- Use `Device.DropConnections()` to test a reconnect without destroying the
  listener, future timeline, or latest-state store. After a state-changing
  command, the next subscriber must receive the changed value once and
  `Commands()` must remain empty unless MGMT intentionally sends another
  command.
- For ADR 0013 evidence, record the command before the drop, the reconnect
  snapshot, and the post-reconnect command count separately. A reconnect that
  replays a command or old timeline burst fails the lane.

## Ordered command expectations

- The implementation merged through PR #58 at
  `f789731bc9f460a92ef3a3db159387101a2066ab`.
- Put exact protobuf commands and consecutive counts in `Scenario.Commands`.
  `Scenario.Validate` rejects nil/unsupported messages, zero counts, and counts
  above `MaxCommandExpectationCount`; `New` clones every command before use.
- Call `WaitForCommandExpectations(ctx)` after the command producer is
  quiescent. A client `Ping(ctx)` is a useful real-protocol ordering barrier
  before the wait when trailing commands must be rejected deterministically.
- Classify failures with `errors.Is`: missing, unexpected, out-of-order, and
  observation overflow are separate sentinels. Missing also preserves the
  caller's cancellation or deadline cause. Typed errors expose only indexes
  and counts; never add a command payload to them.
- `Commands()` remains the exploratory compatibility stream. A full stream
  increments `DeviceStats.DroppedCommands` and also fails an active declared
  expectation with overflow, even when that same command completed the count.

## Slow subscriber saturation

- The implementation merged through PR #60 at
  `091b9af4f600dfa98b1ebea169265d2afc254047`.
- Use `WithCallbackQueueSize(1)`, block the first state callback on a
  caller-owned channel, then advance two equal-time timeline events. The first
  fills the queue and the second must close with `ErrEventQueueFull`.
- Wait for `Client.Done`, inspect `CloseReason`, and prove `Device.Close`
  returns while the callback is still held. Release the caller gate, then use
  `Client.WaitCallbacks(ctx)` before asserting the exact final callback count.
- The dispatcher checks shutdown before dequeuing another event and before
  invoking each callback. Never replace this ordering with a timed sleep or an
  unbounded callback drain.
- `WaitCallbacks` observes cleanup only. It cannot cancel or forcibly unwind
  application callback code; its context bounds the caller's wait.

## Deterministic network shaping

- Put typed `NetworkFault` values in `Scenario.Network`. Use
  `NetworkFragmentFrame`, `NetworkCoalesceSegments`, or `NetworkDelayReply` at
  an existing protocol trigger. Each action affects only the next
  server-to-client response frame.
- Fragmentation and coalescing take no duration. A delayed reply requires a
  positive duration no greater than `MaxNetworkDelay`; advance the device's
  `ManualClock` to release it. Never add a real sleep to make this path work.
- Wait for `DeviceStats.NetworkPendingDelays` before asserting or advancing a
  delayed operation. The counter is an observation barrier, not scenario data.
  After release or cleanup it must return to zero.
- Delayed complete frames use the connection's ordered, fixed-size writer
  queue. `ManualClock.Advance` still commits every due state synchronously, but
  does not wait for a future virtual network deadline. Later frames remain
  behind the delayed frame and cannot overtake it.
- Never replace the fixed 64-frame queue with an unbounded slice or a goroutine
  per frame. Queue overflow closes the connection; tests must prove cleanup as
  well as the expected failure.
- Keep the real client, Noise handshake, and shared framer in the test path.
  Successful exact protobuf decoding plus subsequent `Ping` proves shaping did
  not rewrite bytes or poison the session.
- `Device.Close` and `Device.DropConnections` must cancel pending delay waits.
  Unknown network actions remain no-ops for forward compatibility.

## Evidence hygiene

- Expected outputs should be short deterministic text. Do not save traffic,
  real keys, private hosts, local usernames, camera media, or hardware details.
- A doc command is not done until the exact command has been run, or the doc
  clearly states why it is illustrative and not runnable.
