"""Tests for the merge review identity gate."""

import unittest

import audit_review_threads


class TrustedCodexIdentityTest(unittest.TestCase):
    def test_accepts_the_configured_connector_identity(self) -> None:
        self.assertTrue(
            audit_review_threads.trusted_codex("chatgpt-codex-connector")
        )

    def test_rejects_lookalike_and_missing_identities(self) -> None:
        for login in (
            None,
            "codex",
            "codex-fan",
            "chatgpt-codex-connector-fan",
            "chatgpt-codex-connector[bot]",
        ):
            with self.subTest(login=login):
                self.assertFalse(audit_review_threads.trusted_codex(login))


class ExactHeadReactionTest(unittest.TestCase):
    head = "0123456789abcdef0123456789abcdef01234567"

    def comment(
        self,
        *,
        author: str = "flavio-fernandes",
        requested_head: str | None = None,
        reactor: str = "chatgpt-codex-connector",
        reaction_created_at: str = "2026-07-18T22:18:06Z",
        trailing_context: str = "",
    ) -> dict[str, object]:
        requested_head = requested_head or self.head
        return {
            "author": {"login": author},
            "body": (
                f"@codex review\n\nPlease review exact head `{requested_head}`."
                f"{trailing_context}"
            ),
            "updatedAt": "2026-07-18T22:18:05Z",
            "reactions": {
                "nodes": [
                    {
                        "createdAt": reaction_created_at,
                        "user": {"login": reactor},
                    }
                ]
            },
        }

    def test_accepts_trusted_reaction_bound_to_current_head(self) -> None:
        self.assertTrue(
            audit_review_threads.exact_head_reaction(self.comment(), self.head)
        )

    def test_accepts_trailing_context_after_exact_head_sentence(self) -> None:
        self.assertTrue(
            audit_review_threads.exact_head_reaction(
                self.comment(trailing_context=" The prior finding is fixed."),
                self.head,
            )
        )

    def test_rejects_reaction_bound_to_old_head(self) -> None:
        self.assertFalse(
            audit_review_threads.exact_head_reaction(
                self.comment(requested_head="f" * 40), self.head
            )
        )

    def test_rejects_untrusted_requestor_or_reactor(self) -> None:
        self.assertFalse(
            audit_review_threads.exact_head_reaction(
                self.comment(author="codex-fan"), self.head
            )
        )
        self.assertFalse(
            audit_review_threads.exact_head_reaction(
                self.comment(reactor="codex-fan"), self.head
            )
        )

    def test_rejects_reaction_that_predates_request_edit(self) -> None:
        self.assertFalse(
            audit_review_threads.exact_head_reaction(
                self.comment(reaction_created_at="2026-07-18T22:18:04Z"),
                self.head,
            )
        )

    def test_rejects_reaction_at_same_timestamp_as_request_edit(self) -> None:
        self.assertFalse(
            audit_review_threads.exact_head_reaction(
                self.comment(reaction_created_at="2026-07-18T22:18:05Z"),
                self.head,
            )
        )


class ExactHeadReviewTest(unittest.TestCase):
    head = "0123456789abcdef0123456789abcdef01234567"

    def review(
        self,
        *,
        author: str = "chatgpt-codex-connector",
        state: str = "COMMENTED",
        commit: str | None = None,
    ) -> dict[str, object]:
        return {
            "author": {"login": author},
            "state": state,
            "commit": {"oid": commit or self.head},
        }

    def test_accepts_live_trusted_exact_head_review(self) -> None:
        self.assertTrue(
            audit_review_threads.exact_head_review(self.review(), self.head)
        )

    def test_rejects_dismissed_old_head_and_untrusted_reviews(self) -> None:
        for review in (
            self.review(state="DISMISSED"),
            self.review(commit="f" * 40),
            self.review(author="codex-fan"),
        ):
            with self.subTest(review=review):
                self.assertFalse(
                    audit_review_threads.exact_head_review(review, self.head)
                )


if __name__ == "__main__":
    unittest.main()
