# ADR 0012: accept MGMT review fixture corrections

Status: accepted, 2026-07-17

## Context

The MGMT compatibility contract normally keeps reviewed MCL fixtures
byte-identical. MGMT issue #2 identified two defects in comments and naming:
the blink example described a nonexistent exec resource, and the conveyor
example mixed the misspelling “conveyer” with “conveyor” in its filename,
firmware name, endpoint, and hostname.

Leaving either defect in place would make the public instructions misleading.
The issue explicitly accepts these corrections, so they qualify for the
documented-defect exception to the immutable-fixture rule.

## Decision

- Keep `examples/lang/esphome0.mcl` unchanged at SHA-256
  `8a5ba295eb0a649af89592c0f42899d0078c642fa521c73a7224e00304daa7df`.
- Accept the comment-only `examples/lang/esphome-blink.mcl` correction at
  SHA-256 `359cedc5b3fd1e6793a0705fc4d7c7f844f5d3dc825a372fdf0c6769ef30c187`.
- Rename the conveyor fixture to `examples/lang/esphome-conveyor.mcl`, use
  `esphome-conveyor` consistently for its firmware, endpoint, and `.local`
  hostname, and accept SHA-256
  `38bb730a2897eeed1f8ed72e0299387ae2465e8b9b11b31c831f274e158868d6`.
- Update current acceptance scripts and user documentation. Historical
  append-only compatibility records retain the old paths and hashes because
  they describe runs that already happened.

## Consequences

The blink device needs no firmware change. A device flashed with the old
`esphome-conveyer` name must be reflashed with the corrected embedded firmware
or tested with an explicitly reviewed local host override. The release
acceptance lane does not use `/etc/hosts`; it proves the corrected `.local`
name through multicast DNS.
