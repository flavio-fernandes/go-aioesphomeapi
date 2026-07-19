---
name: merge-reviewed-pr
description: Merge a go-aioesphomeapi pull request only after required checks pass, Codex has reviewed the exact head commit, and every review conversation is addressed and resolved. Use for any request to merge, finalize, land, or make a repository pull request official.
---

# Merge Reviewed PR

Fail closed: a green workflow is necessary but never sufficient to merge.

## Workflow

1. Record the PR number and exact head SHA. Confirm the change is scoped to its
   issue and contains no secret, personal, hardware, or unrelated work.
2. Wait for the required `go` and `validate` checks. Do not poll repeatedly;
   make one bounded check and stop if GitHub still needs time.
3. Do not request Codex review unless the user explicitly authorizes that paid
   action for this PR. Without that authorization, stop with the PR safely
   blocked by the required `codex-review` status. When authorized, run:

   ```bash
   ./tools/codex-review.sh request PR_NUMBER
   ```

4. After Codex finishes, treat every unresolved thread as blocking. Implement
   actionable feedback with a focused regression. For an intentional tradeoff
   or superseded finding, record the exact rationale or correcting PR. Never
   resolve a thread merely to make the gate green.
5. Push any correction and restart at step 1 because the head SHA changed.
   A status belongs only to one commit. Reply with correcting evidence, resolve
   the thread, and request another review only with explicit user authorization.
6. When the exact head is final, run:

   ```bash
   ./tools/codex-review.sh complete PR_NUMBER
   ```

   This proves the explicitly trusted `chatgpt-codex-connector` identity
   reviewed the exact head commit, reacted positively to a trusted review
   request that names the exact full head SHA, or posted its clean comment
   verdict ("Didn't find any major issues" with a backticked
   `**Reviewed commit:**` prefix of at least ten hexadecimal characters
   matching the head). For an edited request, the reaction must follow
   GitHub's server-controlled comment-update time. Commit-authored timestamps
   do not establish freshness. Similar-looking usernames are not trusted.
   Other flat PR comments are not a substitute for thread-aware GraphQL data.
   The command publishes the required `codex-review` success status only when
   the audit passes.
7. Confirm `go`, `validate`, and `codex-review` are successful immediately
   before merging. Merge with an expected-head SHA so a concurrent push cannot
   bypass the reviewed commit.
8. Verify the linked issue closed when appropriate and that `main` protection
   still requires PRs, all three statuses, stale-review dismissal, and resolved
   conversations for administrators.

## Stop conditions

Do not request a paid review without explicit user authorization. Do not merge
while `codex-review` is absent, pending, or failing; any conversation is
unresolved; the head changed after review; another required check is missing;
or a finding lacks evidence. Branch protection is a backstop, not permission
to skip this workflow.
