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
Never commit a generated failure file as-is: after review, minimize it, make
sure it is fully synthetic, and only then add it deliberately under
`testdata/fuzz/` as regression corpus. CI never commits fuzz output.

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

### 4c. Report dependencies, checksums, and licenses

This contributor check prints the Go directive, every runtime module with its
detected license and `go.sum` checksums, the pinned tool-only helpers, and the
accepted dependency budget. It fails on any unexpected module, version drift,
or unrecognized license. It needs only the Go toolchain.

```bash
./tools/report-dependencies.sh
```

A clean run ends with `dependency report matches the accepted budget`.

### 4d. Prove the generated protocol files are current

This check regenerates the protocol wire types and inventory in a throwaway
copy and compares them with the checked-in files, so it never modifies your
checkout. It needs the pinned generators from `tools/generate-protocol.sh`
(protoc v31.1 and protoc-gen-go v1.36.11); CI runs it on every pull request.

```bash
./tools/check-generated-drift.sh
```

A clean run ends with `generated protocol files match their pinned inputs`.

### 4e. Run the extended fuzz lane on demand

The hosted `fuzz-extended` job runs every fuzz target for a finite ten minutes
inside the "Repository policy" workflow. It runs automatically on the weekly
schedule; a maintainer can also start it manually from `main` or any branch.
A pull-request comment cannot trigger it: GitHub only starts
`workflow_dispatch` runs from the Actions page ("Repository policy" → "Run
workflow"), the CLI, or the REST API.

```bash
gh workflow run policy.yml --repo flavio-fernandes/go-aioesphomeapi --ref main
gh run list --repo flavio-fernandes/go-aioesphomeapi --workflow=policy.yml --limit 3
```

The dispatched run executes the validate, Go, generated-drift, and
fuzz-extended jobs; each fuzz step must end with `PASS`. The same coverage runs
locally by raising `-fuzztime` in the section 4a commands to `10m`.

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
go get github.com/flavio-fernandes/go-aioesphomeapi@091b9af4f600dfa98b1ebea169265d2afc254047
```

MGMT `feat/esphome` at `08514da10969b0188a1127c3938790139e7fa0c6`
pins this commit and is published for upstream review in
[purpleidea/mgmt PR #961](https://github.com/purpleidea/mgmt/pull/961).
Review [library PR #48](https://github.com/flavio-fernandes/go-aioesphomeapi/pull/48)
for the current dependency security floor, [PR #46](https://github.com/flavio-fernandes/go-aioesphomeapi/pull/46)
for the mDNS retry correction, and [PR #30](https://github.com/flavio-fernandes/go-aioesphomeapi/pull/30)
for the original client implementation. A tagged release command will replace
this development pin later.

To inspect the exact MGMT revision, unchanged MCL hashes, dependency reduction, and verification record:

```bash
python3 -m json.tool compatibility/mgmt-upstream-pr-961-ready.json
```

Real-device access is deliberately not a beginner copy/paste command. Applications must provide the target and base64 Noise key at runtime, keep both out of source and shell history, and call `WithEncryptionKey`. Plaintext requires `WithInsecurePlaintext()` and is for isolated tests only.

### Temporarily test MGMT with newer library code

MGMT normally pins one exact reviewed `go-aioesphomeapi` commit. Use one of the following methods to test newer library code without accidentally treating a moving branch as the reviewed dependency.

#### Test a newer commit pushed to GitHub

Run these commands from the MGMT repository root:

```bash
module=github.com/flavio-fernandes/go-aioesphomeapi
commit=<FULL_OR_UNAMBIGUOUS_COMMIT_SHA>

go get "${module}@${commit}"
go mod tidy

go list -m -f '{{.Path}} {{.Version}}' "${module}"
```

Go resolves the commit to a pseudo-version and updates `go.mod` and `go.sum`. Do not manually construct the pseudo-version.

Run the focused MGMT checks:

```bash
go test -race ./util/esphome ./engine/resources ./lang/core/net/esphome/...
go vet ./util/esphome ./engine/resources ./lang/core/net/esphome/...
GOWORK=off make build
```

To return to the reviewed library pin, either run `go get` with the reviewed commit documented above, or discard only the dependency-file changes when no other edits to those files need to be kept:

```bash
git restore go.mod go.sum
```

Review `git diff` before discarding or committing dependency changes.

#### Test a local library checkout with `replace`

This is useful for testing uncommitted library changes. The replacement path must point to the directory containing the library's `go.mod`.

For sibling checkouts named `mgmt` and `go-aioesphomeapi`, run from the MGMT repository:

```bash
go mod edit \
  -replace=github.com/flavio-fernandes/go-aioesphomeapi=../go-aioesphomeapi

go list -m -f '{{.Path}} {{.Version}} => {{with .Replace}}{{.Dir}}{{end}}' \
  github.com/flavio-fernandes/go-aioesphomeapi
```

Build and test without `GOWORK=off`:

```bash
go test -race ./util/esphome ./engine/resources ./lang/core/net/esphome/...
go vet ./util/esphome ./engine/resources ./lang/core/net/esphome/...
make build
```

Remove the local replacement afterward:

```bash
go mod edit \
  -dropreplace=github.com/flavio-fernandes/go-aioesphomeapi
```

Do not commit a developer-specific local `replace` path to MGMT.

#### Test both repositories with a Go workspace

For ongoing development across both repositories, a workspace avoids changing MGMT's tracked `go.mod`.

From the parent directory containing both checkouts:

```bash
go work init ./mgmt ./go-aioesphomeapi
go work sync
```

Run local-library tests from the MGMT checkout without `GOWORK=off`:

```bash
cd mgmt

go test -race ./util/esphome ./engine/resources ./lang/core/net/esphome/...
go vet ./util/esphome ./engine/resources ./lang/core/net/esphome/...
make build
```

To verify the exact dependency pinned by MGMT rather than the workspace copy:

```bash
GOWORK=off go list -m -f '{{.Path}} {{.Version}}' \
  github.com/flavio-fernandes/go-aioesphomeapi

GOWORK=off make build
```

Remove the temporary workspace when it is no longer needed:

```bash
cd ..
rm -f go.work go.work.sum
```

Do not commit a developer-specific `go.work` or `go.work.sum` unless the repositories deliberately adopt a shared workspace.

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
unless the scenario declares a randomized action. Network delay validation
uses the same typed, secret-safe error path.

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

**Make the simulated network misbehave predictably:** add one named action at
the protocol point you want to exercise. This example fragments the first
server response after Hello into one-byte writes while still using the normal
encrypted client and framing path:

```go
device := simulator.New(simulator.Scenario{
    Name: "friendly-network-demo",
    Network: []simulator.NetworkFault{
        {
            Trigger: simulator.FaultAfterHello,
            Action:  simulator.NetworkFragmentFrame,
        },
    },
})
defer device.Close()

// Connect through device.ClientOptions(), then call ListEntities normally.
// NetworkCoalesceSegments combines that response's raw framing segments
// instead. Neither action changes the bytes seen by the client.
```

For a delayed response, use `NetworkDelayReply`, set a positive `Delay`, and
pass a `ManualClock` through `WithManualClock`. Start the client operation,
wait until `device.Stats().NetworkPendingDelays == 1`, then call
`clock.Advance(delay)`. No real-time sleep is required. Every action affects
only the next server response frame at its trigger; `Device.Close` or
`Device.DropConnections` releases a pending delay during cleanup. A delayed
timeline response does not block `clock.Advance`: the due state is committed
synchronously, while its complete wire frame waits in order for the later
virtual deadline.

**Check exact commands without sleeps or channel peeking:** declare the ordered
commands and counts before creating the device. After your client operation is
quiescent, wait with your own deadline:

```go
scenario := simulator.Scenario{
    Name: "friendly-command-demo",
    Commands: []simulator.CommandExpectation{
        {
            Command: &pb.SwitchCommandRequest{Key: 1, State: true},
            Count:   1,
        },
    },
}
device := simulator.New(scenario)
defer device.Close()

// Connect through device.ClientOptions(), then make the client send the
// command. Ping is a useful real-protocol barrier when the producer has
// finished and the test must reject trailing commands too.
if err := client.Ping(ctx); err != nil {
    panic(err)
}
if err := device.WaitForCommandExpectations(ctx); err != nil {
    panic(err)
}
```

Failures work with `errors.Is`: use `ErrCommandMissing`,
`ErrCommandUnexpected`, `ErrCommandOutOfOrder`, and `ErrCommandOverflow`.
Missing-command errors also retain `context.Canceled` or
`context.DeadlineExceeded`. Error text contains only counters and indexes, not
command payloads. The original `Commands()` stream remains available for
interactive exploration.

**Finish a simulator test with a clean desk:** the simulator has small,
documented limits so a broken test cannot create work forever. One device
allows up to 64 active/closing sessions and 8 loopback `Serve` calls. A custom
scenario allows 4,096 items per repeated field, 64 KiB per encoded protobuf
message, and 4 MiB of encoded protobuf data in total.

Close the client and device, then use one caller-owned deadline to prove every
simulator-owned task has stopped:

```go
_ = client.Close()
_ = device.Close()

ctx, cancel := context.WithTimeout(context.Background(), time.Second)
defer cancel()
if err := device.WaitForIdle(ctx); err != nil {
    panic(err)
}

stats := device.Stats()
fmt.Printf("connections=%d listeners=%d sessions=%d delays=%d\n",
    stats.ActiveConnections,
    stats.ActiveListeners,
    stats.ActiveSessionTasks,
    stats.NetworkPendingDelays,
)
```

Expected output:

```text
connections=0 listeners=0 sessions=0 delays=0
```

Limit failures are friendly typed errors: check `ErrConnectionLimit` and
`ErrListenerLimit` with `errors.Is`. If `WaitForIdle` reaches its deadline
while work remains, its error matches both `ErrSimulatorBusy` and the context
cause, such as `context.DeadlineExceeded`.

**Prove that a slow callback cannot grow memory:** tests can deliberately use a
one-item callback queue, hold the first callback with a channel, and advance a
manual-clock burst. The client closes instead of blocking the network reader or
silently dropping state:

```go
options := append(device.ClientOptions(), aioesphomeapi.WithCallbackQueueSize(1))
client, err := aioesphomeapi.DialWithContext(ctx, "synthetic:6053", time.Second, options...)
if err != nil {
    panic(err)
}

gate := make(chan struct{})
entered := make(chan struct{})
var enteredOnce sync.Once
_, err = client.SubscribeStates(func(proto.Message) {
    enteredOnce.Do(func() { close(entered) })
    <-gate
})
if err != nil {
    panic(err)
}

select {
case <-entered:
case <-ctx.Done():
    panic(ctx.Err())
}

// Advance a scenario containing two equal-time updates. One fills the queue;
// the next closes the session.
if err := clock.Advance(time.Second); err != nil {
    panic(err)
}
<-client.Done()
if !errors.Is(client.CloseReason(), aioesphomeapi.ErrEventQueueFull) {
    panic("expected bounded callback queue to close")
}

close(gate)
if err := client.WaitCallbacks(ctx); err != nil {
    panic(err)
}
```

`WaitCallbacks` never forces application code to return. It only provides a
context-bounded way to confirm that the serial dispatcher has stopped after the
application releases its own callback. This pattern is for tests; production
queue sizing should remain finite and callbacks should return promptly.
