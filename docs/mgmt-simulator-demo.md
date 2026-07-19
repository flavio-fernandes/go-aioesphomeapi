# MGMT simulator demo

This guide runs MGMT against a simulated ESPHome conveyor device from this
repository. It uses the real Native API wire path, Noise encryption, the real
MGMT binary, and the `esphome-conveyor.mcl` example. It does not use hardware,
flash firmware, open a host-network service, or require a real ESPHome key.

The result is a small end-to-end demo: MGMT discovers a conveyor simulator,
reads its initial telemetry, applies a fan command for the motor, applies an
RGB light command for status color, receives a device log, converges, and sends
the cleanup stop command.

## What you need

Run these commands on Linux from the `go-aioesphomeapi` repository root.

Required tools:

- the Go version requested by the MGMT checkout; the current `feat/esphome`
  branch uses Go 1.26.1
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

From the `go-aioesphomeapi` repository root:

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

## 2. Look at the MCL that will run

This is optional, but useful before the demo:

```bash
sed -n '1,220p' ../mgmt-esphome2/examples/lang/esphome-conveyor.mcl
```

The important thing to notice is that the demo runs MGMT's MCL file as-is. The
test wrapper provides a private simulated network around it instead of asking
you to edit `/etc/hosts` or change the example for your machine. A small mDNS
responder inside that private network answers `esphome-conveyor.local`, so this
walkthrough also tests the same name-resolution path used by an ESPHome device.

## 3. Run the conveyor demo

```bash
./tools/test-mgmt-conveyor.sh ../mgmt-esphome2 /tmp/mgmt-esphome2-sim-demo
```

Expected final line:

```text
MGMT securely converged the reviewed conveyor MCL against the loopback simulator
```

That one line means the wrapper verified all of these checks:

- the conveyor MCL hash matches the reviewed compatibility contract;
- the simulator listened only inside a private namespace on loopback;
- MGMT resolved `esphome-conveyor.local` through multicast DNS, not `/etc/hosts`;
- MGMT connected over the encrypted Native API path;
- MGMT observed conveyor telemetry and a simulator device log;
- MGMT sent the expected Fan command for the motor;
- MGMT sent the expected RGB Light command for the status light;
- MGMT converged and then sent the cleanup Fan stop command.

## 4. Try the original MGMT examples too

The conveyor demo is the fun one. This command proves the older MGMT examples
still work without changing their MCL source:

```bash
./tools/test-mgmt-baselines.sh ../mgmt-esphome2 /tmp/mgmt-esphome2-sim-demo
```

Expected final line:

```text
MGMT securely converged both reviewed baseline MCL examples against dedicated simulators
```

Those runs prove one corrective convergence and exit. To watch the blink
example run as an endless loop against a simulated device that behaves like
the real firmware, follow the [MGMT blink loop demo](mgmt-blink-demo.md).

## Troubleshooting

If the script says a command is missing, install the named Linux tool and rerun
the same command.

If the script says the MGMT binary is not executable, rebuild it with the exact
`go build` command above and verify `/tmp/mgmt-esphome2-sim-demo --version`.

If the script says the MCL hash differs, your MGMT checkout does not match the
reviewed demo branch. Switch back to the `feat/esphome` branch or inspect the
MCL change before trusting the result.

If `unshare` fails, your Linux environment may not allow unprivileged user or
network namespaces. The demo intentionally uses those namespaces so the hard
parts of the example stay isolated from the host network.

## Safety notes

The key printed by the simulator is public test data. Do not replace it with a
real ESPHome Noise key in a shell command, log, issue, or documentation patch.

This demo is simulator evidence. It proves the MGMT integration path without
hardware. It does not prove a physical conveyor, firmware pinout, motor driver,
camera setup, or workbench device.
