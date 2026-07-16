# Support matrix

This document is the sole source of truth for compatibility claims. A row can be present in upstream ESPHome and still have no library support.

## Evidence levels

| Level | Meaning |
|---|---|
| `untracked` | Not yet inventoried from the pinned protocol. |
| `known` | Present in the pinned protocol inventory; generated types alone qualify only for this level. |
| `typed` | Stable public Go types and validation exist. |
| `simulated` | Deterministic client/server integration tests cover success and negative paths. |
| `hardware` | Tested against a recorded ESPHome release and synthetic hardware profile. |
| `production` | Meets security, race, load, observability, and compatibility gates. |

`planned` is roadmap intent, not an evidence level.

## Protocol and transport capabilities

| Capability | Upstream inventory | Public Go API | Simulator | Hardware | Target milestone | Notes |
|---|---|---|---|---|---|---|
| TCP framing | untracked | none | none | none | M1 | Frame and allocation limits required. |
| Noise transport | untracked | none | none | none | M1 | Required production default. |
| Hello/connect handshake | untracked | none | none | none | M1 | Includes API version capture. |
| Device information | untracked | none | none | none | M1 | Synthetic identifiers in tests. |
| Ping/keepalive/disconnect | untracked | none | none | none | M1 | Deterministic timeout tests. |
| Entity discovery | untracked | none | none | none | M1 | Inventory generated during first sync. |
| State subscriptions | untracked | none | none | none | M1 | Bounded fan-out semantics. |
| Home Assistant services/actions | untracked | none | none | none | M3 | Exact terminology follows pinned protocol. |
| Device logs | untracked | none | none | none | M3 | Redaction and rate limiting required. |
| Bluetooth proxy | untracked | none | none | none | backlog | Large surface; inventory first. |
| Voice assistant | untracked | none | none | none | backlog | Not required for MGMT integration. |
| Camera streaming | untracked | none | none | none | backlog | Privacy and resource limits required. |

## Entity families

| Entity family | Protocol known | Typed | Simulated | Hardware | Target milestone | First consumer |
|---|---|---|---|---|---|---|
| Binary sensor | no | no | no | no | M1 | Conveyor position sensing |
| Sensor | no | no | no | no | M1 | Generic telemetry |
| Switch | no | no | no | no | M1 | Generic actuator |
| Fan | no | no | no | no | M1 | H-bridge motor acceptance profile |
| Number | no | no | no | no | M2 | Tunable setpoint/servo profile |
| Button | no | no | no | no | M2 | One-shot action |
| Light | no | no | no | no | M2 | Demo feedback and generic lighting |
| Select | no | no | no | no | M2 | Enumerated desired state |
| Text sensor | no | no | no | no | M2 | Diagnostic state |
| Text | no | no | no | no | M3 | Writable text |
| Climate | no | no | no | no | M3 | Complex entity example |
| Cover | no | no | no | no | M3 | Position-capable actuator |
| Lock | no | no | no | no | M3 | Security-sensitive command policy |
| Alarm control panel | no | no | no | no | backlog | Security-sensitive entity. |
| Media player | no | no | no | no | backlog | Not required for MGMT integration. |
| Update | no | no | no | no | backlog | Never auto-install without policy. |

## Compatibility dimensions

Each release must publish a separate tested matrix for:

- Go versions, beginning with the version required by current MGMT;
- operating systems and architectures;
- ESPHome firmware releases: oldest supported, current stable, and current development snapshot;
- secure transport modes;
- entity family and command/state direction;
- simulator and real-hardware evidence.

The initial Go baseline is **Go 1.25.7**, matching the inspected MGMT module. Gate 0 must confirm whether the library can responsibly support an older Go version without compromising MGMT-first development.
