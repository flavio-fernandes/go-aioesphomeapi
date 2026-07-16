---
name: operate-esp-workbench
description: Prepare and perform explicitly authorized ESPHome workbench builds, flashes, serial observation, camera observation, and physical acceptance checks. Use only for a named device/slot and an active hardware task; never use this workflow to infer permission for physical action.
---

# Operate ESP Workbench

Firmware, flashing, cameras, and actuators cross a physical trust boundary. Follow the checklist exactly.

## Workflow

1. Read `references/safety-checklist.md`, `docs/security-threat-model.md`, and `docs/conveyor-demo.md`.
2. Identify the approved workbench repository, target alias/slot, board profile, observation method, and requested action. Never infer a USB device, serial port, IP address, camera, or actuator target.
3. Inspect repository-local `AGENTS.md` and skills in the selected workbench before commands. Its closer instructions override this generic workflow.
4. Keep secrets in the workbench's ignored local mechanism. Rendered firmware, logs, images, addresses, serial numbers, and credentials never enter this repository.
5. Build before connecting to hardware. Review warnings, pin assignments, motor safe-boot behavior, communications timeout, maximum run time, sensor-fault stop, and e-stop path.
6. Pause before any flash, erase, reset, camera activation, motor power, or actuator movement unless that exact action and target are explicitly authorized in the active task.
7. Perform one bounded action. Observe through the least invasive approved channel. Stop immediately on unexpected movement, heat, noise, current, sensor contradiction, or loss of the safety channel.
8. Return the bench to safe state: motor power disabled, process/serial/camera sessions closed, and temporary secrets or rendered artifacts left only in ignored storage.
9. Record sanitized firmware revision, ESPHome release, test case, outcome, and reviewer in hardware evidence. Do not record unique device or network data.

## Hard stops

- No physical e-stop or independent motor-power interruption.
- Local firmware stop invariants are absent or untested.
- Target identity is ambiguous.
- The requested operation would expose private media or secrets.
- A simulator can answer the question without hardware.
