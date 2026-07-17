# go-aioesphomeapi cheatsheet

Short, safe, copy/paste commands for cloning, checking, contributing to, and—once the first usable release exists—installing and using this library.

> [!IMPORTANT]
> **Current phase: compatibility architecture only.** There is no usable Go client to install or build yet. Commands under **Works today** are verified repository commands. Milestone 1 will make the pinned MGMT examples run through this module before broader features are claimed.

## Works today

### 1. Clone with GitHub CLI

You need Git and [GitHub CLI](https://cli.github.com/). The repository is public, so authentication is optional for cloning but required to open pull requests.

```bash
gh auth status
gh repo clone flavio-fernandes/go-aioesphomeapi
cd go-aioesphomeapi
```

### 2. Clone with Git

Use this if Git authentication is already configured.

```bash
git clone https://github.com/flavio-fernandes/go-aioesphomeapi.git
cd go-aioesphomeapi
```

### 3. Validate the repository

Run this from the repository root. It checks required documents, skill metadata, private-path patterns, likely secrets, and immutable GitHub Action pins.

```bash
./tools/validate-repo.sh
```

Expected final line:

```text
repository policy validation passed
```

### 4. See what is actually supported

```bash
sed -n '1,220p' docs/support-matrix.md
```

`none` and `untracked` are honest current results, not setup failures.

### 5. Read the plan in a terminal

```bash
sed -n '1,240p' docs/roadmap.md
```

The [GitHub roadmap board](https://github.com/users/flavio-fernandes/projects/1) is the live task view.

## Make a small documentation contribution

Start from a clean, current `main` branch:

```bash
git switch main
git pull --ff-only
git switch -c docs/describe-your-change
```

After editing, inspect and validate only your intended change:

```bash
./tools/validate-repo.sh
git diff --check
git status --short
git diff
```

Commit and open a draft pull request:

```bash
git add CHEATSHEET.md README.md docs
git commit -m "docs: describe the change"
git push -u origin docs/describe-your-change
gh pr create --draft --fill
```

Adjust the explicit `git add` paths to match your change. Do not use `git add .` when unrelated files are present.

## Install, build, and use: command contract

These operations are **not available today**. Do not invent package paths or copy commands from another ESPHome client.

Milestone 1 must update this file with verified, copy/paste commands for all four workflows:

1. Install a tagged module version with `go get`.
2. Build and test from a clean clone with the supported Go version.
3. Run a first client example against the in-process simulator—no hardware or secret required.
4. Run the same example against an explicitly selected ESPHome device using a runtime-provided Noise key.

Each command must be exercised in CI or a documented clean environment before it appears in a runnable code block here. The simulator will always be the beginner default; real hardware will always require an explicit target.

## Safe command rules

- Never put a real Noise key, SSID, IP address, device identifier, username, or local path in a command committed to this repository.
- Never make flashing, camera access, motor power, or actuator movement part of a beginner quickstart.
- Use synthetic examples and test-only credentials clearly labeled as test data.
- Check [SECURITY.md](SECURITY.md) before sharing logs or reporting unexpected behavior.

## Quick troubleshooting

**GitHub CLI authentication fails:** cloning still works with the plain `git clone` command above. Run `gh auth login` only when you need authenticated contribution commands.

**The validator is not executable:** run it explicitly with Bash:

```bash
bash ./tools/validate-repo.sh
```

**A command here fails from a clean clone:** open a documentation issue. A broken cheatsheet command is a product bug.
