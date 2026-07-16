# ADR 0003: Pin ESPHome's upstream protocol definition

- Status: proposed; accept at Gate 0 after generator selection
- Date: 2026-07-16

## Context

Current compatibility requires tracking the canonical wire definition. Generated types from a reference client can lag or carry library-specific modifications.

## Decision

Use `esphome/esphome`'s `esphome/components/api/api.proto` at an immutable commit as protocol source. Record source and license hashes and generate Go wire types reproducibly. Keep generated types internal.

## Consequences

Upstream changes become visible quickly without automatically becoming public API. Generator tooling and a machine-readable lock format must be selected and reviewed before the first sync.
