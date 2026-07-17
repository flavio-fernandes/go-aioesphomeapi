# Reference baseline audit

This is a factual architecture input, not a criticism of either project and not a support claim for this repository.

## Immutable snapshots

| Subject | Pinned revision | Role here |
|---|---|---|
| MGMT ESPHome work | [`flavio-fernandes/mgmt@8eab220`](https://github.com/flavio-fernandes/mgmt/commit/8eab220) | Application compatibility contract |
| Independent Go client release | [`Richard87/esphome-apiclient@982fb85`](https://github.com/Richard87/esphome-apiclient/commit/982fb85860e7214e3384e68cb69bf94b16a6985b) | Go API and behavior comparison |
| ESPHome protocol | Not pinned yet | Wire source of truth after Gate 0 protocol decision |

The MGMT comparison contains ten commits and introduces endpoint/resource/function behavior, a shared session wrapper, two MCL examples, tests, and a third-party driver. The machine-readable paths and hashes are in [`compatibility/mgmt-feat-esphome.json`](../compatibility/mgmt-feat-esphome.json).

At the inspected reference revision, repository history contains eight commits from one contributor and one tag, `v1.1.0`. That makes it valuable working software and a useful interoperability target, while still warranting conservative review before it becomes factory-control infrastructure.

## Observed dependency impact

The reference module's `go.mod` directly lists six modules:

| Module | Observed purpose |
|---|---|
| `github.com/flynn/noise` | Noise transport |
| `github.com/miekg/dns` | built-in `.local` mDNS resolution |
| `google.golang.org/protobuf` | Native API messages |
| `github.com/urfave/cli/v3` | bundled CLI |
| `gopkg.in/yaml.v3` | CLI configuration |
| `github.com/stretchr/testify` | tests |

In the inspected MGMT comparison, adding the reference client changes the Go directive from 1.25.7 to 1.26.1 and adds the client, Noise, and DNS modules to `go.mod`; `go.sum` gains nine lines. Some other requirements already existed in MGMT. Our M1 target removes the client and DNS additions, reuses MGMT's existing protobuf runtime where versions permit, adds only the approved Noise module, and preserves `.local` resolution with a bounded standard-library implementation.

The replacement and mDNS compatibility records now prove this target with real
MGMT builds, module-graph comparison, and unchanged-MCL runtime tests.

## What MGMT actually consumes

MGMT uses a narrow slice of the reference client:

- context-aware dial with client information and an optional Noise key;
- entity listing and registry access for six families;
- protobuf state callbacks for five families;
- switch, number, and button commands;
- log subscription;
- done notification and close.

MGMT deliberately disables the reference client's reconnect option because its own shared session owns reconnect and polling.

## Compatibility target

M1 does not require cloning every feature of the reference client. It requires:

| Area | M1 promise |
|---|---|
| MGMT `.mcl` files | Run without source changes unless a reviewed flaw is recorded |
| MGMT Go adapter | Import-path-only changes are the target; any additional delta is listed and justified |
| Required client methods | Source-compatible signatures where doing so does not preserve an unsafe behavior |
| Wire behavior | Interoperate from the official pinned protocol, not reference implementation internals |
| Security and bounds | May be stricter than the reference client; must remain transparent to valid MGMT use |
| Broader reference features | Tracked separately; no implied M1 support |

## Improvements that matter to MGMT

- Noise required by the normal configuration instead of plaintext when the key is accidentally omitted;
- bounded frame sizes, callback queues, pending requests, logs, and reconnect activity;
- context cancellation that terminates every blocking operation;
- callbacks dispatched away from the network read loop;
- deterministic device-side simulator and malformed-peer tests;
- stable entity-name/object-ID handling for current ESPHome;
- typed and redacted errors;
- no implicit command replay;
- a smaller runtime and transitive module graph;
- compatibility tests that compile and exercise the real MGMT branch.

## Explicit non-goals for the first slice

The bundled CLI, YAML loading, mDNS service browsing, Bluetooth proxy, and broad complex-entity coverage are not required by the existing MGMT branch. Direct `.local` A-record resolution is required and implemented; broader discovery can be evaluated later as an optional package or evidence-driven milestone. Its limits must remain visible in the support matrix.
