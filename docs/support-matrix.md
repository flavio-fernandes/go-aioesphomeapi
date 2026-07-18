# Support matrix

This is the sole source of truth for compatibility claims. Protocol presence, reference-client support, MGMT need, and this library's evidence are different facts.

## Evidence levels

| Level | Meaning |
|---|---|
| `untracked` | Not yet inventoried from pinned upstream. |
| `known` | Present in pinned protocol inventory; generated types alone qualify only here. |
| `typed` | Handwritten or declared compatibility API and validation exist. |
| `simulated` | Deterministic real-wire client/server tests cover success and negative paths. |
| `mgmt` | Pinned MGMT compiles and its external behavior passes without MCL changes. |
| `hardware` | Tested against a recorded ESPHome release and sanitized hardware profile. |
| `production` | Security, race, fuzz, load, observability, compatibility, and release gates pass. |

The pinned protocol inventory contains 148 unique message IDs. Its
[generated compatibility view](protocol-inventory.md) records gates, MGMT need,
reference parity, public behavior, and each evidence level for every message.
Thirty-one messages make up the implemented M1 slice; the other 117 remain
explicitly `generated_only`. Generated presence is `known`; the table below
distinguishes the implemented conveyor/MGMT slice from all other generated
messages. `planned` is roadmap intent, not evidence.

## MGMT compatibility baseline

| Behavior | Required by pinned MGMT | Library evidence | Target | Notes |
|---|---|---|---|---|
| Context-bound Noise dial | yes | hardware | M1 | MGMT completed Noise against ESPHome 2026.7.0 hardware; normal path fails closed without secure configuration. |
| Explicit insecure plaintext | compatibility review | simulated | M1 | Requires `WithInsecurePlaintext`; never selected implicitly. |
| `.local` mDNS resolution | yes | hardware | M1 | The unchanged blink MCL resolved a real device; simulator coverage also proves no `/etc/hosts` mapping or added module is needed. |
| Diagnostic error chains and close reason | operational | simulated | M1 | Dial retains `*net.OpError`; mDNS, Noise, rejected keys, and hello are distinct; peer rejection text is capped/sanitized; asynchronous read/decode/context/peer/queue termination is observable. |
| Entity list and registry metadata | yes | hardware | M1 | A real blink switch and binary sensor were resolved by exact current name. |
| Initial state snapshot and live push | yes | hardware | M1 | Real blink state repeatedly reached MCL; broader push/fault evidence is still pending. |
| Binary sensor state | yes | hardware | M1 | Real blink state and three simulated conveyor binary sensors reached MCL. |
| Sensor state and missing/NaN | yes | mgmt | M1 | RGB sensor state reached MCL; missing/NaN remains adapter-test evidence. |
| Text sensor state | yes | typed | M1 | MGMT evidence pending. |
| Switch state and command | yes | hardware | M1 | The unchanged blink MCL issued at least eight corrective commands accepted by real firmware. |
| Number state and command | yes | mgmt | M1 | Unchanged `esphome0.mcl` observed state and issued its safe desired number command. |
| Button discovery and command | yes in driver | simulated | M1 | Exposed even though current examples do not call it. |
| Fan state and command | conveyor | mgmt | M1 | State, speed, direction, and graceful cleanup stop passed. |
| RGB Light state and command | conveyor | mgmt | M1 | State, brightness, and blue RGB command passed. |
| Device logs | yes | hardware | M1 | ESPHome 2026.7.0 logs reached the MGMT endpoint logger; only sanitized evidence was committed. |
| Done signal and idempotent close | yes | simulated | M1 | Race tests cover clean termination. |
| Hostile peer and stalled operation | security | simulated | M1 | Named drop, malformed-protobuf, duplicate-completion, incomplete-discovery, and stalled-discovery tests are panic-free over the real framing/session path; bounded unknown IDs are skipped and subsequent known traffic succeeds. |
| MGMT-owned reconnect and outage accounting | external contract | mgmt | M1 | Real encrypted driver test drops a persistent peer, reconnects through MGMT, records a positive outage, and observes no unrequested replay. |
| Library-owned reconnect | no | none | M2 | MGMT owns reconnect; client option stays disabled. |
| MGMT persistent and polling modes | external contract | mgmt / mgmt | M1 | Persistent MCL and conveyor runs pass; real-driver polling disconnects between cycles and wakes once for a queued command. |
| Unchanged `esphome0.mcl` | yes | mgmt | M1 | Hash verified, real MGMT converged, and switch/number corrections reached the encrypted peer. |
| Reviewed `esphome-blink.mcl` | yes | hardware | M1 | A review-accepted comment-only correction changed its hash without changing behavior; real MGMT converged repeatedly against encrypted ESPHome 2026.7.0 hardware. |

