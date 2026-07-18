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

## Evidence hygiene

- Expected outputs should be short deterministic text. Do not save traffic,
  real keys, private hosts, local usernames, camera media, or hardware details.
- A doc command is not done until the exact command has been run, or the doc
  clearly states why it is illustrative and not runnable.
