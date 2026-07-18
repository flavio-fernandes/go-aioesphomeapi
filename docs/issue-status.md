# Issue status and closure rules

An empty issue tracker is not the goal. A trustworthy issue tracker is. An
issue closes only when its own acceptance evidence is present; adjacent working
features do not qualify. Future milestone epics stay open until that milestone
is intentionally scheduled and completed.

This snapshot was reconciled on 2026-07-17 against library `main` at
`6f954bc92a84b8a2bcb12acef5462b2445edfc08` and MGMT `feat/esphome` at
`90a172d09239925db5a527ee7b2a5edc383c08a3`. The append-only machine-readable
record is [`compatibility/mgmt-feat-esphome-review.json`](../compatibility/mgmt-feat-esphome-review.json).

## Evidence-complete issues

| Issue | Decision | Evidence |
|---|---|---|
| [#1](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/1) MGMT facade and typed contracts | close | ADR 0006, PR #27, the compatibility manifest, merged client API, and the final MGMT replacement establish and exercise the accepted boundary. |
| [#3](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/3) freeze MGMT behavior and migration diff | close | Immutable manifests, preserved `feat/esphome-richard87`, reviewed MCL hashes, PRs #30/#31, and MGMT PRs #1/#3 preserve the shared-session behavior and document the plaintext hardening. |
| [#4](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/4) pinned ESPHome protocol package | close | PR #28 pins source/tool/license hashes, reproducibly generates `pb`, inventories 148 unique IDs, and passes regeneration, validation, race, and vet evidence. |
| [#13](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/13) migrate MGMT driver | close | MGMT `feat/esphome` pins merged library `main`; targeted race/vet and all reviewed MCL simulator lanes pass; the reference implementation remains preserved for comparison. |

## Active Gate 0 and Milestone 1 work

| Issue | Exact remaining evidence |
|---|---|
| [#2](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/2) simulator contract | Add the missing reviewed scenario contract, explicit clock/seed decision, pushed-state timelines, slow-subscriber behavior, network shaping, and complete fidelity/cleanup assertions. |
| [#5](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/5) protocol inventory views | Enrich the machine-readable inventory with version/feature gates, MGMT need, reference parity, implemented behavior, and per-level evidence; validate every M1 message and unknown-value plan. |
| [#6](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/6) security and dependency budgets | Add explicit pending-operation, queue-saturation, deadline, cleanup, goroutine, and allocation budgets with tests and an automated dependency/license/vulnerability report. Reconcile the issue body's historical `x/crypto` and Go values with the accepted ADR. |
| [#7](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/7) public-repository controls | Verify and capture actual branch protection, approval/CODEOWNERS enforcement, stale-review dismissal, conversation resolution, secret scanning/push protection, private reporting, dependency updates, and emergency bypass through a safe test PR. |
| [#8](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/8) bounded framing and Noise | Add deterministic fragmented/coalesced and partial read/write tests, transport deadline/cancellation tests, allocation bounds, and explicit redaction assertions. Update the caller-resolution wording for accepted built-in `.local` mDNS. |
| [#9](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/9) complete MGMT entity slice | Add MGMT-level text-sensor state and button-command evidence plus missing/NaN, capability/type rejection, concurrent command/state, unknown-value, and slow-consumer tests. |
| [#10](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/10) simulator fault engine | Add delayed/fragmented network faults, slow-subscriber saturation, pushed-state timelines, explicit clock/seed semantics, and final cleanup/resource assertions. Existing drop, malformed, unknown, stall, polling, reconnect, and MCL evidence remains valid. |
| [#11](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/11) connection lifecycle | Implement and test device info and bounded client keepalive; add virtual-time state-machine, one-dial-owner, cancellation, goroutine-leak, callback-isolation, and command-interruption/no-replay evidence. |
| [#12](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/12) release-candidate verification | Add scheduled fuzzing, generated-drift CI, explicit allocation/goroutine budgets, automated dependency reporting, and an automatic pinned MGMT checkout lane. Local policy/race/vet/fuzz and current MGMT simulator lanes pass. |
| [#14](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/14) interactive conveyor demo | Complete the presenter story: pushed sensor changes, network interruption and safe stop, contradictory-sensor and slow-subscriber faults, responsibility display, presenter runbook, and sanitized physical recovery checklist. |
| [#15](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/15) conveyor firmware/workbench | Land the board-specific profile in the approved workbench repository with reviewed pins/power/entities and every local safety invariant; retain compile evidence and add the authorized flash/recovery checklist. Physical flashing remains separately authorized. |
| [#23](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/23) durable GitHub automation | The connected GitHub app now performs repository, issue, branch, commit, PR, check, and merge operations without exposing a token. A safe local Git/Actions fallback still needs an OS keyring or short-lived repository-scoped app credential; the invalid file-backed CLI credential must not be reused. |

## Deliberately open roadmap

Issues [#16](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/16)
through [#22](https://github.com/flavio-fernandes/go-aioesphomeapi/issues/22)
represent Milestones 2 through 6: broader typed APIs, compatibility reports,
complex entities, release audits, factory-scale validation, ecosystem breadth,
and the first tagged release. They are not M1 defect counts and should remain
open until their milestone is active.

## Recommended implementation order

1. Finish #2 so later simulator evidence has one accepted deterministic contract.
2. Enrich #5 while adding the missing MGMT entity evidence in #9.
3. Complete the simulator and lifecycle gaps in #10 and #11.
4. Close the security and release-candidate gates in #6, #8, and #12.
5. Enforce repository controls and durable fallback automation in #7 and #23.
6. Finish the interactive and workbench deliverables in #14 and #15.

This order requires no physical hardware until the explicitly authorized part
of #15.
