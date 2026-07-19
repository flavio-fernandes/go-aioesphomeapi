#!/usr/bin/env python3
"""Fail unless one pull request has a completed Codex review and no open threads."""

from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
from typing import Any


TRUSTED_CODEX_LOGINS = frozenset({"chatgpt-codex-connector"})
TRUSTED_REVIEW_REQUEST_LOGINS = frozenset({"flavio-fernandes"})
ACCEPTED_CODEX_REVIEW_STATES = frozenset({"APPROVED", "COMMENTED"})

# Codex posts its clean verdict as an issue comment, not a review object:
# "Codex Review: Didn't find any major issues." plus a backticked
# "**Reviewed commit:**" prefix of the head SHA. Require both, and at least
# ten hexadecimal characters, before accepting the comment as head evidence.
CODEX_REVIEWED_COMMIT_PATTERN = re.compile(
    r"\*\*Reviewed commit:\*\*\s*`([0-9a-f]{10,40})`"
)
CODEX_CLEAN_VERDICT_MARKER = "Didn't find any major issues"


QUERY = """
query($owner: String!, $repo: String!, $number: Int!, $cursor: String) {
  repository(owner: $owner, name: $repo) {
    pullRequest(number: $number) {
      headRefOid
      reviews(last: 100) {
        nodes { author { login } state submittedAt commit { oid } }
      }
      comments(last: 100) {
        nodes {
          author { login }
          body
          updatedAt
          reactions(last: 100, content: THUMBS_UP) {
            nodes { createdAt user { login } }
          }
        }
      }
      reviewThreads(first: 100, after: $cursor) {
        pageInfo { hasNextPage endCursor }
        nodes {
          id isResolved isOutdated path line originalLine
          comments(first: 1) { nodes { author { login } body } }
        }
      }
    }
  }
}
"""


def graphql(owner: str, repo: str, number: int, cursor: str | None) -> dict[str, Any]:
    command = [
        "gh", "api", "graphql", "-F", "query=@-", "-F", f"owner={owner}",
        "-F", f"repo={repo}", "-F", f"number={number}",
    ]
    if cursor:
        command.extend(["-F", f"cursor={cursor}"])
    result = subprocess.run(command, input=QUERY, text=True, capture_output=True)
    if result.returncode != 0:
        raise RuntimeError(result.stderr.strip() or "GitHub GraphQL request failed")
    payload = json.loads(result.stdout)
    if payload.get("errors"):
        raise RuntimeError(json.dumps(payload["errors"], sort_keys=True))
    return payload


def trusted_codex(login: str | None) -> bool:
    """Return whether login is an explicitly trusted Codex reviewer identity."""
    return login in TRUSTED_CODEX_LOGINS


def exact_head_reaction_time(comment: dict[str, Any], head: str) -> str | None:
    """Return the latest trusted Codex reaction time for an exact-head request."""
    author = (comment.get("author") or {}).get("login")
    if author not in TRUSTED_REVIEW_REQUEST_LOGINS:
        return None
    lines = {line.strip() for line in (comment.get("body") or "").splitlines()}
    request_prefix = f"Please review exact head `{head}`."
    if "@codex review" not in lines or not any(
        line.startswith(request_prefix) for line in lines
    ):
        return None
    updated_at = comment.get("updatedAt")
    if not isinstance(updated_at, str):
        return None
    times = [
        reaction["createdAt"]
        for reaction in (comment.get("reactions") or {}).get("nodes") or []
        if trusted_codex((reaction.get("user") or {}).get("login"))
        and isinstance(reaction.get("createdAt"), str)
        and reaction["createdAt"] > updated_at
    ]
    return max(times, default=None)


def exact_head_reaction(comment: dict[str, Any], head: str) -> bool:
    """Return whether trusted Codex approved a trusted exact-head request."""
    return exact_head_reaction_time(comment, head) is not None


