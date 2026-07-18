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

Install Go 1.25.12 or a later compatible Go 1.25 patch, then run:

```bash
go version
go test -race ./...
go vet ./...
```

### 4a. Run the short security fuzz check

This optional contributor check feeds synthetic, malformed bytes into the
bounded plaintext framer, protobuf decoder, and mDNS parser. It needs no
network, key, or hardware and normally finishes in about fifteen seconds.

```bash
go test ./internal/wire -run=^$ -fuzz=FuzzPlainFramerRead -fuzztime=5s
go test ./internal/wire -run=^$ -fuzz=FuzzDecode -fuzztime=5s
go test ./internal/mdns -run=^$ -fuzz=FuzzAnswerIP -fuzztime=5s
```

Each command should end with `PASS`. A crash, panic, excessive allocation, or
unexpected decoded frame is a security bug; keep the generated fuzz input
private until it is reviewed for sensitive data, then follow `SECURITY.md`.

### 4b. Check for reachable vulnerabilities

This contributor check runs the official Go vulnerability scanner at the exact
version used by CI. The first run downloads the pinned tool through the Go
module checksum system; it does not add anything to this library's `go.mod`.

```bash
./tools/run-govulncheck.sh
```

A clean run ends with `No vulnerabilities found.` A reachable finding fails the
command and must be fixed before merge. Required-but-unreachable module findings
still need maintainer triage under the [dependency policy](docs/dependency-policy.md).

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

To build it as a standalone executable, or to create a tiny external Go module
that imports this library without MGMT, use the [standalone simulator demo](docs/standalone-simulator-demo.md).

### 6. See what is actually supported

```bash
sed -n '1,220p' docs/support-matrix.md
```

`none` and `untracked` are honest current results, not setup failures.

### 6a. Check the protocol compatibility map

This no-network check proves the generated message-by-message map matches its
reviewed annotations and pinned ESPHome protocol. It needs the same supported Go
toolchain as the build step.

```bash
go run ./cmd/protocol-inventory -summary
go run ./cmd/protocol-inventory -check protocol/inventory.json
```

Expected output:

```text
ESPHome 2026.7.0: 148 unique messages, 33 M1 accounted (31 implemented), 117 generated-only
protocol inventory is current: 148 unique messages
```

See the [protocol inventory guide](docs/protocol-inventory.md) before changing
an evidence level or synchronizing a new ESPHome release.

### 7. Prove the real MGMT integration without hardware

This maintainer check runs the unchanged conveyor MCL against the encrypted
simulator in an isolated Linux network namespace. It does not change
`/etc/hosts`, open a host-network port, flash firmware, or control hardware.
The reviewed `esphome-conveyor.local` name is resolved through the library's
built-in multicast DNS (mDNS) path inside that namespace.
You need Linux user namespaces, `ip`, `timeout`, a built MGMT candidate,
and both repositories next to each other.

For a friendlier walkthrough with the MGMT build command, expected behavior,
and troubleshooting, see [the MGMT simulator demo](docs/mgmt-simulator-demo.md).

```bash
./tools/test-mgmt-conveyor.sh ../mgmt /tmp/mgmt
```

Expected output:

```text
MGMT securely converged the reviewed conveyor MCL against the loopback simulator
```

### 8. Prove both original MGMT examples without hardware

This maintainer check verifies the reviewed hashes, then runs `esphome0.mcl`
and `esphome-blink.mcl` byte-for-byte through real MGMT processes and encrypted
simulators. It uses private Linux user and network namespaces. The
hardcoded documentation address in `esphome0.mcl` is reachable only through a
TCP forwarder confined to that private network namespace.
`esphome-blink.local` is answered by a real mDNS responder; the script does not
add it to `/etc/hosts`.

Use the same prerequisites as the conveyor acceptance command:

```bash
./tools/test-mgmt-baselines.sh ../mgmt /tmp/mgmt
```

Expected output:

