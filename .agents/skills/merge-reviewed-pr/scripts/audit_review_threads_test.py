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


if __name__ == "__main__":
    unittest.main()
