# Conveyor acceptance profile

The conveyor is a visible end-to-end acceptance system. It must exercise the same generic public API available to any Go application. No conveyor-specific type belongs in the core library or simulator engine.

## Sanitized hardware profile

- ESP-class device running ESPHome
- DRV8833 dual H-bridge driving a low-voltage geared DC motor
- two APDS9960 optical sensors on separate TCA9548A virtual I²C buses
- independent sensor interrupt inputs where the selected board and firmware support them
- motor supply sized for stall current, common ground where required by the driver design
- physical e-stop or power/enable interruption independent of MGMT and the network

Exact serial numbers, addresses, SSIDs, keys, camera images, bench layout, and private procurement records are not repository data.

ESPHome already provides building blocks for this profile: TCA9548A virtual buses, APDS9960 sensors, and an H-bridge fan abstraction compatible with DRV8833-style direction/speed control. The final firmware pinout is board-specific and stays in an explicitly approved workbench profile.

## Entity contract

The first firmware should advertise generic entities similar to:

| Purpose | ESPHome-facing family | Direction |
|---|---|---|
| Motor direction and speed | fan with H-bridge behavior; temporary template number only if the MGMT adapter needs staging | command and state |
| Entry presence | binary sensor | state |
| Exit presence | binary sensor | state |
| Optional optical telemetry | sensor | state |
| Local fault/ready indication | binary sensor or text sensor | state |
| Explicit reset, if safe | button | command |

Names are illustrative. Acceptance binds by stable identifiers recorded in a synthetic demo manifest, not by hard-coded friendly names in the library.

The two existing MGMT MCL examples remain unchanged compatibility fixtures. The conveyor is a new example and may add a generic fan resource; it does not redefine the older switch/number contract.

## Local firmware invariants

On boot, motor output is off. Firmware stops the motor without waiting for MGMT when any configured condition occurs: communications timeout, maximum continuous run time, contradictory or impossible sensor state, internal fault, or physical e-stop. Reconnection never resumes motion automatically.

## Demonstration story

1. Start MGMT, the Go client, and either the deterministic simulator or explicitly selected workbench device.
2. Show discovery and live state without moving hardware.
3. Request a transfer through MGMT desired state; show MGMT convergence and generic Native API commands.
4. Show entry/exit sensor events driving MGMT graph changes.
5. Introduce a network interruption. Local firmware stops; the client reconnects with bounded backoff; MGMT observes rather than blindly replaying motion.
6. Run the same scenario against the simulator with a contradictory-sensor fault and a slow-subscriber fault.
7. Display an evidence panel separating MGMT decisions, library transport/session behavior, ESPHome local safeguards, and physical signals.

The demo may be playful and visually rich. Its safety state must be boring, obvious, and locally enforced.