## Current MGMT migration proof

The evidence is append-only: [`compatibility/mgmt-feat-esphome2.json`](../compatibility/mgmt-feat-esphome2.json) records the first candidate, while [`compatibility/mgmt-feat-esphome-review.json`](../compatibility/mgmt-feat-esphome-review.json) records the final reviewed branches. The [timeline candidate record](../compatibility/mgmt-feat-esphome-simulator-timeline-candidate.json) preserves ADR 0013's measured before/after reconnect behavior. Historical rows remain so a successful build is never mistaken for later runtime or hardware evidence.

| Check | Result | Evidence level impact |
|---|---|---|
| Rebased MGMT baseline | `5bf41f505bc601e6d2c4da8ecb3050b7c01ff34a` on upstream `0bd1c2f4aa7c2d107de0dbe413ed8c9e5a36fd99` | Reproducible baseline only. |
| Replacement candidate | `398a8e9296fc79513756964304f16fdf7c1a1da0` using library `238f06dc564ec3b4a16473ef5225447c4303166c` | Candidate pin only. |
| Existing MCL sources | Both SHA-256 values unchanged; both pass `mgmt check` | Contract and type-check proof. |
| MGMT build, targeted tests, race, and vet | pass | Adapter integration proof. |
| Module graph | one module added, three removed; net `-2` | Dependency-budget proof. |
| Conveyor firmware with ESPHome 2026.7.0 | configuration and compile pass | Firmware build proof, not hardware proof. |
| MGMT-to-library simulator session | pass for the unchanged conveyor MCL | Binary sensor, sensor, Fan, Light, Noise, discovery, initial state, and logs reach `mgmt`. |
| Graceful conveyor cleanup | pass after MGMT follow-up `acddc3f1` | A second fan-stop command is observed; failed cleanup is forbidden. |
| Hostile-peer simulator and fuzz smoke | pass in library tests | Simulator evidence only; no MGMT or hardware claim. |
| Both original MGMT MCL examples | pass byte-for-byte over Noise | Switch and number plus both immutable MCL rows reach `mgmt`. |
| Real-driver polling and reconnect | pass, including 10 race-enabled repetitions | Poll cleanup, command wake, MGMT-owned reconnect, outage accounting, and no unrequested replay reach `mgmt`. |
| Post-merge active MGMT branch | pass on `feat/esphome` at `c60c22eb` | Both original MCL examples and the unchanged conveyor MCL remain green after PR #1 merged and `feat/esphome2` was retired. |
| Post-merge `.local` parity | pass with library `55602f04` | Real MGMT resolves blink and conveyor names from multicast answers; no `/etc/hosts` entry or new module is used. |
| Final mDNS and diagnostics pin | MGMT `d6259199` pins library `73b5d58e` | All unchanged MCL demos pass from the committed module version; synchronous connection failures retain causes and attempted targets. |
| Physical ESPHome blink device | pass on ESPHome 2026.7.0 with MGMT `d6259199` | Noise, `.local`, hello, discovery, binary-state push, switch command, and logs reach `hardware`. |
| Final reviewed MGMT branch | pass on `feat/esphome` at `90a172d0`, pinning merged library `main` at `6f954bc9` | Targeted race/vet, both reviewed baseline MCL examples, and the reviewed conveyor MCL pass without `/etc/hosts`; issue reconciliation may close only evidence-complete work. |
| Security-updated MGMT pin | pass on `feat/esphome` at `ede17372`, pinning library `main` at `f1f9e3ef` | Go 1.25.12, `x/crypto` v0.52.0, zero open Dependabot alerts, targeted race/vet/build, and all reviewed MCL simulator lanes pass without module-count or MCL changes. |
| Latest-state timeline candidate | pass with library `62c6962b` and MGMT `8e8b1599` | Manual-clock pushes retain equal-time order; one switch command survives a same-device reconnect as the latest snapshot, with positive outage accounting and zero command-count delta. All unchanged MCL lanes remain green. |
| Firmware flash and physical conveyor actuation | not performed | Conveyor, Fan, Light, and firmware-provisioning hardware cells remain `no`. |

## Protocol and transport

