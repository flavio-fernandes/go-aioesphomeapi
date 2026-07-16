# Contributing

This project is architecture-first and uses pull requests for every non-bootstrap change.

## Before changing code or protocol files

1. Read `AGENTS.md`, `docs/architecture.md`, and `docs/security-threat-model.md`.
2. Link the change to a narrowly scoped issue and milestone.
3. For protocol changes, invoke the `sync-esphome-protocol` project skill.
4. Update the support matrix with evidence; never infer support from generated types alone.
5. Follow `docs/documentation-style.md` and update `CHEATSHEET.md` when a user-facing command or prerequisite changes.
6. Run `./tools/validate-repo.sh` and the tests introduced by the relevant milestone.

## Make the first experience friendly

- Keep the working path short and simulator-first.
- Put prerequisites before commands and expected results after them.
- Test every command presented as runnable from its documented starting point.
- Use plain language and define specialized terms on first use.
- Treat a broken cheatsheet command or confusing first-use error as a product bug.
- Never document a future command as though it works today.

## Pull request expectations

- Keep generated wire code, handwritten public APIs, and domain examples in separate changes where practical.
- Add negative-path tests for malformed input, disconnects, cancellation, and resource limits.
- State security, compatibility, provenance, and MGMT impact explicitly.
- State whether `CHEATSHEET.md` changed and why. User-facing behavior needs a clean-clone example.
- Never include secrets, local paths, private project links, device identifiers, network details, camera captures, or personal contact information.
- Require review and passing checks before merge. Force-pushes and direct pushes to the default branch are not part of the normal workflow.

By contributing, you agree that your contribution is licensed under this repository's GNU General Public License v3.0 only.
