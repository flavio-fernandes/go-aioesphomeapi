#!/usr/bin/env python3
"""Fail unless one pull request has a completed Codex review and no open threads."""

from __future__ import annotations

import argparse
import json
import subprocess
import sys
from typing import Any


TRUSTED_CODEX_LOGINS = frozenset({"chatgpt-codex-connector"})
TRUSTED_REVIEW_REQUEST_LOGINS = frozenset({"flavio-fernandes"})


QUERY = """
query($owner: String!, $repo: String!, $number: Int!, $cursor: String) {
  repository(owner: $owner, name: $repo) {
    pullRequest(number: $number) {
      headRefOid
      reviews(last: 100) {
        nodes { author { login } submittedAt commit { oid } }
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


def exact_head_reaction(comment: dict[str, Any], head: str) -> bool:
    """Return whether trusted Codex approved a trusted exact-head request."""
    author = (comment.get("author") or {}).get("login")
    if author not in TRUSTED_REVIEW_REQUEST_LOGINS:
        return False
    lines = {line.strip() for line in (comment.get("body") or "").splitlines()}
    if "@codex review" not in lines or f"Please review exact head `{head}`." not in lines:
        return False
    updated_at = comment.get("updatedAt")
    if not isinstance(updated_at, str):
        return False
    return any(
        trusted_codex((reaction.get("user") or {}).get("login"))
        and reaction.get("createdAt", "") >= updated_at
        for reaction in (comment.get("reactions") or {}).get("nodes") or []
    )


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
    reviewed = any(
        trusted_codex((review.get("author") or {}).get("login"))
        and (review.get("commit") or {}).get("oid") == head
        for review in metadata["reviews"]["nodes"]
    )
    reacted = any(
        exact_head_reaction(comment, head)
        for comment in metadata["comments"]["nodes"]
    )
    return {
        "repository": repository,
        "pull_request": number,
        "head": head,
        "codex_review_complete": reviewed or reacted,
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
