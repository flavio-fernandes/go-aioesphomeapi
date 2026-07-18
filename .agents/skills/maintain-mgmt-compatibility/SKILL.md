---
name: maintain-mgmt-compatibility
description: Maintain go-aioesphomeapi's MGMT compatibility lane, including MGMT branch preservation, PR merges, module pins, unchanged MCL evidence, simulator acceptance, and append-only compatibility records.
---

# Maintain MGMT Compatibility

MGMT is the first release-blocking consumer. Keep the library generic, but make
MGMT evidence precise and reproducible.

## Workflow

1. Read `docs/mgmt-integration.md`, `docs/support-matrix.md`,
   `docs/dependency-policy.md`, and `references/field-notes.md`.
2. Identify both repositories, active branches, remotes, and dirty state before
   changing anything. Never assume `feat/esphome`, `feat/esphome2`, or a PR
   target still means what it meant in a prior turn.
3. Preserve external baselines before replacing them. For the Richard87 MGMT
   path, keep `feat/esphome-richard87` at the old `feat/esphome` tip before
   merging a replacement into `feat/esphome`.
4. Keep MCL compatibility mechanical. Existing pinned MCL files must remain
   byte-identical unless a documented MGMT defect fix has been accepted.
5. Pin this library in MGMT with an exact pseudo-version or release tag. Record
   the library commit, MGMT commit, MCL hashes, and module-graph delta.
6. Run the relevant MGMT unit, race, vet, `mgmt check`, and no-hardware
   simulator acceptance lanes. Use `$run-device-simulator` when creating or
   changing simulator scenarios or scripts.
   For `.local` resolver changes, include a real UDP loopback regression:
   multicast fakes do not reproduce `net.UDPConn`'s shared read/write deadline
   behavior. Use read-only deadlines for retry scheduling so an expired read
   deadline cannot poison a retransmit write.
7. Update append-only compatibility records and support-matrix evidence only
   for behavior proven by checked-in tests or recorded acceptance scripts.
8. Keep GitHub branch operations explicit: inspect PR base/head, preserve old
   branch tips, merge only after conflicts/checks are resolved, delete obsolete
   branches only after the replacement branch is verified.
9. Leave both worktrees clean and on the branch the next task is expected to
   use.

## Hard stops

- Do not silently edit existing MCL fixtures.
- Do not add MGMT imports to this library.
- Do not add a runtime dependency to make MGMT work without the dependency
  policy evidence.
- Do not treat a simulator pass as hardware evidence.
- Do not delete a branch that preserves a review baseline unless the user
  explicitly asks for that exact branch deletion.
