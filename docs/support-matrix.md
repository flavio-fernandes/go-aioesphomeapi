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

Every implementation column is `none` today. `planned` is roadmap intent, not evidence.

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
| Plain framing with limits | untracked | none | none | none | none | M1 |
| Noise transport | untracked | none | none | none | none | M1 |
| Hello and API version | untracked | none | none | none | none | M1 |
| Device information | untracked | none | none | none | none | M1 |
| Ping, disconnect, close | untracked | none | none | none | none | M1 |
| Entity discovery | untracked | none | none | none | none | M1 |
| State subscriptions | untracked | none | none | none | none | M1 |
| Bounded device logs | untracked | none | none | none | none | M1 |
| Client-owned reconnect | untracked | none | none | n/a | none | M2 |
| Home Assistant services/actions | untracked | none | none | none | none | M3 |
| Bluetooth proxy | untracked | none | none | none | none | backlog |
| Voice assistant | untracked | none | none | none | none | backlog |
| Camera streaming | untracked | none | none | none | none | backlog |

## Entity families

| Family | MGMT M1 need | Protocol known | Typed | Simulated | MGMT | Hardware | Target |
|---|---|---|---|---|---|---|---|
| Binary sensor | state | no | no | no | no | no | M1 |
| Sensor | state | no | no | no | no | no | M1 |
| Text sensor | state | no | no | no | no | no | M1 |
| Switch | state/command | no | no | no | no | no | M1 |
| Number | state/command | no | no | no | no | no | M1 |
| Button | command seam | no | no | no | no | no | M1 |
| Fan | conveyor state/command | no | no | no | no | no | M1 |
| Light | no | no | no | no | no | no | M2 |
| Select | no | no | no | no | no | no | M2 |
| Text | no | no | no | no | no | no | M3 |
| Climate | no | no | no | no | no | no | M3 |
| Cover | no | no | no | no | no | no | M3 |
| Lock | no | no | no | no | no | no | M3 |
| Alarm control panel | no | no | no | no | no | no | backlog |
| Media player | no | no | no | no | no | no | backlog |
| Update | no | no | no | no | no | no | backlog |

## Reference-client parity

`v1.1.0` is a comparison baseline, not evidence for this library. M1 covers only its MGMT-consumed subset. CLI, YAML configuration, mDNS implementation, Bluetooth proxy, services, and broad complex-entity commands are explicitly unsupported here until separately scheduled and evidenced.

## Compatibility dimensions

Each release reports Go version, OS/architecture, ESPHome oldest/current/development versions, transport mode, entity direction, simulator evidence, MGMT revision, reference-client migration surface, and hardware evidence separately.

The starting Go target is **Go 1.25.7**, matching the inspected MGMT default branch. The experimental MGMT branch moved to Go 1.26.1 with the reference client. Gate 0 must prove whether this module can avoid forcing that increase.