```text
MGMT securely converged both reviewed baseline MCL examples against dedicated simulators
```

### 8a. Run an explicitly authorized hardware blink demo

After the simulator passes, maintainers with a pre-provisioned blink device can
follow the [hardware blink walkthrough](docs/mgmt-hardware-blink.md). It keeps
the MCL unchanged, does not flash firmware, and explains how to avoid retaining
private identifiers from ESPHome's verbose device logs.

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
go get github.com/flavio-fernandes/go-aioesphomeapi@f1f9e3ef9b5efca161aa97cbe0040d278fdb4038
```

MGMT `feat/esphome` at `ede1737219be106e2c5e06bb497af9a1ec9e17c8`
pins this commit. Review [library PR #48](https://github.com/flavio-fernandes/go-aioesphomeapi/pull/48)
for the current dependency security floor, [PR #46](https://github.com/flavio-fernandes/go-aioesphomeapi/pull/46)
for the mDNS retry correction, and [PR #30](https://github.com/flavio-fernandes/go-aioesphomeapi/pull/30)
for the original client implementation. A tagged release command will replace
this development pin later.

To inspect the exact MGMT revision, unchanged MCL hashes, dependency reduction, and verification record:

```bash
python3 -m json.tool compatibility/mgmt-feat-esphome-security.json
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

**A device connection fails:** the returned error names the attempted target and
the failed stage. Standard `errors.Is` and `errors.As` still reach causes such
as `*net.OpError`, `ErrNameResolution`, `ErrNoiseHandshake`, and `ErrHello`.
Never paste a production error into an issue until you have removed private
hostnames and addresses.

An ESPHome peer that explicitly rejects a key returns both the broad
`ErrNoiseHandshake` category and the more actionable `ErrNoiseKeyRejected`
category. Its untrusted reason text is printable and length-limited; key
material is never included.

**An established connection closes:** wait for `client.Done()`, then inspect
`client.CloseReason()`. A deliberate `client.Close()` leaves the reason nil;
network, protocol, peer-disconnect, context, and queue failures record a cause.

**Check an established connection now:** call `client.Ping(ctx)` with your own
short context deadline. The probe is serialized and returns `ErrPing` while
preserving cancellation, connection, or protocol causes. A sent probe that
times out closes the ambiguous connection so a late reply cannot satisfy a
later probe. Automatic periodic keepalive remains application policy for now.

**A fuzz command cannot start:** confirm `go version` reports Go 1.25.12 or a
later compatible Go 1.25 patch and that the repository dependencies have been
downloaded by `go test ./...`.

**A custom simulator scenario is rejected:** call `scenario.Validate()` before
`simulator.New(scenario)`. The same typed error is returned from `DialContext`
or `Serve` when preflight is skipped. Use `errors.Is(err,
simulator.ErrInvalidScenario)` and `errors.As` to `*simulator.ValidationError`.
The error reports a safe field/index/code, not entity data. A zero seed is valid
unless the scenario declares a randomized action.

**Drive a custom scenario without waiting in real time:** use one manual clock
for the device and advance it only when your test is ready:

```go
clock := simulator.NewManualClock()
device := simulator.New(simulator.Scenario{
    Name: "friendly-demo",
    InitialStates: []proto.Message{
        &pb.SwitchStateResponse{Key: 1, State: false},
    },
    StateTimeline: []simulator.StateEvent{
        {At: time.Second, State: &pb.SwitchStateResponse{Key: 1, State: true}},
    },
}, simulator.WithManualClock(clock))
defer device.Close()

// Subscribe with the normal client first. This applies and pushes the 1s
// event immediately; the test does not sleep for a second.
if err := clock.Advance(time.Second); err != nil {
    panic(err)
}
```

`device.DropConnections()` intentionally breaks current sessions while keeping
the device and its latest state alive. It is the friendly way to test
application-owned reconnect behavior. A reconnect gets one current snapshot;
old timeline events and old commands are not replayed.
