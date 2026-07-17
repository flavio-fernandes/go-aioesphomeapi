# Milestone 1 implementation sequence

Follow this order. A later slice may start only when the prior slice's listed evidence passes. Keep each numbered slice in its own small pull request unless an accepted issue explicitly combines them.

## Fixed inputs

- MGMT contract: `compatibility/mgmt-feat-esphome.json`
- architecture: `docs/architecture.md`
- dependency gate: `docs/dependency-policy.md`
- security limits: accepted result of issue 6
- wire source: accepted ESPHome commit and generator from issue 4
- simulator contract: accepted result of issue 2

If any fixed input is still proposed or ambiguous, stop at Gate 0. Do not invent the decision in an implementation pull request.

## Slice 1 — protocol lock and generated messages

Produce the pinned upstream lock, deterministic generator, `pb` compatibility package, internal message registry, and protocol inventory. Generate only from official ESPHome source. Run clean regeneration and uniqueness checks. Mark support as `known`, never implemented.

## Slice 2 — configuration and error types

Implement secret-safe configuration, explicit secure and insecure constructors/options, dialer injection, finite limits, API version values, typed redacted errors, and compile-only public API examples. Add no network goroutine yet.

Stop if a zero/omitted key can silently select plaintext in the normal production path.

## Slice 3 — plain framing and simulated peer skeleton

Implement bounded plain framing against an in-process loopback simulated peer. Cover fragmented/coalesced reads, partial writes, invalid preambles/varints/lengths/types, cancellation, deadlines, allocation bounds, and close.

Plain transport is reachable only through the explicit insecure test/development API.

## Slice 4 — Noise transport

Add the single approved Noise dependency behind the same framing/session interface. Test correct key, wrong key, name validation decision, truncation, authentication failure, downgrade refusal, deadlines, cancellation, and redaction. Report the exact module and Go-version delta.

Do not implement or modify cryptographic primitives.

## Slice 5 — one connection lifecycle

Implement hello, API version capture, device information, ping, disconnect, done signaling, and idempotent close. Network reads feed bounded internal routing; user callbacks never execute on the read loop. Closing or cancelling waits for owned goroutines.

Do not add MGMT pooling, polling, outage tracking, or default reconnect here.

## Slice 6 — discovery and immutable registry

Discover binary sensor, sensor, text sensor, switch, number, button, and fan descriptors. Index exact current names and legacy object IDs, detect ambiguity, and return copied immutable views. Unknown messages/fields/enums remain safe.

Expose only the MGMT-required generated registry accessors plus the preferred typed API.

## Slice 7 — state, logs, and bounded delivery

Subscribe to the five MGMT state families plus fan. Preserve missing values and sensor NaN semantics. Add bounded, redacted logs. Define queue overflow and unsubscribe behavior. The simulator sends initial snapshots, updates, bursts, and slow-consumer faults.

## Slice 8 — commands

Implement switch, number, button, and fan commands with entity-family and capability validation. A returned success means the message was written under the documented connection state; it does not claim physical completion. Disconnect never replays a command.

## Slice 9 — MGMT compatibility facade

Compile the pinned MGMT driver using this module. Start with import-path changes only. Record every additional adapter change and its safety rationale. Run MGMT session tests for persistent/polling behavior, logs, names without object IDs, connection failure, queued commands, outage state, and cleanup.

MCL file hashes must still match the manifest.

## Slice 10 — MCL scenarios

Run `esphome0.mcl` and `esphome-blink.mcl` unchanged against deterministic simulator scenarios. Assert one device connection per endpoint, state-driven graph events, expected commands and logs, reconnect behavior owned by MGMT, and zero leaked goroutines.

Raise relevant rows to `mgmt` evidence only after this slice passes.

## Slice 11 — conveyor extension

Add the generic fan behavior and new conveyor MCL/firmware contracts without changing the two baseline examples. Run the complete simulator story before requesting any workbench action. Physical flashing or motion remains a separately authorized task.

## Slice 12 — M1 release candidate evidence

From clean checkouts, run repository validation, generation drift, format, vet, tests, race, fuzz smoke, resource bounds, dependency report, MGMT cross-repository suite, and simulator demos. Publish exact revisions and limitations. Do not label the result production-ready unless the support matrix reaches `production` through all required gates.
