# Third-party notices and provenance

No third-party source code is included in the bootstrap commit.

Future protocol definitions may be derived from ESPHome's `api.proto`. ESPHome documents the non-C++ portion of its source tree under the MIT license; every protocol sync must record the exact upstream repository, commit, file path, license text, and generator version before generated Go output is accepted.

Reference implementations are for behavioral research only:

- `esphome/aioesphomeapi` — MIT, official Python Native API client.
- `mycontroller-org/esphome_api` — Apache-2.0, historical Go client.
- `Richard87/esphome-apiclient` — MIT, independent Go client.

Do not copy implementation code from a reference repository merely because its license is compatible. Prefer clean implementation from the upstream protocol specification and record any intentional derivation in `docs/provenance.md`.
