# Run the MGMT blink demo on an authorized device

This maintainer workflow builds the latest `feat/esphome` branch and runs its
unchanged blink MCL against a real ESPHome device. MGMT watches the device's
binary-sensor state and turns the onboard LED switch off each time the firmware
turns it on.

Use the [simulator walkthrough](mgmt-simulator-demo.md) first. Continue here
only when you are authorized to control the named device and you can identify
its onboard LED safely. This workflow does not flash firmware.

## Before you start

You need:

- Linux or another host that can receive multicast DNS from the device;
- Git, GNU Make, and Go 1.26.1;
- the device already running the exact firmware embedded at the top of
  `examples/lang/esphome-blink.mcl`;
- the host and device on a trusted network where `esphome-blink.local` reaches
  the intended board.

The fixture contains an intentionally public demonstration Noise key. Use it
only for this controlled demo. A normal deployment needs a unique key supplied
through an ignored local configuration; never commit that key or paste it into
an issue.

## Build the exact MGMT branch

From a directory where you keep source repositories:

```bash
git clone https://github.com/flavio-fernandes/mgmt.git
cd mgmt
git switch feat/esphome
git pull --ff-only origin feat/esphome
make build
```

The successful build prints a version and leaves the executable at `./mgmt`.
If the shell reports `go: command not found`, install Go 1.26.1 and confirm
`go version` works before retrying.

## Run the blink loop

```bash
./mgmt run --tmp-prefix lang examples/lang/esphome-blink.mcl
```

Within a few seconds you should see all of these behaviors:

- the endpoint publishes `esphome-blink.local:6053`;
- ESPHome logs flow through MGMT;
- `on-board led is on` changes from `false` to `true`;
- MGMT reports that it is turning `Onboard LED` off;
- the state returns to `false`, then repeats about three seconds later.

Press `Ctrl+C` after several cycles. A clean stop ends with `main: goodbye!`.

ESPHome debug output can contain private network identifiers even when the
Noise key is absent. Do not save or share the raw console output. When recording
evidence, keep only the MGMT and ESPHome versions, sanitized board profile,
MCL hash, behaviors observed, and outcome.

## If it does not connect

- Confirm the device is online and running the firmware embedded in the MCL.
- Confirm multicast DNS is allowed between the host and device. Do not add an
  `/etc/hosts` entry when testing `.local` compatibility; that bypasses the
  behavior this demo is meant to prove.
- A Noise failure usually means the device and MCL keys differ. Update only an
  ignored local copy for a private key, and never commit the result.
- Read the stage and target in the connection error, but redact private targets
  before sharing it.

The sanitized evidence from a successful ESPHome 2026.7.0 run is recorded in
[`compatibility/mgmt-feat-esphome-hardware-blink.json`](../compatibility/mgmt-feat-esphome-hardware-blink.json).
