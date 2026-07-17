# MGMT compatibility field notes

Use these notes for branch operations and cross-repository evidence. Keep new
facts concise and stable; detailed history belongs in compatibility manifests
or PR descriptions.

## Branch preservation and merge flow

- Before replacing the Richard87 MGMT branch, create
  `feat/esphome-richard87` from the old `feat/esphome` remote tip.
- After the replacement PR merges into `feat/esphome`, delete `feat/esphome2`
  only after verifying `feat/esphome` moved to the merge commit and
  `feat/esphome-richard87` still points at the preserved tip.
- Narrow local fetch refspecs can keep tracking deleted branches. If a fetch
  fails because `feat/esphome2` is gone, inspect `remote.origin.fetch`, remove
  only the stale refspec, fetch the surviving branches, and delete the local
  stale branch.

## PR and branch hygiene

- Check `gh pr view --json baseRefName,headRefName,isDraft,mergeable` before
  merging or rebasing a PR. Mark a PR ready only when the user has authorized
  merging and GitHub requires it.
- Use `--force-with-lease` only after a deliberate rebase of the PR branch.
- For stacked library PRs, merging a lower PR with squash can close or conflict
  higher PRs. Re-check the next PR after every merge and report whether it
  needs a new branch or replacement PR.

## Local verification habits

- Build MGMT with its own `go.mod` toolchain requirement, not this library's
  minimum Go version.
- Keep Go caches under writable temporary locations in managed workspaces.
- If a command fails because of sandboxed DNS, netlink, or network namespace
  permissions, rerun the same command with appropriate permission; do not
  weaken the acceptance script.
- No-hardware MGMT acceptance currently means the checked-in simulator scripts,
  not an ad hoc TCP server.
- Do not satisfy an immutable `.local` MCL hostname by injecting `/etc/hosts`.
  Start the checked-in multicast responder inside the private network namespace
  so the acceptance run proves the library's real mDNS path.

## Evidence records

- Record exact MGMT commit, library commit, MCL SHA-256 values, tested commands,
  and limitations in append-only compatibility files.
- Update `docs/support-matrix.md` only to the evidence level actually reached:
  `mgmt` for real MGMT over the simulator, not `hardware` or `production`.
- When auditing replacement parity, list deliberate security differences such
  as rejecting implicit plaintext separately from accidental regressions.
- A connection error must preserve its cause for `errors.Is`/`errors.As`, name
  the stage and attempted target, and never contain a Noise key. After `Done`,
  use `CloseReason` to distinguish a failure from intentional shutdown.