def codex_comment_verdict_time(comment: dict[str, Any], head: str) -> str | None:
    """Return the time of a trusted Codex clean comment verdict naming head."""
    if not trusted_codex((comment.get("author") or {}).get("login")):
        return None
    body = comment.get("body") or ""
    if CODEX_CLEAN_VERDICT_MARKER not in body:
        return None
    match = CODEX_REVIEWED_COMMIT_PATTERN.search(body)
    if match is None or not head.startswith(match.group(1)):
        return None
    updated_at = comment.get("updatedAt")
    return updated_at if isinstance(updated_at, str) else None


def exact_head_review(review: dict[str, Any], head: str) -> bool:
    """Return whether trusted Codex submitted a live review for head."""
    return (
        trusted_codex((review.get("author") or {}).get("login"))
        and review.get("state") in ACCEPTED_CODEX_REVIEW_STATES
        and (review.get("commit") or {}).get("oid") == head
    )


def latest_codex_head_review(
    reviews: list[dict[str, Any]], head: str
) -> dict[str, Any] | None:
    """Return the latest trusted Codex review attached to head."""
    candidates = [
        review
        for review in reviews
        if trusted_codex((review.get("author") or {}).get("login"))
        and (review.get("commit") or {}).get("oid") == head
        and isinstance(review.get("submittedAt"), str)
    ]
    return max(candidates, key=lambda review: review["submittedAt"], default=None)


def codex_evidence_complete(
    reviews: list[dict[str, Any]], comments: list[dict[str, Any]], head: str
) -> bool:
    """Return whether the newest exact-head Codex evidence is non-blocking."""
    latest_review = latest_codex_head_review(reviews, head)
    evidence_times = [
        evidence_time
        for comment in comments
        for evidence_time in (
            exact_head_reaction_time(comment, head),
            codex_comment_verdict_time(comment, head),
        )
        if evidence_time is not None
    ]
    latest_evidence = max(evidence_times, default=None)
    if latest_review is None:
        return latest_evidence is not None
    if latest_evidence is not None and latest_evidence > latest_review["submittedAt"]:
        return True
    return exact_head_review(latest_review, head)


def audit(repository: str, number: int) -> dict[str, Any]:
    owner, repo = repository.split("/", 1)
    cursor: str | None = None
    unresolved: list[dict[str, Any]] = []
    metadata: dict[str, Any] | None = None
    while True:
        payload = graphql(owner, repo, number, cursor)
        pr = payload["data"]["repository"]["pullRequest"]
        if pr is None:
            raise RuntimeError(f"pull request #{number} was not found")
        if metadata is None:
            metadata = pr
        threads = pr["reviewThreads"]
        for thread in threads["nodes"]:
            if thread["isResolved"]:
                continue
            first = (thread["comments"]["nodes"] or [{}])[0]
            unresolved.append({
                "id": thread["id"],
                "outdated": thread["isOutdated"],
                "path": thread["path"],
                "line": thread["line"] or thread["originalLine"],
                "author": (first.get("author") or {}).get("login"),
                "summary": (first.get("body") or "").splitlines()[0][:160],
            })
        if not threads["pageInfo"]["hasNextPage"]:
            break
        cursor = threads["pageInfo"]["endCursor"]

    assert metadata is not None
    head = metadata["headRefOid"]
    review_complete = codex_evidence_complete(
        metadata["reviews"]["nodes"], metadata["comments"]["nodes"], head
    )
    return {
        "repository": repository,
        "pull_request": number,
        "head": head,
        "codex_review_complete": review_complete,
        "unresolved_count": len(unresolved),
        "unresolved_threads": unresolved,
    }


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("repository", help="OWNER/REPOSITORY")
    parser.add_argument("pull_request", type=int)
    parser.add_argument("--require-codex", action="store_true")
    args = parser.parse_args()
    try:
        result = audit(args.repository, args.pull_request)
    except (RuntimeError, ValueError, KeyError, json.JSONDecodeError) as error:
        print(f"review audit failed: {error}", file=sys.stderr)
        return 2
    print(json.dumps(result, indent=2, sort_keys=True))
    if result["unresolved_count"]:
        return 1
    if args.require_codex and not result["codex_review_complete"]:
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
