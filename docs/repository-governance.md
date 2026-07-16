# Repository governance

## Bootstrap and visibility

Create the repository privately. The initial reviewed architecture commit may bootstrap the default branch. After that commit, every change uses a branch and pull request. Public visibility is a Milestone 6 gated operation, not a side effect of repository creation.

## Required GitHub controls

Configure a ruleset for the default branch:

- pull request required;
- at least one approving review and conversation resolution;
- stale approvals dismissed when code changes;
- required status checks with branches up to date;
- force pushes and deletion blocked;
- linear history preferred;
- administrator bypass limited to documented emergencies.

Also enable private vulnerability reporting, secret scanning/push protection where the account supports them, Dependabot security updates after manifests exist, and automatic deletion of merged branches.

## Pull request flow

Use short-lived branches named for issue intent, such as `protocol/first-sync` or `simulator/fault-script`. Generated protocol updates get their own reviewable diff. Releases are created only from reviewed tags after the public-release gate.

## Decision records

Architecture decisions live in `docs/decisions`. An ADR is required for public API compatibility, layer movement, security-default changes, new third-party runtime dependencies, protocol-source changes, or simulator fidelity changes.

## Dependency policy

The standard library is preferred. Every runtime dependency needs a recorded purpose, license, maintenance/ownership assessment, known-vulnerability review, and explanation of why a small internal implementation is not safer. Cryptography must use established Go or audited libraries; do not create custom cryptographic primitives.
