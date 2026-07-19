# MGMT blink loop demo

This guide runs MGMT's unchanged `examples/lang/esphome-blink.mcl` against a
simulated ESPHome blink device from this repository. The simulator reproduces
the firmware automations documented inside that MCL file, so MGMT cannot tell
it is not talking to real hardware: the device turns its LED back on three
seconds after it goes off, and MGMT turns the LED off as soon as it sees it
on. The result is the endless cooperative blink loop from the real
[hardware walkthrough](mgmt-hardware-blink.md), with no hardware, no firmware
flashing, no host-network service, and no real ESPHome key.

This is different from the [MGMT simulator demo](mgmt-simulator-demo.md) and
the baseline acceptance script, which prove one corrective convergence and
then exit. Here the device keeps relighting the LED, so the loop runs until
you stop it.

## What you need

Run these commands on Linux from the `go-aioesphomeapi` repository root.

Required tools:

- the Go version requested by the MGMT checkout; the current `feat/esphome`
  branch and the pinned CI compatibility lane use Go 1.25.12
- `git`
- `ip`
- `sha256sum`
- `timeout`
- `unshare`

Required checkout layout for the commands below:

```text
mgmt-dev/
  go-aioesphomeapi/
  mgmt-esphome2/
```

If your MGMT checkout is named `mgmt` instead, replace `../mgmt-esphome2` with
`../mgmt` in the commands.

## 1. Build the MGMT demo binary

This is the same binary the [MGMT simulator demo](mgmt-simulator-demo.md)
builds; skip this step if you already have it. From the `go-aioesphomeapi`
repository root:

```bash
cd ../mgmt-esphome2
go version
go build -ldflags '-X main.program=mgmt -X main.version=esphome2-sim-demo' -o /tmp/mgmt-esphome2-sim-demo .
cd ../go-aioesphomeapi
```

Expected check:

```bash
/tmp/mgmt-esphome2-sim-demo --version
```

Expected output:

```text
esphome2-sim-demo
```

## 2. Watch the endless blink loop

```bash
./tools/demo-mgmt-blink.sh ../mgmt-esphome2 /tmp/mgmt-esphome2-sim-demo
```

The script verifies that `esphome-blink.mcl` matches the reviewed
compatibility contract, starts the simulated device inside a private network
namespace, and then runs MGMT's MCL file as-is. A small mDNS responder inside
that namespace answers `esphome-blink.local`, exactly like the device would.

After MGMT connects, its log settles into a repeating cycle. Simulator lines
are prefixed with `[simulator]`; everything else is MGMT's own output. One
cycle looks like this, with your own timestamps:

```text
[simulator] received switch command: key=212 state=false
15:19:12 engine: esphome:endpoint[esphome-blink]: device log [info]: LED turned off; turning it back on in three seconds
15:19:12 engine: print[led state]: Msg: on-board led is on: false
[simulator] simulated firmware relit the LED
15:19:15 engine: esphome:endpoint[esphome-blink]: device log [info]: LED is still off; turning it on
15:19:15 engine: print[led state]: Msg: on-board led is on: true
15:19:15 engine: esphome:switch[Onboard LED]: turning off
```

Reading the cycle: MGMT commanded the LED off, the device streamed its own
log lines through MGMT's log subscription, three seconds later the simulated
firmware relit the LED, MGMT saw the pushed state and turned it off again.

Press Ctrl-C to stop. MGMT shuts down gracefully and the script prints:

```text
blink demo stopped
```

## 3. Run a bounded, self-verifying pass

Append a cycle count to stop automatically and check the evidence:

```bash
./tools/demo-mgmt-blink.sh ../mgmt-esphome2 /tmp/mgmt-esphome2-sim-demo 3
```

Expected final line:

```text
MGMT blinked the unchanged blink MCL for 3 cycles against the loopback simulator
```

That line means the script observed at least three complete blink cycles and
verified the evidence in the logs: MGMT saw the LED both on and off, sent the
corrective switch commands, and received the device's "turning it back on"
and "turning it on" firmware logs. The connection itself used the encrypted
Native API path after resolving `esphome-blink.local` through multicast DNS —
the private namespace offers no other listener and no other way to resolve
that name. The same bounded run executes in CI for every pull request through
[`mgmt-compat.yml`](../.github/workflows/mgmt-compat.yml).

## How the simulation works

The `blink-device` scenario of `cmd/mgmt-compat-sim-server` serves the same
entities as the one-shot `blink` acceptance scenario: the writable
`Onboard LED` switch and the read-only `Onboard LED State` binary sensor.
On top of that, it runs the automations that the firmware YAML embedded in
`esphome-blink.mcl` documents:

- every switch command is mirrored into the binary sensor, like the template
  sensor lambda;
- whenever the LED turns off, the device relights it three seconds later and
  pushes the new switch and sensor states, like the `on_turn_off` automation;
- each transition emits the firmware's log line through the native log stream.

MGMT and its MCL file are byte-for-byte unchanged; the script refuses to run
if the MCL hash differs from the reviewed contract.

## Troubleshooting

If the script says a command is missing, install the named Linux tool and
rerun the same command.

If the script says the MGMT binary is not executable, rebuild it with the
exact `go build` command above and verify `/tmp/mgmt-esphome2-sim-demo --version`.

If the script says the MCL hash differs, your MGMT checkout does not match
the reviewed demo branch. Switch back to the `feat/esphome` branch or inspect
the MCL change before trusting the result.

If `unshare` fails, your Linux environment may not allow unprivileged user or
network namespaces. The demo intentionally uses those namespaces so the
simulated device stays isolated from the host network.

## Safety notes

The key printed by the simulator is public test data. Do not replace it with a
real ESPHome Noise key in a shell command, log, issue, or documentation patch.

This demo is simulator evidence. It proves the MGMT integration path and the
cooperative blink behavior without hardware. It does not prove a physical
device, firmware pinout, or LED wiring; the
[hardware walkthrough](mgmt-hardware-blink.md) covers that.
