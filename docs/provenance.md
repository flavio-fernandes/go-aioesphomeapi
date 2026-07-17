# Protocol and compatibility provenance

## Wire source of truth

Protocol synchronization starts from `esphome/esphome`, specifically `esphome/components/api/api.proto`, at an immutable commit SHA. Release tags can guide selection but do not replace a commit pin.

Each sync records upstream repository and URL, commit and release, source and license SHA-256, compiler/plugin versions, generated diff, protocol inventory changes, support-matrix changes, and test results. The machine-readable lock lives at `protocol/upstream.lock.json`.

## Compatibility research sources

Two immutable snapshots inform the current design:

- MGMT `feat/esphome` at `8eab220` defines external MCL and adapter behavior.
- `Richard87/esphome-apiclient` `v1.1.0` at `982fb85860e7214e3384e68cb69bf94b16a6985b` defines the initial Go migration comparison.

The original snapshot remains immutable. A second append-only record, `compatibility/mgmt-feat-esphome2.json`, captures the rebased MGMT baseline, the replacement branch, this library's exact candidate revision, preserved MCL hashes, dependency delta, and verification results. A later run creates another record instead of rewriting either historical record.

The local manifest records only public repository paths, symbols, revisions, and SHA-256 values. It does not vendor the GPL MGMT source or reference-client implementation.

## Clean implementation rule

Implement wire behavior from the official protocol definition and public documentation. Use reference clients to identify interoperability questions, observable behavior, and test cases. Do not transliterate or copy their implementation. If any compatible fragment is intentionally derived, record its source, commit, license, file, rationale, and required notice before merge.

Black-box and cross-repository tests may compile or execute a pinned external checkout. Test results are evidence; external source does not become this repository’s GPL-3.0-only content.

For `.local` compatibility, the reference snapshot was inspected only to
confirm the observable behavior: names ending in `.local` are resolved with an
IPv4 multicast DNS A query before TCP dial. The implementation in
`internal/mdns` was written independently from DNS wire-format rules and Go's
standard networking interfaces; no reference-client source was copied.

## Generated code

Generated output includes generator markers and source attribution. Clean generation must reproduce committed files exactly. CI fails on generated drift.

The `pb` package is permitted as the narrow MGMT compatibility exception described by ADR 0006. Generated presence means only `known` support.

## Current record

ESPHome 2026.7.0 protocol definitions and their generated Go output are committed with the exact upstream license and lock. No third-party implementation source is committed. Generated presence raises protocol inventory entries only to `known`.
