# go-aioesphomeapi cheatsheet

Short, safe, copy/paste commands for cloning, checking, building, and using this library.

> [!IMPORTANT]
> **Current phase: usable development branch; no tagged release yet.** The simulator workflow below is safe and verified. Do not use `go get ...@latest` until the first release is published.

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

### 4. Build and test

Install Go 1.25.10 or a later compatible Go 1.25 patch, then run:

```bash
go version
go test -race ./...
go vet ./...
```

### 5. Run the safe first example

This uses a real Noise handshake over an in-process connection. It opens no port, needs no hardware, and contains only a public test key.

```bash
go run ./cmd/conveyor-sim
```

Expected output:

```text
connected securely to conveyor-simulator; discovered 12 entities
simulated conveyor speed=35 and status color=#00ff00
```

### 6. See what is actually supported

```bash
sed -n '1,220p' docs/support-matrix.md
```

`none` and `untracked` are honest current results, not setup failures.

### 7. Read the plan in a terminal

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

## Use from another Go module

Until a release is tagged, pin an exact reviewed commit rather than a moving branch:

```bash
go get github.com/flavio-fernandes/go-aioesphomeapi@COMMIT_SHA
```

Replace `COMMIT_SHA` with the reviewed commit from the pull request. A release command will replace this placeholder after tagging.

Real-device access is deliberately not a beginner copy/paste command. Applications must provide the target and base64 Noise key at runtime, keep both out of source and shell history, and call `WithEncryptionKey`. Plaintext requires `WithInsecurePlaintext()` and is for isolated tests only.

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
