# MGMT compatibility contract

MGMT is the first customer and the release-blocking external test. The baseline is the ten-commit `feat/esphome` series ending at [`8eab220`](https://github.com/flavio-fernandes/mgmt/commit/8eab220). Paths and hashes are locked in [`compatibility/mgmt-feat-esphome.json`](../compatibility/mgmt-feat-esphome.json).

## Non-negotiable outcome

The existing `esphome0.mcl` and `esphome-blink.mcl` examples run without source changes. A proposed MCL change is not called an incompatibility fix until the flaw is documented, covered by a failing test, and accepted in MGMT review.

The Go migration target is import-path-only changes in `util/esphome/apiclient.go`. If a stronger safety contract requires another adapter change, the pull request lists every changed line and why valid MCL behavior is unaffected.

## Existing MCL surface

| Surface | Contract |
|---|---|
| `esphome:endpoint` | `host`, default `port` 6053, `key`, rejected legacy `password`, `interval`, `logs` |
| `esphome:switch` | endpoint, desired `on`/`off`, entity `id` defaulting to resource name |
| `esphome:number` | endpoint, desired value, optional entity `id`, outage `stop`, `safe` value |
| `esphome.connected` | live health; persistent connection or last successful poll |
| `esphome.binary_sensor` | live boolean or zero value while unknown/missing |
| `esphome.sensor` | live float or zero value while unknown/missing |
| `esphome.text_sensor` | live string or zero value while unknown/missing |

This repository records the contract but does not copy MGMT's GPL source or MCL files.

## Existing MGMT behavior we preserve

- one shared session per logical endpoint across functions and resources;
- endpoint data published through MGMT's local bridge;
- zero-valued functions before endpoint publication or first valid state;
- persistent native push when `interval` is zero;
- polling, command wakeup, and last-success health when `interval` is positive;
- exact current entity name and legacy object-ID lookup;
- initial state snapshot followed by change events;
- MGMT-owned reconnect and exponential backoff;
- pending commands serialized against a live connection;
- no command issued during check-only evaluation;
- number outage interlock and best-effort safe cleanup;
- optional device logs flowing through the endpoint logger;
- final reservation release closes the shared session.

The compatibility test suite must exercise these behaviors, not only compile symbols.

## Library surface MGMT needs

The compatibility facade initially mirrors the subset recorded in the manifest:

- `DialWithContext`, `WithClientInfo`, and `WithEncryptionKey`;
- client `ListEntities`, `Entities`, `SubscribeStates`, and `SubscribeLogs`;
- registry accessors for binary sensor, sensor, text sensor, switch, number, and button;
- client `SetSwitch`, `SetNumber`, and `PressButton`;
- client `Done` and `Close`;
- generated state and log messages needed by the existing driver.

Internally, the implementation may be entirely different. It must improve cancellation, limits, secret redaction, callback isolation, and malformed-peer handling without altering valid MGMT results.

## Ownership

| Concern | Owner |
|---|---|
| Native API framing, Noise, one connection, discovery, state, commands | this library |
| Shared endpoint pool, persistent/polling loop, backoff, outage tracking | MGMT branch |
| Desired state, graph ordering, CheckApply, resources/functions | MGMT |
| Conveyor policy and MCL | MGMT example/module |
| Immediate motor safety | ESPHome firmware and physical hardware |

The library never imports MGMT. MGMT may vendor or require this GPL-3.0-only module.

## Security adaptation

The baseline allows plaintext when `key` is empty. This project's normal production API fails closed without Noise. MGMT must therefore make plaintext an explicit insecure endpoint choice before production acceptance. This is an intentional hardening candidate, not a silent MCL change; it needs a dedicated MGMT review and simulator test.

Keys remain runtime-only values and never appear in errors, logs, snapshots, compatibility fixtures, or command examples. Hostnames and entity names are treated as operational metadata and redacted by default observability hooks.

## Cross-repository test lane

1. Check out the exact MGMT revision from the manifest.
2. Verify hashes before applying the small import/module patch.
3. Point MGMT to the local candidate module without editing MCL files.
4. Run the MGMT bridge and session unit tests.
5. Run both MCL examples against named simulator scenarios.
6. Assert one connection per endpoint, expected state events/commands/logs, polling behavior, reconnect/outage behavior, and clean shutdown.
7. Fail if MCL files differ, the module graph exceeds budget, or generated support claims lack evidence.

Later MGMT revisions get new manifest records; history is append-only so old compatibility remains reproducible.
