# MGMT simulator demo

This guide runs MGMT against a simulated ESPHome conveyor device from this
repository. It uses the real Native API wire path, Noise encryption, the real
MGMT binary, and the `esphome-conveyor.mcl` example unchanged. It does not use
hardware, flash firmware, open a host-network service, or require a real
ESPHome key.

The result is a small end-to-end demo of the whole conveyor story: bricks go
down a belt, MGMT decides when the belt runs, and MGMT decides what the status
light says about each brick.

## What the demo shows

The simulated device reports two facts and accepts two commands. That is the
entire interface:

| Entity | Family | Direction | Meaning |
|---|---|---|---|
| `Brick En Route` | binary sensor | device reports | a brick is on its way to the exit |
| `Exit Red Ratio` | sensor | device reports | how red the brick at the exit is, or a negative value for no trustworthy reading |
| `Conveyor Motor` | fan | MGMT commands | the belt |
| `Status Light` | light | MGMT commands | color and effect |

The device never steers the belt while MGMT is connected. It reports what its
sensors see; MGMT owns every decision. That single-owner rule is what the demo
is really about, and the simulator enforces it by only ever moving the motor
itself in the one case the firmware does: when a run request goes unanswered
long enough to look like a wedged controller.

One brick produces four state messages, not a telemetry stream. Every raw
sensor behind those two facts is internal to the device.

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

The `-ldflags` are not optional. A plain `go build` produces a binary that
refuses to run because it has no program name compiled in.

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

Two lines in that file decide everything the light does. The threshold that
separates a red brick from every other color, and the rules that pick an
effect, are ordinary MCL. Change the threshold, save, and the next brick is
classified by the new rule with no reflash and no restart.

## 3. Run the conveyor demo

```bash
./tools/test-mgmt-conveyor.sh ../mgmt-esphome2 /tmp/mgmt-esphome2-sim-demo
```

The run takes about twenty seconds, because it runs two bricks down the belt at
the real firmware's pacing rather than a scaled-down one.

Expected final line:

```text
MGMT securely converged the reviewed conveyor MCL against the loopback simulator
```

That one line means the wrapper verified all of these checks:

- the conveyor MCL hash matches the reviewed compatibility contract;
- the simulator listened only inside a private namespace on loopback;
- MGMT resolved `esphome-conveyor.local` through multicast DNS, not
  `/etc/hosts`;
- MGMT connected over the encrypted Native API path;
- MGMT read the device's boot state: no brick en route and no color reading;
- a brick settled on the entry, so MGMT started the belt and selected the
  traveling blink effect;
- the brick arrived, so MGMT stopped the belt and received one averaged color
  reading;
- MGMT classified a 48% red ratio as red and set the light to solid red;
- MGMT classified a 29% red ratio as another color and selected the rainbow
  effect;
- the light returned to idle white with no effect after the last brick;
- MGMT converged, exited, and left the belt stopped;
- the device's own jam backstop never had to fire.

The second-to-last check is a regression guard rather than a formality. See
[returning to idle](#returning-to-idle) below.

## 4. Watch it yourself

The acceptance script asserts; it does not narrate. To watch the same thing,
run the two halves in two terminals.

First terminal, the device. `--cycles -1` keeps putting bricks on the belt
until you interrupt it:

```bash
go run ./cmd/conveyor-sim-server --listen 127.0.0.1:6053 --cycles -1
```

It prints the public test key and every command it receives, including the
effect MGMT selected:

```text
secure conveyor simulator listening on 127.0.0.1:6053
public test-only Noise key: kJ7hc0lJ0Zw9N3DcJzXn1kJ7hc0lJ0Zw9N3DcJzXn1k=
received fan command: state=true speed=100 direction=forward
received light command: state=true brightness=0.35 rgb=#ffffff effect="Traveling Blink"
```

Second terminal, MGMT. This one needs `esphome-conveyor.local` to resolve to
`127.0.0.1`, which the acceptance script arranges with a private namespace and
an mDNS responder. Outside that namespace, add the name to `/etc/hosts`
yourself, or run the acceptance script instead:

```bash
cd ../mgmt-esphome2
/tmp/mgmt-esphome2-sim-demo run --tmp-prefix lang examples/lang/esphome-conveyor.mcl
```

Useful flags on the device for trying things:

| Flag | Default | What it does |
|---|---|---|
| `--cycles` | `0` | bricks to run: `0` is a static device that only reports its boot state, negative runs until interrupted |
| `--travel` | `1200ms` | how long a brick takes to get from the entry sensor to the exit |
| `--dwell` | `1500ms` | how long a measured brick sits at the exit before the simulated operator lifts it off |

Setting `--travel` longer than fifteen seconds is the way to watch the jam
timeout: the device withdraws the run request, MGMT stops the belt because its
own policy says to, and the firmware backstop then finds nothing left to do.

## Returning to idle

The demo asserts that the light ends idle, because that specific transition
used to fail on real hardware. Removing a red brick worked; removing any other
color left the rainbow running until something unrelated disturbed the light.

The cause is an MGMT bug, not a device or library one:
[purpleidea/mgmt#966](https://github.com/purpleidea/mgmt/issues/966). An `if`
expression nested inside the branch of another one makes MGMT drop the update
that switches between them, and the dropped update was the one that cleared the
animation. The MCL avoids it by keeping each rule flat and combining the
results, so the demo asserts the outcome to keep it that way.

## Try the original MGMT examples too

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
MCL change before trusting the result. The conveyor MCL and the simulated
firmware are one contract: a change to the example's entity names, light
effects, or red threshold needs a matching change to
`simulator.ConveyorScenario` and to the assertions in
`tools/test-mgmt-conveyor.sh`.

If the script fails an assertion, it prints both logs. The MGMT log shows what
MGMT decided and the simulator log shows what the device received, so the two
together say which side of the wire lost the story.

If `unshare` fails, your Linux environment may not allow unprivileged user or
network namespaces. The demo intentionally uses those namespaces so the hard
parts of the example stay isolated from the host network.

## Safety notes

The key printed by the simulator is public test data. Do not replace it with a
real ESPHome Noise key in a shell command, log, issue, or documentation patch.

This demo is simulator evidence. It proves the MGMT integration path without
hardware. It does not prove a physical conveyor, firmware pinout, motor driver,
sensor calibration, or workbench device. The red ratios the simulator publishes
are the values recorded from a bench capture, not measurements taken during
this run.
