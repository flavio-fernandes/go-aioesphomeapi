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
3. After checks pass, run:

   ```bash
   python3 .agents/skills/merge-reviewed-pr/scripts/audit_review_threads.py \
     flavio-fernandes/go-aioesphomeapi PR_NUMBER --require-codex
   ```

   This must prove the explicitly trusted `chatgpt-codex-connector` identity
   reviewed the exact head commit, or reacted positively to a trusted review
   request that names the exact full head SHA. For an edited request, the
   reaction must follow GitHub's server-controlled comment-update time.
   Commit-authored timestamps do not establish freshness. Similar-looking
   usernames are not trusted. Flat PR comments are not a substitute for
   thread-aware GraphQL data.
4. Treat every unresolved thread as blocking. Implement actionable feedback
   with a focused regression. For an intentional tradeoff or superseded
   finding, record the exact rationale or correcting PR. Never resolve a thread
   merely to make the gate green.
5. Push any correction and restart at step 1 because the head SHA changed.
   Reply with the correcting commit/PR or evidence, then resolve the thread.
6. Run the audit command again immediately before merging. Merge with an
   expected-head SHA so a concurrent push cannot bypass the reviewed commit.
7. Verify the linked issue closed when appropriate and that `main` protection
   still requires PRs, `go`, `validate`, stale-review dismissal, and resolved
   conversations for administrators.

## Stop conditions

Do not merge while Codex review is pending, any conversation is unresolved,
the head changed after review, a required check is missing, or a finding lacks
test/evidence. Branch protection is a backstop, not permission to skip this
workflow.
