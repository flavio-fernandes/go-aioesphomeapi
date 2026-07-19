# Issue status and closure rules

An empty issue tracker is not the goal. A trustworthy issue tracker is. An
issue closes only when its own acceptance evidence is present; adjacent working
features do not qualify. Future milestone epics stay open until that milestone
is intentionally scheduled and completed.

This snapshot was reconciled on 2026-07-18 against library implementation
`main` at `091b9af4f600dfa98b1ebea169265d2afc254047` and MGMT `feat/esphome` at
`08514da10969b0188a1127c3938790139e7fa0c6`. The latest append-only
machine-readable record is
[`compatibility/mgmt-upstream-pr-961-ready.json`](../compatibility/mgmt-upstream-pr-961-ready.json).

## Evidence-complete issues

| Issue | Decision | Evidence |
|---|---|---|
| [#1](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/1) MGMT facade and typed contracts | close | ADR 0006, PR #27, the compatibility manifest, merged client API, and the final MGMT replacement establish and exercise the accepted boundary. |
| [#2](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/2) deterministic simulator contract | close | ADR 0004 and `references/scenario-contract.md` accept exact time/seed, state, command, network-shaping, slow-subscriber, cleanup, and fidelity semantics. Issue #10 completes the M1 simulator-owned rows; #11 explicitly retains broader client lifecycle work. |
| [#3](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/3) freeze MGMT behavior and migration diff | close | Immutable manifests, preserved `feat/esphome-richard87`, reviewed MCL hashes, PRs #30/#31, and MGMT PRs #1/#3 preserve the shared-session behavior and document the plaintext hardening. |
| [#4](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/4) pinned ESPHome protocol package | close | PR #28 pins source/tool/license hashes, reproducibly generates `pb`, inventories 148 unique IDs, and passes regeneration, validation, race, and vet evidence. |
| [#5](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/5) protocol inventory views | close | The reviewed annotation schema and generated inventory classify all 148 unique IDs by version/feature gate, entity family, MGMT need, reference parity, public behavior, and every evidence level. All 33 M1 messages are accounted for: 31 implemented messages have typed/simulated proof and DeviceInfo request/response remain explicitly known-only under #11. Unknown IDs/enums/fields have separate status and test plans. |
| [#8](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/8) bounded framing and Noise | close | Plain and Noise packet tests deterministically prove fragmented/coalesced reads and partial writes; hostile lengths cannot raise the 64 KiB plaintext allocation ceiling; dial, Noise handshake, and Hello preserve deadline/cancellation causes; key configuration, handshake, and peer-controlled rejection paths assert redaction. The built-in standard-library `.local` resolver and injected-dialer bypass match ADR 0010. `go.mod`, `go.sum`, the two-direct/two-transitive module graph, and Go 1.25.12 remain unchanged. |
| [#13](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/13) migrate MGMT driver | close | MGMT `feat/esphome` pins merged library `main`; targeted race/vet and all reviewed MCL simulator lanes pass; the reference implementation remains preserved for comparison. |
| [#32](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/32) duplicate entity-list completion panic | close | Entity-list completion is consumed atomically; duplicate and spurious completion tests traverse the real simulator wire path under the race detector and the connection remains usable. |
| [#33](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/33) unbounded Hello | close | One context/timeout budget now covers dial, Noise, and Hello. Plaintext and Noise silent-peer tests prove bounded return and preserve `ErrHello` plus cancellation/deadline causes. |
| [#34](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/34) unknown-message compatibility | close | ADR 0008 now requires bounded unknown IDs to be skipped; a real-wire simulator test proves discovery and Ping continue, while malformed known protobuf remains fatal. |
| [#35](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/35) Noise key rejection diagnostics | close | `ErrNoiseKeyRejected` remains within the broad handshake category; wire and public-client tests prove target-aware, capped, printable diagnostics with no key material. |
| [#36](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/36) robustness batch | close | Tests cover the corrected Noise limit and message-type category, bounded mDNS retransmission/response validation, Hello name checks on plaintext, no forced log dump, context-bound Ping, observable simulator overflow, and wrapped accept errors. Automatic keepalive remains correctly tracked by #11. |
| [#39](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/39) continuous vulnerability monitoring | close | The official scanner is version-pinned in one repository script, runs for pull requests, `main` pushes, and weekly schedules, and has documented fail-closed reachable findings plus explicit module-level triage. The workflow path and clean result are tested in CI. |
| [#44](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/44) mDNS retransmit deadline | close | Retry scheduling now uses read-only UDP deadlines; a real loopback socket regression proves retransmit writes survive the first timeout and the lookup consumes its overall budget. All unchanged MGMT MCL acceptance lanes pass through multicast `.local` resolution. |
| [#47](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/47) simulator validation and reconnect decisions | close | ADR 0013 preserves `New`, adds typed `Scenario.Validate` plus deferred pre-connection rejection, permits zero seed without randomized actions, and makes issue #10's latest-state implementation contingent on an append-only before/after MGMT reconnect re-baseline. Simulator tests prove typed/redacted errors, zero-seed compatibility, no connection side effects, and defensive copies. |
| [#52](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/52) scenario-field completeness guard | close | An exact reflected `Scenario` field inventory fails on additions with instructions to extend validation and cloning. A fully populated clone test proves every current mutable field preserves its value without retaining source aliases. |
| [#56](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/56) duplicate initial-state keys | close | Both supported initial-state field names reject same-family duplicate keys with typed, secret-safe structural indexes before any dial or listener work. Tests preserve invalid-type classification and allow the same numeric key across distinct state families. |
| [#72](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/72) wrapped Noise key echo | close | Wire and public-dial tests prove CR/LF-wrapped base64 accepted by Go's decoder is matched before printable sanitization; the complete peer reason is redacted while handshake and rejected-key categories remain intact. The accepted-representation invariant is recorded in the release skill. |
| [#10](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/10) simulator fault engine | close | Manual time/latest-state reconnects, the pinned ADR 0013 MGMT re-baseline, command expectations, slow-subscriber saturation, all named M1 fault classes, bounded network writing, fixed scenario/session/listener limits, typed saturation, and `WaitForIdle` owned-resource cleanup have race-tested evidence. Zero seed remains valid because no randomized action exists; the field-completeness guard requires validation before a future randomized field can run. |

## Active Milestone 1 work

| Issue | Exact remaining evidence |
|---|---|
| [#6](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/6) security and dependency budgets | Callback queue saturation is now deterministic and bounded. Add explicit pending-operation, deadline, broader cleanup/goroutine/allocation budgets with tests and an automated dependency/license/vulnerability report. Reconcile the issue body's historical `x/crypto` and Go values with the accepted ADR. |
| [#7](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/7) public-repository controls | `main` requires PRs, strict `go`/`validate` plus an explicitly requested exact-head `codex-review` status, stale-review dismissal, conversation resolution for administrators, and forbids force-push/deletion. Automatic Codex reviews are disabled while manual review remains available. Verify secret scanning/push protection, private reporting, dependency updates, emergency bypass, and the future independent-approval/CODEOWNERS gate before closing. |
| [#9](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/9) complete MGMT entity slice | Add MGMT-level text-sensor state and button-command evidence plus missing/NaN, capability/type rejection, concurrent command/state, unknown-value, and slow-consumer tests. |
| [#11](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/11) connection lifecycle | Implement and test device info and bounded client keepalive; add virtual-time state-machine, one-dial-owner, cancellation, goroutine-leak, callback-isolation, and command-interruption/no-replay evidence. |
| [#12](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/12) release-candidate verification | PR #69 lands all five lanes: the three-target pull-request fuzz smoke plus a finite scheduled/dispatch extended fuzz job, generated-drift CI, documented owned-goroutine budgets (three per connected client, one per accepted simulator connection, zero after close) with allocation ceilings, a fail-closed dependency/module/license report, and the pinned-MGMT lane using MGMT's `noaugeas novirt` tags. Hosted pull-request runs of the policy, Go, and generated-drift jobs pass; all three 10-minute extended fuzz targets pass locally; the MGMT lane passes end-to-end locally. Remaining: the PR's hosted MGMT lane check, then first scheduled or dispatched hosted executions of the extended fuzz and MGMT lanes from `main`, then a small closure follow-up recording them. |
| [#14](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/14) interactive conveyor demo | Complete the presenter story: pushed sensor changes, network interruption and safe stop, contradictory-sensor and slow-subscriber faults, responsibility display, presenter runbook, and sanitized physical recovery checklist. |
| [#15](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/15) conveyor firmware/workbench | Land the board-specific profile in the approved workbench repository with reviewed pins/power/entities and every local safety invariant; retain compile evidence and add the authorized flash/recovery checklist. Physical flashing remains separately authorized. |
| [#23](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/23) durable GitHub automation | The connected GitHub app now performs repository, issue, branch, commit, PR, check, and merge operations without exposing a token. A safe local Git/Actions fallback still needs an OS keyring or short-lived repository-scoped app credential; the invalid file-backed CLI credential must not be reused. |

## Remaining blocking review findings

| Issue | Required outcome |
|---|---|
| [#40](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/40) MGMT disconnect attribution | Extend the MGMT driver boundary to surface the library's sanitized `CloseReason`; prove persistent and applicable polling paths report asynchronous encrypted connection loss and queue overflow without misreporting deliberate shutdown. Closing #13 remains valid because its original replacement acceptance criteria are complete; this is newly identified operability work. |

## Deliberately open roadmap

Issues [#16](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/16)
through [#22](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/22)
represent Milestones 2 through 6: broader typed APIs, compatibility reports,
complex entities, release audits, factory-scale validation, ecosystem breadth,
and the first tagged release. They are not M1 defect counts and should remain
open until their milestone is active.

## Recommended implementation order

1. Add the missing MGMT entity and unknown-value evidence in #9 using the protocol inventory as the claim ledger.
2. Complete the remaining client lifecycle contract in #11 and surface #40 so
   MGMT can attribute asynchronous connection loss without coupling its
   session layer to the concrete library client.
3. Close the remaining security and release-candidate gates in #6 and #12.
4. Enforce repository controls and durable fallback automation in #7 and #23.
5. Finish the interactive and workbench deliverables in #14 and #15.

This order requires no physical hardware until the explicitly authorized part
of #15.
