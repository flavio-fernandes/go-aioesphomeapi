---
name: prepare-public-release
description: Audit go-aioesphomeapi before a public GitHub visibility change or release. Use for privacy/history review, licensing and provenance, security settings, support claims, reproducibility, release notes, tags, or published artifacts.
---

# Prepare Public Release

Fail closed: this workflow reports findings and does not change visibility or publish until a maintainer approves the completed evidence.

## Workflow

1. Read Milestone 6 in `docs/roadmap.md` and `references/release-checklist.md`.
2. Freeze the candidate commit and inspect the entire reachable Git history, not only the working tree.
3. Run repository validation, secret scanning, personal/private-data scanning, dependency/license review, generated provenance verification, tests, race tests, fuzz smoke, compatibility suites, and reproducible build/generation checks.
   Use `./tools/run-govulncheck.sh` for the pinned source scan and inspect open
   Dependabot alerts separately. An unreachable call path does not clear a
   module-level alert when a compatible patched release exists.
   For credential-redaction tests, cover every representation accepted by the
   decoder, including raw bytes, canonical base64, and CR/LF-wrapped base64;
   verify the public error contains neither the complete credential nor its
   wrapped printable fragments.
4. Compare README and release notes against `docs/support-matrix.md`. Remove or qualify every claim without matching evidence.
5. Verify repository rulesets, private vulnerability reporting, least-privilege workflows, immutable action pins, branch deletion policy, and dependency update policy.
6. Build artifacts from the candidate in a clean environment. Produce checksums and provenance without embedding usernames, paths, hosts, or timestamps that prevent reproducibility.
7. Have a maintainer review the complete history scan, license inventory, security findings, compatibility report, artifacts, and limitations.
8. Only after explicit approval, change visibility or publish the signed/tagged release through the repository's documented release workflow.

## Stop conditions

Stop for any suspected secret or personal data, unknown-origin file, mutable build input, unresolved vulnerability, failing test, non-reproducible generation, or support claim above its evidence level. Never rewrite shared history automatically; present the remediation plan for approval.
