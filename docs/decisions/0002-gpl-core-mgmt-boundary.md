# ADR 0002: GPL-3.0-only core with an external MGMT adapter

- Status: accepted for bootstrap
- Date: 2026-07-16

## Context

The library is intended for GPL-3.0-only publication, aligned with MGMT’s GPL licensing and resource lifecycle. MGMT is the first customer but not the only possible consumer.

## Decision

The core repository imports no MGMT packages and exposes only generic ESPHome concepts. MGMT-specific provider/resource code lives in the MGMT repository and imports this module.

## Consequences

The Go library remains independently reusable under GPL-3.0-only. The adapter can follow MGMT conventions without leaking domain types into the protocol client. End-to-end examples may span repositories and must pin compatible revisions.
