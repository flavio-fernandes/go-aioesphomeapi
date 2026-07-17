# Agent operating contract

Read `README.md`, `CHEATSHEET.md`, `docs/architecture.md`, `docs/mgmt-integration.md`, `docs/dependency-policy.md`, `docs/documentation-style.md`, `docs/security-threat-model.md`, `docs/support-matrix.md`, and the nearest nested `AGENTS.md` before changing files.

## Non-negotiable rules

- Keep the core library generic. Conveyor and MGMT domain behavior belongs in adapters or examples.
- Treat `compatibility/mgmt-feat-esphome.json` as an immutable external contract. Existing pinned MCL files must remain byte-identical in cross-repository tests unless MGMT accepts a documented defect fix.
- Keep the MGMT migration mechanically small. Do not absorb MGMT's session pool, polling, convergence, or outage logic into this module.
- Treat ESPHome devices and simulator peers as untrusted network inputs.
- Do not add secrets, personal data, private URLs, local absolute paths, device identifiers, camera media, packet captures, or firmware binaries.
- Do not copy reference-client implementation code. Follow `docs/provenance.md` for protocol material.
- Do not claim a capability until the support matrix contains test evidence at the claimed level.
- Do not add a runtime dependency without the dependency-policy evidence, ADR, module-graph delta, and MGMT impact review. Never implement cryptographic primitives locally.
- Keep user documentation friendly and honest. Test copy/paste commands, list prerequisites, show expected results, and update `CHEATSHEET.md` with user-facing behavior.
- Make the simulator the default first-use path. Never make real hardware, secrets, flashing, cameras, motors, or actuators a beginner default.
- Do not flash hardware, energize a motor, move an actuator, or use a camera unless the current task explicitly authorizes it and the `operate-esp-workbench` skill's preflight passes.
- Use small pull requests tied to one issue and milestone. Preserve generated/handwritten boundaries.

## Required checks

Run `./tools/validate-repo.sh`. Once Go packages exist, also run the repository's documented format, vet, race, fuzz-smoke, and test commands. Never bypass a failing security or provenance check.

## Project skills

- `$sync-esphome-protocol` for upstream protocol changes.
- `$run-device-simulator` for simulator scenarios and faults.
- `$operate-esp-workbench` for firmware, flash, serial, and camera work.
- `$prepare-public-release` before changing repository visibility or publishing a release.
