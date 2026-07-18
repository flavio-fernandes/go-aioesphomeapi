---
name: sync-esphome-protocol
description: Synchronize the pinned ESPHome Native API protobuf definition, generated Go wire types, protocol inventory, provenance record, and support matrix. Use for an ESPHome release update, api.proto change, unknown-message finding, generator change, or compatibility review.
---

# Sync ESPHome Protocol

Keep upstream bytes and compatibility claims reproducible without copying a reference client's implementation.

## Workflow

1. Read `docs/provenance.md`, `docs/support-matrix.md`, ADR 0003, and `references/sync-checklist.md`.
2. Require a Gate 0-approved issue naming the immutable `esphome/esphome` commit. If the lock schema or generator toolchain is not accepted, stop and report that prerequisite; do not invent it inside a sync PR.
3. Start from a clean branch. Never reuse a locally downloaded protocol file without verifying its repository, commit, path, SHA-256, and license.
4. Run the repository's pinned fetch/generate command. Do not install an unpinned generator globally or use a mutable container tag.
5. Inspect the protocol inventory diff before handwritten code. Classify additions, removals, field changes, enum changes, and message-ID changes. Update `protocol/inventory.annotations.json` for any changed gate, MGMT need, parity class, public behavior, evidence, or unknown-value plan; never hand-edit `protocol/inventory.json`.
6. Update `protocol/upstream.lock.json`, generated wire output, annotation input, generated inventory, `THIRD_PARTY_NOTICES.md` when needed, and `docs/support-matrix.md` in one PR.
7. Record new generated definitions as `known` only. Raise evidence levels only when the corresponding public API and tests exist.
8. Run `go run ./cmd/protocol-inventory -check protocol/inventory.json`, clean regeneration, policy validation, unit/integration tests, race tests, fuzz smoke, and compatibility tests required by the changed surface.
9. In the PR, include upstream commit, hashes, generator versions, inventory summary, compatibility risks, and deliberately unsupported features.

## Hard stops

- Do not copy code from Python or Go reference clients.
- Do not accept mutable refs, unreviewed source patches, unexplained generated diffs, or missing license evidence.
- Do not combine a broad protocol sync with unrelated public API redesign.
