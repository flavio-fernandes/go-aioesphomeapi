# ADR 0003: Pin ESPHome's upstream protocol definition

- Status: accepted
- Date: 2026-07-16
- Accepted: 2026-07-17

## Context

Current compatibility requires tracking the canonical wire definition. Generated types from a reference client can lag or carry library-specific modifications.

## Decision

Use `esphome/esphome` release 2026.7.0 at commit `920a8b761b680d9864da2ef4b44b4af95c99dba8` as the first protocol source. Import unmodified `esphome/components/api/api.proto`, `api_options.proto`, and the upstream license with recorded SHA-256 values.

Generate with official `protoc` v31.1 and `protoc-gen-go` v1.36.11 under Go 1.25.7. Generated types are internal by default, with the narrow public `pb` compatibility exception accepted by ADR 0006.

## Consequences

Upstream changes become visible quickly without automatically becoming stable handwritten API. Compatibility `pb` drift is separately reported to MGMT. A future sync changes the immutable commit only in a dedicated protocol pull request.
