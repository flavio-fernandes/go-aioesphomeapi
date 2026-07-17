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

The pinned protocol inventory contains 148 unique message IDs. Generated presence is `known`; the table below distinguishes the implemented conveyor/MGMT slice from all other generated messages. `planned` is roadmap intent, not evidence.

## MGMT compatibility baseline

| Behavior | Required by pinned MGMT | Library evidence | Target | Notes |
|---|---|---|---|---|
| Context-bound Noise dial | yes | mgmt | M1 | Real MGMT process passed over Noise; normal path fails closed without secure configuration. |
| Explicit insecure plaintext | compatibility review | simulated | M1 | Requires `WithInsecurePlaintext`; never selected implicitly. |
| `.local` mDNS resolution | yes | mgmt | M1 | Both unchanged hostname-based MCL demos pass through a real multicast responder with no `/etc/hosts` mapping and no added module. |
| Diagnostic error chains and close reason | operational | simulated | M1 | Dial retains `*net.OpError`; mDNS, Noise, and hello are distinct; asynchronous read/decode/context/peer/queue termination is observable. |
| Entity list and registry metadata | yes | mgmt | M1 | Conveyor entities were resolved by exact current name. |
| Initial state snapshot and live push | yes | mgmt | M1 | Initial conveyor telemetry reached MCL; later push/fault evidence is still pending. |
| Binary sensor state | yes | mgmt | M1 | Three conveyor binary sensors reached MCL. |
| Sensor state and missing/NaN | yes | mgmt | M1 | RGB sensor state reached MCL; missing/NaN remains adapter-test evidence. |
| Text sensor state | yes | typed | M1 | MGMT evidence pending. |
| Switch state and command | yes | mgmt | M1 | Both unchanged baseline examples issued and observed corrective switch commands. |
| Number state and command | yes | mgmt | M1 | Unchanged `esphome0.mcl` observed state and issued its safe desired number command. |
| Button discovery and command | yes in driver | simulated | M1 | Exposed even though current examples do not call it. |
| Fan state and command | conveyor | mgmt | M1 | State, speed, direction, and graceful cleanup stop passed. |
| RGB Light state and command | conveyor | mgmt | M1 | State, brightness, and blue RGB command passed. |
| Device logs | yes | mgmt | M1 | Simulator info log reached the MGMT endpoint logger. |
| Done signal and idempotent close | yes | simulated | M1 | Race tests cover clean termination. |
| Hostile peer and stalled operation | security | simulated | M1 | Named drop, malformed-protobuf, unknown-message, incomplete-discovery, and stalled-discovery tests fail closed over the real framing/session path. |
| MGMT-owned reconnect and outage accounting | external contract | mgmt | M1 | Real encrypted driver test drops a persistent peer, reconnects through MGMT, records a positive outage, and observes no unrequested replay. |
| Library-owned reconnect | no | none | M2 | MGMT owns reconnect; client option stays disabled. |
| MGMT persistent and polling modes | external contract | mgmt / mgmt | M1 | Persistent MCL and conveyor runs pass; real-driver polling disconnects between cycles and wakes once for a queued command. |
| Unchanged `esphome0.mcl` | yes | mgmt | M1 | Hash verified, real MGMT converged, and switch/number corrections reached the encrypted peer. |
| Unchanged `esphome-blink.mcl` | yes | mgmt | M1 | Hash verified, real MGMT converged, and name-based switch/log behavior reached the encrypted peer. |

## Current MGMT migration proof

The candidate record is [`compatibility/mgmt-feat-esphome2.json`](../compatibility/mgmt-feat-esphome2.json). Evidence is intentionally split so a successful build is not mistaken for external runtime behavior.

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
| Physical device flash and actuation | not performed | Hardware cells remain `no`. |

## Protocol and transport

| Capability | Upstream | Public API | Simulator | MGMT | Hardware | Target |
|---|---|---|---|---|---|---|
| Plain framing with limits | known | typed | simulated | none | none | M1 |
| Noise transport | known | typed | simulated | mgmt | none | M1 |
| `.local` A-record resolution | known | typed | simulated | mgmt | none | M1 |
| Hello and API version | known | typed | simulated | mgmt | none | M1 |
| Device information | known | none | none | none | none | M1 |
| Ping, disconnect, close | known | typed | typed | none | none | M1 |
| Entity discovery | known | typed | simulated | mgmt | none | M1 |
| State subscriptions | known | typed | simulated | mgmt | none | M1 |
| Bounded device logs | known | typed | simulated | mgmt | none | M1 |
| Client-owned reconnect | known | none | none | n/a | none | M2 |
| Home Assistant services/actions | known | none | none | none | none | M3 |
| Bluetooth proxy | known | none | none | none | none | backlog |
| Voice assistant | known | none | none | none | none | backlog |
| Camera streaming | known | none | none | none | none | backlog |

## Entity families

| Family | MGMT M1 need | Protocol known | Typed | Simulated | MGMT | Hardware | Target |
|---|---|---|---|---|---|---|---|
| Binary sensor | state | yes | yes | yes | yes | no | M1 |
| Sensor | state | yes | yes | yes | yes | no | M1 |
| Text sensor | state | yes | yes | yes | no | no | M1 |
| Switch | state/command | yes | yes | yes | yes | no | M1 |
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

## Reference-client parity

`v1.1.0` is a comparison baseline, not evidence for this library. M1 covers only its MGMT-consumed subset. Direct `.local` A-record resolution is supported; CLI, YAML configuration, mDNS service browsing, Bluetooth proxy, services, and broad complex-entity commands are explicitly unsupported here until separately scheduled and evidenced.

## Compatibility dimensions

Each release reports Go version, OS/architecture, ESPHome oldest/current/development versions, transport mode, entity direction, simulator evidence, MGMT revision, reference-client migration surface, and hardware evidence separately.

The supported toolchain starts at **Go 1.25.10**. MGMT's inspected default was 1.25.7, but `govulncheck` found reachable GO-2026-4971 in that standard library; 1.25.10 is the fixed patch without a language-version jump. The experimental MGMT branch moved to Go 1.26.1 with the reference client.
