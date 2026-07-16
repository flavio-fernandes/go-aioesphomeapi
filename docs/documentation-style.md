# Approachable documentation contract

Approachability is a product requirement. A technically correct library that newcomers cannot install, test, or understand is not complete.

## Reader promise

Documentation must help a reader answer, in order:

1. What works today?
2. What do I need before I start?
3. What is the smallest safe command I can run?
4. What result should I expect?
5. What should I try when it fails?
6. Where do I go next?

Do not make readers reconstruct these answers from architecture documents or issue history.

## Writing rules

- Lead with the useful outcome and current project status.
- Prefer short sentences and ordinary words. Define an acronym on first use.
- Explain why a security or compatibility restriction exists without scolding the reader.
- Separate beginner, contributor, maintainer, simulator, and physical-workbench paths.
- State what is unsupported as clearly as what is supported.
- Use consistent names from the public API and support matrix.
- Link to deeper design material after the simple path, not before it.

## Command-block rules

Every command presented as runnable must:

- work when pasted in order from the stated directory;
- list its prerequisites and supported environment;
- avoid hidden shell aliases, private files, local absolute paths, and prior unmentioned state;
- use a pinned or supported version where version choice matters;
- use synthetic values and never invite a real secret into shell history unnecessarily;
- default to the simulator when an operation can affect a device;
- state the expected success signal and one useful failure remedy.

Place future or illustrative commands outside runnable code blocks and label them unavailable. Placeholder commands must never look like a supported workflow.

## Required entry points

- `README.md` explains the project and points newcomers to the next document.
- `CHEATSHEET.md` owns clone, validate, install, build, first-use, and common contributor commands.
- `docs/support-matrix.md` owns compatibility truth.
- `CONTRIBUTING.md` owns contribution expectations.
- `SECURITY.md` owns safe reporting.

Avoid duplicating long command sequences elsewhere. Link to the cheatsheet so one reviewed source stays correct.

## Definition of done for user-facing work

A change is not complete until:

- the cheatsheet is updated or the PR explains why no command changed;
- examples compile or run under the same checks users are told to run;
- a clean-clone test covers the primary beginner path when implementation exists;
- error messages identify the failed operation and a safe next action without leaking sensitive data;
- new support claims match the evidence level in the support matrix;
- a reviewer can follow the documented path without project-specific tribal knowledge.

Review friendliness with the same seriousness as API compatibility, security, and test coverage.
