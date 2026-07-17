# Third-party notices and provenance

## ESPHome protocol definitions

`protocol/upstream/api.proto` and `api_options.proto` are unmodified copies from `esphome/esphome` release 2026.7.0 at commit `920a8b761b680d9864da2ef4b44b4af95c99dba8`. ESPHome's license states that non-C++ portions of its repository are MIT-licensed. The exact upstream license is preserved at `protocol/upstream/LICENSE`; source, license, and generator hashes are recorded in `protocol/upstream.lock.json`.

The `pb` Go files are generated from those definitions with `protoc` v31.1 and `protoc-gen-go` v1.36.11. They contain no reference-client source.

Every future protocol sync must record the exact upstream repository, commit, file path, license text, and generator version before generated Go output is accepted.

Reference implementations are for behavioral research only:

- `esphome/aioesphomeapi` — MIT, official Python Native API client.
- `mycontroller-org/esphome_api` — Apache-2.0, historical Go client.
- `Richard87/esphome-apiclient` — MIT, independent Go client.

The MGMT compatibility baseline is GPL-licensed external application code. It is pinned and exercised only through cross-repository tests; no MGMT source or MCL file is copied into this GPL-3.0-only repository.

Do not copy implementation code from a reference repository merely because its license is compatible. Prefer clean implementation from the upstream protocol specification and record any intentional derivation in `docs/provenance.md`.
