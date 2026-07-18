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
