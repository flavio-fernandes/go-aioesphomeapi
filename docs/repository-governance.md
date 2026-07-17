# Repository governance

## Current visibility

The repository is public. Public visibility does not mean implementation or production readiness; the support matrix controls those claims. Every change after bootstrap uses a branch and pull request.

Because the visibility gate has already occurred, history-wide privacy, secret, license, and provenance checks run now and again before releases. Never rewrite shared history automatically.

## Required GitHub controls

The public default branch must require a pull request, at least one approval, CODEOWNERS review where applicable, stale-approval dismissal, conversation resolution, passing validation, no force push, and no deletion. Enable secret scanning/push protection, private vulnerability reporting, and Dependabot security updates where supported.

Automatic deletion of merged branches and linear history are preferred. Administrator bypass is limited to documented emergencies.

## Pull request flow

Use short-lived branches named for issue intent. Generated protocol changes get their own reviewable diff. Cross-repository MGMT changes identify exact revisions and include the adapter/MCL diff. Releases come only from reviewed tags.

## Decision records

An ADR is required for public API compatibility, MGMT behavior changes, layer movement, security-default changes, runtime dependencies, Go-version changes, protocol-source changes, or simulator fidelity changes.

## Dependencies

The standard library is preferred. Follow [dependency policy](dependency-policy.md). Cryptography uses an accepted implementation; custom cryptographic primitives are prohibited.
