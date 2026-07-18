# MGMT compatibility contract

MGMT is the first customer and the release-blocking external test. The baseline is the ten-commit `feat/esphome` series ending at [`8eab220`](https://github.com/flavio-fernandes/mgmt/commit/8eab220). Paths and hashes are locked in [`compatibility/mgmt-feat-esphome.json`](../compatibility/mgmt-feat-esphome.json).

## Non-negotiable outcome

The original `esphome0.mcl` and `esphome-blink.mcl` examples established the
baseline contract. A proposed MCL change is not called an incompatibility fix
until the flaw is documented, covered by review evidence, and accepted in MGMT
review. MGMT issue #2 accepted a comment-only blink correction and the spelling
fix from `esphome-conveyer.mcl` to `esphome-conveyor.mcl`; ADR 0012 records the
new hashes and migration impact.

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
- built-in `.local` multicast DNS resolution without requiring host-file edits;
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
- client `Done`, `Close`, and diagnostic `CloseReason`;
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

Keys remain runtime-only values and never appear in errors, logs, snapshots,
compatibility fixtures, or command examples. Connection errors name the
attempted host and port so operators can distinguish resolution, TCP, Noise,
and hello failures. Applications must treat those targets as operational
metadata and redact them before sharing diagnostics outside the deployment.

## Cross-repository test lane

1. Check out the exact MGMT revision from the manifest.
2. Verify hashes before applying the small import/module patch.
3. Point MGMT to the local candidate module without editing MCL files.
4. Run the MGMT bridge and session unit tests.
5. Run both MCL examples against named simulator scenarios.
6. Assert one connection per endpoint, expected state events/commands/logs, polling behavior, reconnect/outage behavior, and clean shutdown.
7. Fail if MCL files differ, the module graph exceeds budget, or generated support claims lack evidence.

The `.local` lane must run with no matching `/etc/hosts` entry. A simulator
host-file injection proves only TCP behavior and is not accepted as mDNS
compatibility evidence.

Later MGMT revisions get new manifest records; history is append-only so old compatibility remains reproducible.

## Historical candidate and current merged lane

The append-only [`mgmt-feat-esphome2.json`](../compatibility/mgmt-feat-esphome2.json) record captures the current proof:

- upstream `purpleidea/mgmt:master` at `0bd1c2f4aa7c2d107de0dbe413ed8c9e5a36fd99`;
- rebased reference-client baseline `feat/esphome` at `5bf41f505bc601e6d2c4da8ecb3050b7c01ff34a`;
- three-commit replacement `feat/esphome2` at `398a8e9296fc79513756964304f16fdf7c1a1da0` in [MGMT PR #1](https://github.com/flavio-fernandes/mgmt/pull/1);
- this library at `238f06dc564ec3b4a16473ef5225447c4303166c` in [library PR #29](https://github.com/flavio-fernandes/go-aioesphomeapi/pull/29).

Both existing MCL examples are byte-identical between the rebased baseline and replacement candidate. The candidate builds MGMT, passes the targeted race/resource/vet checks, type-checks all three MCL examples, and replaces three modules with one direct module for a net reduction of two modules. It also requires a Noise key at the MGMT endpoint, so the adapter cannot silently downgrade to plaintext.

The append-only [`mgmt-feat-esphome2-runtime.json`](../compatibility/mgmt-feat-esphome2-runtime.json) follow-up records the next evidence level. A real MGMT process securely drove the loopback simulator with the unchanged conveyor MCL, observed initial telemetry and a device log, applied Fan and RGB Light commands, converged, and sent a second fan-stop command during graceful cleanup. The run exposed and fixed an endpoint-removal ordering defect in MGMT cleanup at `acddc3f1804dd3ae3e29f077996b7845e768ae29`.

This qualifies only the conveyor-exercised cells for `mgmt` evidence. Polling, fault scenarios, and entity families not used by that MCL keep their prior evidence. No physical device has been flashed or actuated.

The next append-only [`mgmt-feat-esphome2-baselines.json`](../compatibility/mgmt-feat-esphome2-baselines.json) record pins MGMT `a29ebe1e` and library `ef838682`. A real MGMT process converged both original MCL examples byte-for-byte over Noise. Real-driver tests additionally prove polling cleanup and command wakeup plus MGMT-owned reconnect and positive outage accounting. Those tests exposed and fixed a stale `Configure` wake race that could truncate polling's initial snapshot-settle window.

MGMT PR #1 has since merged into active branch `feat/esphome`, while the original Richard87 implementation is preserved at `feat/esphome-richard87`. The append-only [`mgmt-feat-esphome-postmerge.json`](../compatibility/mgmt-feat-esphome-postmerge.json) record pins post-merge MGMT `c60c22eb`. It confirms both original MCL examples and the unchanged conveyor MCL still pass over Noise. It also records a post-merge cleanup race fix: conveyor fan cleanup now uses a one-shot command with the last known endpoint info instead of racing the shared endpoint bridge unpublish. Later pushed-state timelines, text-sensor runtime evidence, and hardware remain pending.

The current append-only [`mgmt-feat-esphome-review.json`](../compatibility/mgmt-feat-esphome-review.json)
record pins the final reviewed branch after MGMT issue #2: MGMT `feat/esphome`
at `90a172d09239925db5a527ee7b2a5edc383c08a3` selects merged library `main`
at `6f954bc92a84b8a2bcb12acef5462b2445edfc08`. The two reviewed baseline
MCL files and the corrected conveyor MCL pass through encrypted simulators and
multicast DNS without `/etc/hosts`. `feat/esphome-richard87` remains preserved
at `5bf41f505bc601e6d2c4da8ecb3050b7c01ff34a`; implicit plaintext remains the
only intentional parity exception.

## Preserved-branch parity audit

The replacement preserves the valid encrypted MGMT behavior exercised by
`feat/esphome-richard87`: endpoint publication, shared sessions, persistent and
polling operation, reconnect/outage accounting, entity discovery and lookup,
state callbacks, logs, switch/number/button commands, cleanup, and `.local`
hostname resolution. The two original MCL files remain byte-identical.

There is one deliberate exception: an empty Noise key no longer silently
selects plaintext. That behavior is rejected by the security policy and is not
restored as “parity.” Explicit insecure plaintext remains available to isolated
library tests, but MGMT's normal endpoint requires a key.

The append-only [`mgmt-feat-esphome-mdns.json`](../compatibility/mgmt-feat-esphome-mdns.json)
records the regression test: real MGMT resolves both `esphome-blink.local` and
`esphome-conveyer.local` over multicast DNS and converges the unchanged MCL
files without an `/etc/hosts` mapping.

The follow-up [`mgmt-feat-esphome-diagnostics.json`](../compatibility/mgmt-feat-esphome-diagnostics.json)
closes the pending pin: MGMT `feat/esphome` now selects the exact published
library revision that provides built-in mDNS and preserved connection causes.
Both original MCL examples and the unchanged conveyor MCL pass from that
committed `go.mod` with `GOWORK=off`. MGMT receives the improved synchronous
connection errors automatically; exposing asynchronous `CloseReason` through
the MGMT adapter remains future work.

## First sanitized hardware evidence

The append-only [`mgmt-feat-esphome-hardware-blink.json`](../compatibility/mgmt-feat-esphome-hardware-blink.json)
records the first real-device run. MGMT `d6259199` used its committed library
pin and the byte-identical `esphome-blink.mcl` to connect to ESPHome 2026.7.0.
The real device resolved through `.local`, completed Noise and hello, published
the expected switch and binary-sensor entities, streamed states and logs, and
accepted at least eight corrective switch commands during a bounded run.

The run did not flash firmware or write raw logs to a file or repository.
Network identifiers, the Noise key, local paths, and firmware build metadata
are deliberately absent from the record. This raises only the exercised blink
path to `hardware`;
conveyor, Fan, Light, Number, Button, reconnect, and scale claims remain at
their existing evidence levels.
