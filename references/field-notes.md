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
