# Issue status and closure rules

An empty issue tracker is not the goal. A trustworthy issue tracker is. An
issue closes only when its own acceptance evidence is present; adjacent working
features do not qualify. Future milestone epics stay open until that milestone
is intentionally scheduled and completed.

This snapshot was reconciled on 2026-07-18 against library `main` at
`3655bef5c0a9d871ca5ee262665afddcf83158a7` and MGMT `feat/esphome` at
`eb8953a4be99147ae4cc7f48b6e3b6939b426ab2`. The latest append-only
machine-readable record is
[`compatibility/mgmt-feat-esphome-simulator-timeline-postmerge.json`](../compatibility/mgmt-feat-esphome-simulator-timeline-postmerge.json).

## Evidence-complete issues

| Issue | Decision | Evidence |
|---|---|---|
| [#1](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/1) MGMT facade and typed contracts | close | ADR 0006, PR #27, the compatibility manifest, merged client API, and the final MGMT replacement establish and exercise the accepted boundary. |
| [#2](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/2) deterministic simulator contract | close | ADR 0004 and `references/scenario-contract.md` accept exact time/seed, state, command, network-shaping, slow-subscriber, cleanup, and fidelity semantics. Existing real-wire peers and MGMT lanes establish the implementation baseline; #10/#11 explicitly retain unimplemented contract rows. |
| [#3](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/3) freeze MGMT behavior and migration diff | close | Immutable manifests, preserved `feat/esphome-richard87`, reviewed MCL hashes, PRs #30/#31, and MGMT PRs #1/#3 preserve the shared-session behavior and document the plaintext hardening. |
| [#4](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/4) pinned ESPHome protocol package | close | PR #28 pins source/tool/license hashes, reproducibly generates `pb`, inventories 148 unique IDs, and passes regeneration, validation, race, and vet evidence. |
| [#5](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/5) protocol inventory views | close | The reviewed annotation schema and generated inventory classify all 148 unique IDs by version/feature gate, entity family, MGMT need, reference parity, public behavior, and every evidence level. All 33 M1 messages are accounted for: 31 implemented messages have typed/simulated proof and DeviceInfo request/response remain explicitly known-only under #11. Unknown IDs/enums/fields have separate status and test plans. |
| [#13](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/13) migrate MGMT driver | close | MGMT `feat/esphome` pins merged library `main`; targeted race/vet and all reviewed MCL simulator lanes pass; the reference implementation remains preserved for comparison. |
| [#32](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/32) duplicate entity-list completion panic | close | Entity-list completion is consumed atomically; duplicate and spurious completion tests traverse the real simulator wire path under the race detector and the connection remains usable. |
| [#33](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/33) unbounded Hello | close | One context/timeout budget now covers dial, Noise, and Hello. Plaintext and Noise silent-peer tests prove bounded return and preserve `ErrHello` plus cancellation/deadline causes. |
| [#34](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/34) unknown-message compatibility | close | ADR 0008 now requires bounded unknown IDs to be skipped; a real-wire simulator test proves discovery and Ping continue, while malformed known protobuf remains fatal. |
| [#35](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/35) Noise key rejection diagnostics | close | `ErrNoiseKeyRejected` remains within the broad handshake category; wire and public-client tests prove target-aware, capped, printable diagnostics with no key material. |
| [#36](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/36) robustness batch | close | Tests cover the corrected Noise limit and message-type category, bounded mDNS retransmission/response validation, Hello name checks on plaintext, no forced log dump, context-bound Ping, observable simulator overflow, and wrapped accept errors. Automatic keepalive remains correctly tracked by #11. |
| [#39](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/39) continuous vulnerability monitoring | close | The official scanner is version-pinned in one repository script, runs for pull requests, `main` pushes, and weekly schedules, and has documented fail-closed reachable findings plus explicit module-level triage. The workflow path and clean result are tested in CI. |
| [#44](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/44) mDNS retransmit deadline | close | Retry scheduling now uses read-only UDP deadlines; a real loopback socket regression proves retransmit writes survive the first timeout and the lookup consumes its overall budget. All unchanged MGMT MCL acceptance lanes pass through multicast `.local` resolution. |
| [#47](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/47) simulator validation and reconnect decisions | close | ADR 0013 preserves `New`, adds typed `Scenario.Validate` plus deferred pre-connection rejection, permits zero seed without randomized actions, and makes issue #10's latest-state implementation contingent on an append-only before/after MGMT reconnect re-baseline. Simulator tests prove typed/redacted errors, zero-seed compatibility, no connection side effects, and defensive copies. |

## Active Milestone 1 work

| Issue | Exact remaining evidence |
|---|---|
| [#6](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/6) security and dependency budgets | Add explicit pending-operation, queue-saturation, deadline, cleanup, goroutine, and allocation budgets with tests and an automated dependency/license/vulnerability report. Reconcile the issue body's historical `x/crypto` and Go values with the accepted ADR. |
| [#7](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/7) public-repository controls | Verify and capture actual branch protection, approval/CODEOWNERS enforcement, stale-review dismissal, conversation resolution, secret scanning/push protection, private reporting, dependency updates, and emergency bypass through a safe test PR. |
| [#8](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/8) bounded framing and Noise | Add deterministic fragmented/coalesced and partial read/write tests, transport deadline/cancellation tests, allocation bounds, and explicit redaction assertions. Update the caller-resolution wording for accepted built-in `.local` mDNS. |
| [#9](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/9) complete MGMT entity slice | Add MGMT-level text-sensor state and button-command evidence plus missing/NaN, capability/type rejection, concurrent command/state, unknown-value, and slow-consumer tests. |
| [#10](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/10) simulator fault engine | The manual clock/latest-state timeline, ADR 0013 pinned MGMT re-baseline, and ordered command expectations are complete. Add conditional seed validation when randomized actions arrive, delayed/fragmented/coalesced shaping, slow-subscriber saturation, and owned-resource cleanup assertions. |
| [#11](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/11) connection lifecycle | Implement and test device info and bounded client keepalive; add virtual-time state-machine, one-dial-owner, cancellation, goroutine-leak, callback-isolation, and command-interruption/no-replay evidence. |
| [#12](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/12) release-candidate verification | Add scheduled fuzzing, generated-drift CI, explicit allocation/goroutine budgets, automated dependency reporting, and an automatic pinned MGMT checkout lane. Local policy/race/vet/fuzz and current MGMT simulator lanes pass. |
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
2. Complete the accepted simulator and lifecycle contract in #10 and #11, and surface #40
   so MGMT can attribute asynchronous connection loss without coupling its
   session layer to the concrete library client.
3. Close the remaining security and release-candidate gates in #6, #8, and #12.
4. Enforce repository controls and durable fallback automation in #7 and #23.
5. Finish the interactive and workbench deliverables in #14 and #15.

This order requires no physical hardware until the explicitly authorized part
of #15.
