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

The pinned protocol inventory contains 148 unique message IDs. Generated presence is `known`; every handwritten implementation column remains `none` today. `planned` is roadmap intent, not evidence.

## MGMT compatibility baseline

| Behavior | Required by pinned MGMT | Library evidence | Target | Notes |
|---|---|---|---|---|
| Context-bound Noise dial | yes | none | M1 | Normal path must fail closed without secure configuration. |
| Explicit insecure plaintext | compatibility review | none | M1 | Baseline uses empty key; new spelling must be unmistakable. |
| Entity list and registry metadata | yes | none | M1 | Current name plus legacy object ID. |
| Initial state snapshot and live push | yes | none | M1 | Callback may not block network read loop. |
| Binary sensor state | yes | none | M1 | MCL bool function. |
| Sensor state and missing/NaN | yes | none | M1 | MCL float function. |
| Text sensor state | yes | none | M1 | MCL string function. |
| Switch state and command | yes | none | M1 | MCL resource. |
| Number state and command | yes | none | M1 | MCL resource and outage safety. |
| Button discovery and command | yes in driver | none | M1 | Expose even though current examples do not call it. |
| Device logs | yes | none | M1 | Bounded and redacted. |
| Done signal and idempotent close | yes | none | M1 | Must terminate goroutines. |
| Library-owned reconnect | no | none | M2 | MGMT owns reconnect; client option stays disabled. |
| MGMT persistent and polling modes | external contract | none | M1 | Tested cross-repository; implemented in MGMT. |
| Unchanged `esphome0.mcl` | yes | none | M1 | Hash locked in manifest. |
| Unchanged `esphome-blink.mcl` | yes | none | M1 | Hash locked in manifest. |

## Protocol and transport

| Capability | Upstream | Public API | Simulator | MGMT | Hardware | Target |
|---|---|---|---|---|---|---|
| Plain framing with limits | known | none | none | none | none | M1 |
| Noise transport | known | none | none | none | none | M1 |
| Hello and API version | known | none | none | none | none | M1 |
| Device information | known | none | none | none | none | M1 |
| Ping, disconnect, close | known | none | none | none | none | M1 |
| Entity discovery | known | none | none | none | none | M1 |
| State subscriptions | known | none | none | none | none | M1 |
| Bounded device logs | known | none | none | none | none | M1 |
| Client-owned reconnect | known | none | none | n/a | none | M2 |
| Home Assistant services/actions | known | none | none | none | none | M3 |
| Bluetooth proxy | known | none | none | none | none | backlog |
| Voice assistant | known | none | none | none | none | backlog |
| Camera streaming | known | none | none | none | none | backlog |

## Entity families

| Family | MGMT M1 need | Protocol known | Typed | Simulated | MGMT | Hardware | Target |
|---|---|---|---|---|---|---|---|
| Binary sensor | state | yes | no | no | no | no | M1 |
| Sensor | state | yes | no | no | no | no | M1 |
| Text sensor | state | yes | no | no | no | no | M1 |
| Switch | state/command | yes | no | no | no | no | M1 |
| Number | state/command | yes | no | no | no | no | M1 |
| Button | command seam | yes | no | no | no | no | M1 |
| Fan | conveyor state/command | yes | no | no | no | no | M1 |
| Light | conveyor color/state/command | yes | no | no | no | no | M1 |
| Select | no | yes | no | no | no | no | M2 |
| Text | no | yes | no | no | no | no | M3 |
| Climate | no | yes | no | no | no | no | M3 |
| Cover | no | yes | no | no | no | no | M3 |
| Lock | no | yes | no | no | no | no | M3 |
| Alarm control panel | no | yes | no | no | no | no | backlog |
| Media player | no | yes | no | no | no | no | backlog |
| Update | no | yes | no | no | no | no | backlog |

## Reference-client parity

`v1.1.0` is a comparison baseline, not evidence for this library. M1 covers only its MGMT-consumed subset. CLI, YAML configuration, mDNS implementation, Bluetooth proxy, services, and broad complex-entity commands are explicitly unsupported here until separately scheduled and evidenced.

## Compatibility dimensions

Each release reports Go version, OS/architecture, ESPHome oldest/current/development versions, transport mode, entity direction, simulator evidence, MGMT revision, reference-client migration surface, and hardware evidence separately.

The starting Go target is **Go 1.25.7**, matching the inspected MGMT default branch. The experimental MGMT branch moved to Go 1.26.1 with the reference client. Gate 0 must prove whether this module can avoid forcing that increase.
