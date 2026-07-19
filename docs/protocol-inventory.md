# Protocol compatibility inventory

The protocol inventory answers a narrow but important question: what does each
message in the pinned ESPHome Native API mean for this library today?

It does not treat generated Go code as implemented behavior. A message can be
known upstream while still having no handwritten API, simulator proof, MGMT
proof, or hardware proof.

## Quick check

You need the supported Go toolchain and must run these commands from the
repository root.

```bash
go run ./cmd/protocol-inventory -summary
go run ./cmd/protocol-inventory -check protocol/inventory.json
```

Expected output includes:

```text
ESPHome 2026.7.0: 148 unique messages, 33 M1 accounted (33 implemented), 115 generated-only
protocol inventory is current: 148 unique messages
```

These commands use only committed inputs. They do not contact GitHub, a device,
or the local network.

## The two inventory files

- [`protocol/inventory.annotations.json`](../protocol/inventory.annotations.json)
  is the small, reviewed compatibility input. Maintainers classify M1 messages,
  evidence profiles, and unknown-value plans here.
- [`protocol/inventory.json`](../protocol/inventory.json) is the complete,
  generated view. Every pinned message has the same explicit fields, including
  messages that are only known.

The generated view records:

| Field | Meaning |
|---|---|
| `id`, `name`, `direction` | Canonical wire identity from the pinned protobuf definition. |
| `version_gate` | The earliest version supported by evidence. `not_declared_upstream` with a `null` minimum means the pinned protobuf did not declare an introduction version; it is not a universal-version claim. |
| `feature_gate` | ESPHome compile-time `ifdef`, or `always` when none is declared. |
| `entity_family` | Generic device family or protocol area. |
| `milestone` | Delivery target. M1 includes two known-only device-info messages still owned by issue #11; otherwise unscheduled rows say `not_scheduled`. |
| `mgmt_required` | Whether the reviewed MGMT/conveyor contract needs the message. |
| `reference_parity` | Relationship to the pinned Go reference baseline, without using it as wire truth. |
| `public_behavior` | Handwritten behavior, or `generated_only`. |
| `evidence` | Separate `known`, `typed`, `simulated`, `mgmt`, `hardware`, and `production` source lists. |
| `notes` | A short limitation or operational explanation. |

Empty evidence arrays are deliberate. They prevent a generated type, a passing
simulator, or one hardware profile from silently becoming a broader claim.

## Unknown future values

The top-level `unknown_values` object keeps message-ID, enum-value, and unknown
field behavior visible. Each row has a status, intended behavior, test plan,
and current evidence. Unknown message IDs are verified today. Unknown M1 enum
values and fields remain explicit planned tests; they do not receive evidence
until those tests exist.

## Safe update workflow

For a compatibility-only correction, edit the annotation file and regenerate
only the inventory:

```bash
go run ./cmd/protocol-inventory -output protocol/inventory.json
go test ./cmd/protocol-inventory
git diff -- protocol/inventory.annotations.json protocol/inventory.json
```

For a new ESPHome release or protobuf change, follow the repository's
`sync-esphome-protocol` skill and pinned generator workflow. Never hand-edit the
generated inventory, copy a reference client's implementation, or raise an
evidence level without a committed proof source.
