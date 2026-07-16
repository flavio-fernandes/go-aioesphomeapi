# Protocol provenance policy

## Source of truth

Protocol synchronization starts from `esphome/esphome`, specifically `esphome/components/api/api.proto`, at an immutable commit SHA. Release tags may help choose a commit but are not sufficient provenance by themselves.

For each sync, record:

- upstream repository and HTTPS URL;
- immutable commit SHA and upstream release, if any;
- source file SHA-256;
- upstream license file and its SHA-256;
- protobuf compiler and Go plugin versions;
- generated-file diff summary;
- protocol inventory and support-matrix changes;
- tests executed and their results.

The future machine-readable record will live under `protocol/upstream.lock.json`. The first protocol sync issue defines and reviews its schema before fetching source.

## Clean implementation rule

Use the official protocol definition and public behavior documentation to implement behavior. Reference clients may be used to identify test cases and interoperability questions. Do not transliterate or copy their implementation. If a small compatible fragment is intentionally derived, record the source, commit, original license, file, and rationale in this document and preserve all required notices.

## Generated code

Generated output must include its generator marker and source attribution. A clean generation command must reproduce committed files exactly. CI fails if regeneration changes the tree.

## Current record

No protocol definition, generated code, or third-party implementation source is committed in the bootstrap. Documentation links are references, not vendored material.
