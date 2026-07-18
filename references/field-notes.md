# Sanitized maintainer field notes

These notes preserve reusable operational knowledge for repository skills. Do
not place credentials, private addresses, device identifiers, raw logs, local
absolute paths, or camera/serial output here.

## 2026-07-17 MGMT compatibility reconciliation

- Final reviewed library `main`: `6f954bc92a84b8a2bcb12acef5462b2445edfc08`.
- Final reviewed MGMT `feat/esphome`: `90a172d09239925db5a527ee7b2a5edc383c08a3`.
- Preserved comparison branch: `feat/esphome-richard87` at `5bf41f505bc601e6d2c4da8ecb3050b7c01ff34a`.
- Build MGMT with its documented `make build` target. A plain `go build` can
  produce a binary without the generated language/resource registration and
  that binary will correctly reject execution as incompletely compiled.
- Baseline and conveyor scripts must run in isolated multicast-capable network
  namespaces and must not substitute `/etc/hosts` for `.local` resolution.
- In restricted runners, put only build/module caches under a disposable
  temporary directory; never work around restrictions by broadening access to
  credential or home-directory files.
- Preserve raw real-device logs only in memory long enough to inspect them.
  Commit only sanitized assertions and exact public software revisions.
- The connected GitHub app is the preferred repository/issue/PR path. Do not
  replace an invalid file-backed CLI credential with another long-lived token.

See `docs/issue-status.md` for the current evidence ledger and exact remaining
work. Update both files when future work changes the operational truth.

## 2026-07-18 deterministic simulator contract

- Architecture issue #2 is a decision gate; implementation issue #10 owns the
  remaining virtual-clock, state-timeline, slow-subscriber, network-shaping,
  command-expectation, and resource-assertion code.
- Deterministic scenario time is a device-global duration starting at zero.
  Acceptance tests advance an injected manual clock; equal-time state events
  retain declaration order, and reconnect snapshots expose the latest state.
- A non-zero explicit seed is required only when a scenario declares randomized
  actions. Zero remains valid for deterministic raw literals; normal entity,
  state, log, and command order never depends on randomness.
- Network shaping belongs below plaintext or Noise framing on the simulator
  side of the real connection. Fragmentation and coalescing preserve bytes;
  delay and stall wait on virtual time or cancellation rather than sleeping.
- A slow callback fills the bounded client event queue and ends the ambiguous
  session with `ErrEventQueueFull`; it never causes silent drops or unbounded
  memory.
- Cleanup evidence counts resources owned by the simulator. Do not use exact
  process-wide goroutine counts as a stable assertion.
- `Scenario.Validate` is the public preflight. `New` remains source-compatible
  and defers its typed, secret-safe error to `DialContext` or `Serve`; a valid
  scenario is defensively copied before use.
- When issue #10 lands the device-global latest-state store, capture MGMT's
  reconnect command sequence before and after the change in a new append-only
  compatibility record. A redundant correction may disappear; outage
  accounting, no replay, unchanged MCL hashes, and all baseline/conveyor lanes
  remain mandatory.

## 2026-07-18 mDNS retransmit deadlines

- `net.UDPConn.SetDeadline` arms both reads and writes. If a read deadline is
  used as a retransmit timer, the timeout is already expired when the retry is
  sent and the UDP write fails immediately.
- Use `SetReadDeadline` for bounded mDNS retry scheduling. Keep a real UDP
  loopback regression because simple packet-connection fakes commonly record
  deadlines without enforcing the write-side behavior of a real socket.
- The full lookup budget, caller cancellation, hostname-bearing error chain,
  multicast source validation, and injected-dialer bypass remain part of the
  MGMT `.local` compatibility proof.

## 2026-07-18 dependency vulnerability response

- Dependabot and `govulncheck` answer different questions. Dependabot reports a
  vulnerable selected module version; source-mode `govulncheck` determines
  whether known vulnerable symbols are reachable from the current packages.
- Upgrade a compatible patched release even when the affected API appears
  unreachable. Here one `x/crypto` update closed 13 advisories without adding a
  module or raising the Go directive.
- Keep the official scanner version in one executable repository script. Run
  it on pull requests, `main` pushes, and a schedule; do not add its module to
  the library graph or automatically merge dependency updates.
- Verified security pin: library `f1f9e3ef9b5efca161aa97cbe0040d278fdb4038`
  and MGMT `feat/esphome` `ede1737219be106e2c5e06bb497af9a1ec9e17c8`.
  The committed MGMT graph and all three MCL acceptance lanes passed with the
  Go workspace disabled.

## 2026-07-17 M1 hostile-peer and lifecycle review

- The dial timeout covers TCP establishment, Noise, and Native API Hello as one
  budget. Cancellation must close an in-progress Hello on both transports.
- An injected `DialContext` follows `net.Dialer` semantics: its context bounds
  establishment, not the lifetime of the returned connection. The client owns
  the established connection until its session context or `Close` ends it.
- A bounded unknown message ID is forward-compatible and is skipped. A
  malformed payload for a known ID remains fatal and records `CloseReason`.
- Duplicate or spurious entity-list completion is untrusted input and must be
  harmless. Never close a completion channel twice.
- ESPHome key-rejection text is pre-authentication input. Keep the broad Noise
  handshake category, add a distinct rejected-key category, and sanitize and
  cap the displayed reason.
- `Ping(ctx)` is a caller-controlled liveness seam. A sent probe that times out
  closes the ambiguous connection so its late response cannot satisfy a later
  probe. Automatic keepalive policy remains separate work under issue #11.

## 2026-07-18 protocol compatibility inventory

- Keep canonical descriptor facts and reviewed compatibility claims separate.
  `protocol/inventory.annotations.json` is the small handwritten input;
  `protocol/inventory.json` is always generated from that input, the upstream
  lock, and the compiled protobuf descriptors.
- Every pinned message must expose the same fields and all evidence arrays.
  Generated presence earns only `known`; an empty array is stronger and safer
  than an inferred claim.
- The pinned protobuf does not declare when most messages were introduced.
  Record `not_declared_upstream`, the snapshot where the message is known, and
  a null minimum API version instead of inventing a compatibility floor.
- M1 currently accounts for 33 wire messages. Thirty-one have typed and
  simulated proof; DeviceInfo request/response remain known-only under issue
  #11. MGMT and hardware evidence is attached per message, so text-sensor and
  button gaps remain visible for issue #9.
- Keep unknown message IDs, enum values, and fields as separate plans. Unknown
  IDs are verified; unknown M1 enums and fields remain planned until focused
  tests establish their handwritten behavior.
