---
name: merge-reviewed-pr
description: Merge a go-aioesphomeapi pull request only after required checks pass, every review conversation is addressed and resolved, and the expected head SHA is confirmed. Codex review is optional. Use for any request to merge, finalize, land, or make a repository pull request official.
---

# Merge Reviewed PR

Fail closed: a green workflow is necessary but never sufficient to merge.

## Workflow

1. Record the PR number and exact head SHA. Confirm the change is scoped to its
   issue and contains no secret, personal, hardware, or unrelated work.
2. Wait for the required `go` and `validate` checks. Do not poll repeatedly;
   make one bounded check and stop if GitHub still needs time.
3. Inspect thread-aware review data and treat every unresolved conversation as
   blocking. Implement actionable feedback with a focused regression. For an
   intentional tradeoff or superseded finding, record the exact rationale or
   correcting PR. Never resolve a thread merely to make the gate green.
4. Codex review is optional. Do not request it unless the user explicitly
   authorizes that paid action for this PR. Its absence is not a blocker. When
   authorized, run:

   ```bash
   ./tools/codex-review.sh request PR_NUMBER
   ```

   If an optional review was requested, wait for it, address every resulting
   conversation, and then run:

   ```bash
   ./tools/codex-review.sh complete PR_NUMBER
   ```

   The resulting status records exact-head review evidence but is not required
   for merge.
5. Push any correction and restart at step 1 because the head SHA changed.
   Reply with correcting evidence, resolve the thread, and request another
   optional review only with fresh explicit user authorization.
6. Confirm `go` and `validate` are successful and review conversations remain
   resolved immediately before merging. Merge with an expected-head SHA so a
   concurrent push cannot bypass the inspected commit.
7. Verify the linked issue closed when appropriate and that `main` protection
   still requires PRs, `go`, `validate`, stale-review dismissal, and resolved
   conversations for administrators. Confirm it does not require the optional
   `codex-review` status.

## Stop conditions

Do not request a paid review without explicit user authorization. If an
optional Codex review was requested, do not merge before it finishes and its
conversations are addressed unless the user explicitly withdraws that request.
Do not merge while any conversation is unresolved, the head changed after
inspection, another required check is missing, or a finding lacks evidence.
Branch protection is a backstop, not permission to skip this workflow.
