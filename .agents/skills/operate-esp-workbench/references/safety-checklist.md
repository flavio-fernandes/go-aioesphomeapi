# Physical workbench preflight

Confirm all items before an authorized physical operation:

- The active task names the repository, target/slot, action, and allowed observation method.
- The board and wiring are visually matched to the selected profile.
- Motor supply and logic supply satisfy the driver design; common ground is correct where required.
- A physical e-stop or independent motor power/enable interruption is reachable and tested.
- Motor output is off at boot and after connection loss.
- Maximum continuous run time and contradictory-sensor stop are configured locally.
- Flashing cannot target another attached board through auto-discovery.
- Camera use, if explicitly authorized, avoids people, screens, labels, and unrelated bench areas.
- Logs and captures use sanitized, ignored local storage.
- A simulator-first test has already passed for the same expected protocol behavior.

Reference workbenches:

- `flavio-fernandes/crowpanel-esphome`
- `flavio-fernandes/esp-codex-platform`

Read their current local instructions before using their tooling; this document does not override them.