| Capability | Upstream | Public API | Simulator | MGMT | Hardware | Target |
|---|---|---|---|---|---|---|
| Plain framing with limits | known | typed | simulated | none | none | M1 |
| Noise transport | known | typed | simulated | mgmt | hardware | M1 |
| `.local` A-record resolution | known | typed | simulated | mgmt | hardware | M1 |
| Hello and API version | known | typed | simulated | mgmt | hardware | M1 |
| Device information | known | none | none | none | none | M1 |
| Ping, disconnect, close | known | typed | simulated | none | none | M1 |
| Entity discovery | known | typed | simulated | mgmt | hardware | M1 |
| State subscriptions | known | typed | simulated | mgmt | hardware | M1 |
| Bounded device logs | known | typed | simulated | mgmt | hardware | M1 |
| Client-owned reconnect | known | none | none | n/a | none | M2 |
| Home Assistant services/actions | known | none | none | none | none | M3 |
| Bluetooth proxy | known | none | none | none | none | backlog |
| Voice assistant | known | none | none | none | none | backlog |
| Camera streaming | known | none | none | none | none | backlog |

## Entity families

| Family | MGMT M1 need | Protocol known | Typed | Simulated | MGMT | Hardware | Target |
|---|---|---|---|---|---|---|---|
| Binary sensor | state | yes | yes | yes | yes | yes | M1 |
| Sensor | state | yes | yes | yes | yes | no | M1 |
| Text sensor | state | yes | yes | yes | no | no | M1 |
| Switch | state/command | yes | yes | yes | yes | yes | M1 |
| Number | state/command | yes | yes | yes | yes | no | M1 |
| Button | command seam | yes | yes | yes | no | no | M1 |
| Fan | conveyor state/command | yes | yes | yes | yes | no | M1 |
| Light | conveyor color/state/command | yes | yes | yes | yes | no | M1 |
| Select | no | yes | no | no | no | no | M2 |
| Text | no | yes | no | no | no | no | M3 |
| Climate | no | yes | no | no | no | no | M3 |
| Cover | no | yes | no | no | no | no | M3 |
| Lock | no | yes | no | no | no | no | M3 |
| Alarm control panel | no | yes | no | no | no | no | backlog |
| Media player | no | yes | no | no | no | no | backlog |
| Update | no | yes | no | no | no | no | backlog |

## Simulator contract

| Capability | Public API | Simulator evidence | MGMT evidence | Target | Notes |
|---|---|---|---|---|---|
| Typed scenario validation | typed | simulated | n/a | M1 | `Scenario.Validate` and deferred `DialContext`/`Serve` rejection preserve `New` compatibility; errors expose stable codes without scenario data. |
| Defensive scenario creation | typed | simulated | n/a | M1 | Valid protobuf entities, initial states, timeline events, and logs are cloned before the device can observe caller mutation. |
| Conditional random seed | typed | simulated | n/a | M1 | Zero is valid without randomized actions; issue #10 must require non-zero only when such actions are introduced. |
| Manual virtual time and ordered state pushes | typed | simulated | candidate | M1 | Explicit synchronous advances apply absolute events; equal-time events retain declaration order. Final `mgmt` evidence requires the published pin and unchanged MCL lanes. |
| Device-global latest-state reconnect snapshot | typed | simulated | candidate | M1 | Commands and timeline events share one store; a same-device reconnect receives one latest snapshot with no command or past-event replay. ADR 0013's final pinned re-baseline remains required. |

## Reference-client parity

`v1.1.0` is a comparison baseline, not evidence for this library. M1 covers only its MGMT-consumed subset. Direct `.local` A-record resolution is supported; CLI, YAML configuration, mDNS service browsing, Bluetooth proxy, services, and broad complex-entity commands are explicitly unsupported here until separately scheduled and evidenced.

## Compatibility dimensions

Each release reports Go version, OS/architecture, ESPHome oldest/current/development versions, transport mode, entity direction, simulator evidence, MGMT revision, reference-client migration surface, and hardware evidence separately.

The supported toolchain starts at **Go 1.25.12**. Earlier review raised the
floor from MGMT's Go 1.25.7 to 1.25.10 for reachable GO-2026-4971. Continuous
scanning then found imported-package GO-2026-4970 plus four module-level
standard-library findings in 1.25.10; 1.25.12 fixes all five without a language
version jump. `x/crypto`'s GO-2026-5932 remains an unreachable module-level
notice for the unimported `openpgp` package, which has no fixed release; this
library uses only Noise-required crypto packages and records the non-exposure
in ADR 0007.
