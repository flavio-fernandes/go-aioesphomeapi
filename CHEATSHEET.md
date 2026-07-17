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

### 4a. Run the short security fuzz check

This optional contributor check feeds synthetic, malformed bytes into the
bounded plaintext framer and protobuf decoder. It needs no network, key, or
hardware and normally finishes in about ten seconds.

```bash
go test ./internal/wire -run=^$ -fuzz=FuzzPlainFramerRead -fuzztime=5s
go test ./internal/wire -run=^$ -fuzz=FuzzDecode -fuzztime=5s
```

Each command should end with `PASS`. A crash, panic, excessive allocation, or
unexpected decoded frame is a security bug; keep the generated fuzz input
private until it is reviewed for sensitive data, then follow `SECURITY.md`.

### 5. Run the safe first example

This uses a real Noise handshake over an in-process connection. It opens no port, needs no hardware, and contains only a public test key.

```bash
go run ./cmd/conveyor-sim
```

Expected output:

```text
connected securely to conveyor-simulator; discovered 13 entities
simulated conveyor speed=35 and status color=#00ff00
```

### 6. See what is actually supported

```bash
sed -n '1,220p' docs/support-matrix.md
```

`none` and `untracked` are honest current results, not setup failures.

### 7. Prove the real MGMT integration without hardware

This maintainer check runs the unchanged conveyor MCL against the encrypted
simulator in an isolated Linux network namespace. It does not change
`/etc/hosts`, open a host-network port, flash firmware, or control hardware.
You need Linux user namespaces, `ip`, `mount`, `timeout`, a built MGMT candidate,
and both repositories next to each other.

```bash
./tools/test-mgmt-conveyor.sh ../mgmt /tmp/mgmt
```

Expected output:

```text
MGMT securely converged the unchanged conveyor MCL against the loopback simulator
```

### 8. Prove both original MGMT examples without hardware

This maintainer check verifies the reviewed hashes, then runs `esphome0.mcl`
and `esphome-blink.mcl` byte-for-byte through real MGMT processes and encrypted
simulators. It uses private Linux user, mount, and network namespaces. The
hardcoded documentation address in `esphome0.mcl` is reachable only through a
TCP forwarder confined to that private network namespace.

Use the same prerequisites as the conveyor acceptance command:

```bash
./tools/test-mgmt-baselines.sh ../mgmt /tmp/mgmt
```

Expected output:

```text
MGMT securely converged both unchanged baseline MCL examples against dedicated simulators
```

### 9. Read the plan in a terminal

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
go get github.com/flavio-fernandes/go-aioesphomeapi@ef8386820978611d313f976e68bd2aaf9009e8b8
```

That commit is the current draft Milestone 1 candidate used by the latest MGMT replacement tests. Review [library PR #29](https://github.com/flavio-fernandes/go-aioesphomeapi/pull/29) before adopting it; a tagged release command will replace this development pin after merge.

To inspect the exact MGMT revision, unchanged MCL hashes, dependency reduction, and verification record:

```bash
python3 -m json.tool compatibility/mgmt-feat-esphome2.json
```

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

**A fuzz command cannot start:** confirm `go version` reports Go 1.25.10 or a
later compatible Go 1.25 patch and that the repository dependencies have been
downloaded by `go test ./...`.
