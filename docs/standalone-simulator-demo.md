# Standalone simulator demo

This guide builds a small Go executable that uses `go-aioesphomeapi` directly.
MGMT is not involved. The executable talks to a simulated ESPHome conveyor
device through the same client API that an application would use for a real
device later.

The demo is safe by default:

- no hardware;
- no firmware flashing;
- no camera, motor, or actuator access;
- no real ESPHome key;
- no host-network listener.

It uses an in-process simulated device with a public test-only Noise key.

## What you need

Run these commands from the `go-aioesphomeapi` repository root.

Required tools:

- Go 1.25.10 or a later compatible Go 1.25 patch
- `git`

## 1. Build the built-in standalone example

This is the smallest useful command. It creates a normal executable from the
example program in `cmd/conveyor-sim`.

```bash
go build -o /tmp/go-aio-conveyor-sim ./cmd/conveyor-sim
```

Run it:

```bash
/tmp/go-aio-conveyor-sim
```

Expected output:

```text
connected securely to conveyor-simulator; discovered 13 entities
simulated conveyor speed=35 and status color=#00ff00
```

That output means the executable created a simulated ESPHome peer, opened an
encrypted client session, discovered entities, subscribed to state, sent a Fan
command for conveyor speed, and sent an RGB Light command for the status color.

## 2. Create your own tiny app module

This creates a throwaway Go module under `/tmp`, copies the demo source as a
starting point, and points the module at your local checkout. This is useful
while the project is still a development branch and not a tagged release.

```bash
demo_dir="$(mktemp -d)"
repo_root="$(pwd)"
cp cmd/conveyor-sim/main.go "${demo_dir}/main.go"
cd "${demo_dir}"
go mod init example.com/standalone-esphome-sim
go mod edit -require github.com/flavio-fernandes/go-aioesphomeapi@v0.0.0
go mod edit -replace github.com/flavio-fernandes/go-aioesphomeapi="${repo_root}"
go mod tidy
go build -o standalone-esphome-sim .
./standalone-esphome-sim
```

Expected final output:

```text
connected securely to conveyor-simulator; discovered 13 entities
simulated conveyor speed=35 and status color=#00ff00
```

## 3. Read the program

From the throwaway module directory created above:

```bash
sed -n '1,220p' main.go
```

The important calls are:

- `simulator.New(simulator.ConveyorScenario())` creates the device-side peer;
- `device.ClientOptions()` supplies the secure test transport;
- `api.DialWithContext(...)` opens the client session;
- `client.ListEntities()` discovers the simulated ESPHome entities;
- `client.SubscribeStates(nil)` starts state delivery;
- `client.SetFan(...)` sends a motor-style Fan command;
- `client.SetLight(...)` sends an RGB Light command.

## 4. Clean up

The demo lives in `/tmp`. Remove it when you are done:

```bash
rm -rf "${demo_dir}"
rm -f /tmp/go-aio-conveyor-sim
```

## Troubleshooting

If `go build` says `go: command not found`, install Go and rerun the same
commands from the repository root.

If `go mod tidy` cannot download dependencies, check network access and rerun
the same command. The library itself still comes from your local checkout
because of the `replace` line.

If the output changes, run the built-in example first:

```bash
go run ./cmd/conveyor-sim
```

That narrows the problem to either the local checkout or the throwaway module
setup.

## Safety and licensing notes

The simulator key is public test data. Do not put a real ESPHome Noise key,
device address, SSID, serial number, or bench detail into a demo command or
documentation patch.

This repository is GPL-3.0-only. If you distribute a program that links this
library, review the license obligations for that program.
