# Contributing

This project is architecture-first and uses pull requests for every non-bootstrap change.

## Before changing code or protocol files

1. Read `AGENTS.md`, `docs/architecture.md`, and `docs/security-threat-model.md`.
2. Link the change to a narrowly scoped issue and milestone.
3. For protocol changes, invoke the `sync-esphome-protocol` project skill.
4. Update the support matrix with evidence; never infer support from generated types alone.
5. Run `./tools/validate-repo.sh` and the tests introduced by the relevant milestone.

## Pull request expectations

- Keep generated wire code, handwritten public APIs, and domain examples in separate changes where practical.
- Add negative-path tests for malformed input, disconnects, cancellation, and resource limits.
- State security, compatibility, provenance, and MGMT impact explicitly.
- Never include secrets, local paths, private project links, device identifiers, network details, camera captures, or personal contact information.
- Require review and passing checks before merge. Force-pushes and direct pushes to the default branch are not part of the normal workflow.

By contributing, you agree that your contribution is licensed under this repository's GNU General Public License v3.0 only.
