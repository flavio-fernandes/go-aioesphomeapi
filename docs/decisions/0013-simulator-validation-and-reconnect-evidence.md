# ADR 0013: Preserve simulator construction while validating before use

- Status: accepted
- Date: 2026-07-18

## Context

The accepted scenario contract required typed validation but the public
`simulator.New` function returned only `*Device`. Changing that signature would
break existing library, MGMT, and external callers. Panicking or silently
normalizing an invalid scenario would conflict with the library's error rules.

The same contract required every future `Seed` to be non-zero even though a
seed affects only explicitly randomized actions. Rejecting the zero value as
soon as the field appeared would break existing raw `Scenario` literals that
contain no randomness.

Finally, issue #10 will replace the current per-connection state map with a
device-global latest-state snapshot. That is more faithful, but it can remove a
second MGMT corrective command after reconnect. Existing acceptance evidence
must not silently hide that observable change.

## Decision

`Scenario.Validate() error` is the public preflight. It returns
`*ValidationError`, unwraps to `ErrInvalidScenario`, and reports only a field,
index, related index, and stable `ValidationCode`. It never includes scenario
names, entity names, keys, values, addresses, or credentials.

`New(Scenario, ...Option) *Device` keeps its existing signature. It validates
and defensively copies a valid scenario. If validation fails, it creates an
inert device that retains the error; both `DialContext` and `Serve` return that
same typed cause before creating a connection, reading a listener address, or
starting a goroutine. Future scenario fields extend `Validate` before their
runtime behavior is enabled.

`Seed == 0` is valid when the scenario declares no randomized action. When
randomized actions are added under issue #10, `Validate` must require a
non-zero seed only for a scenario that uses them and return
`ValidationSeedRequired` otherwise. Zero never selects hidden randomness or a
package-global random source.

The device-global latest-state rule remains accepted. Its implementation pull
request must deliberately re-baseline MGMT reconnect evidence:

1. record the pre-change library and MGMT revisions, unchanged MCL hashes, and
   observed reconnect command sequence;
2. run the focused MGMT reconnect/outage test and all unchanged baseline and
   conveyor MCL lanes against the candidate;
3. record the post-change latest-state snapshot and any corrective-command
   count change in a new append-only compatibility record;
4. update the support matrix without rewriting historical evidence.

A disappearing redundant correction is expected when the simulator already
retains the commanded state. Missing outage accounting, replay of a
non-idempotent command, MCL changes, or an unexplained command delta fails the
gate.

## Consequences

Existing `New` callers continue to compile. Custom-scenario authors can fail
early by calling `Validate`, while callers that do not preflight receive the
same error from the first operation that can report one. Validation cannot race
with caller mutation because a valid scenario is cloned before use.

Non-random raw literals remain source- and behavior-compatible. Randomized
features cannot be enabled without reproducibility. Issue #10 carries an
explicit cross-repository evidence cost when it changes reconnect snapshots.
