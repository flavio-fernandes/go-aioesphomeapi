# Conveyor acceptance profile

The conveyor is a visible end-to-end acceptance system. It must exercise the same generic public API available to any Go application. No conveyor-specific type belongs in the core library or simulator engine.

To run it, see the [MGMT simulator demo](mgmt-simulator-demo.md). This document is the profile the demo implements.

## Sanitized hardware profile

- ESP-class device running ESPHome
- DRV8833 dual H-bridge driving a low-voltage geared DC motor
- two APDS9960 optical sensors on separate TCA9548A virtual I²C buses
- independent sensor interrupt inputs where the selected board and firmware support them
- motor supply sized for stall current, common ground where required by the driver design
- physical e-stop or power/enable interruption independent of MGMT and the network

Exact serial numbers, addresses, SSIDs, keys, camera images, bench layout, and private procurement records are not repository data.

ESPHome already provides building blocks for this profile: TCA9548A virtual buses, APDS9960 sensors, and an H-bridge fan abstraction compatible with DRV8833-style direction/speed control. The final firmware pinout is board-specific and stays in an explicitly approved workbench profile.

## Ownership rules

These three rules decide what belongs on the device and what belongs in MCL. They matter more than the entity list, because the entity list follows from them.

**One owner per actuator.** While a controller is connected, only the controller commands the motor and the light. The firmware has no opinion to converge against, so there is no fight over who stopped the belt. The firmware acts on the motor in exactly two cases: it lost the controller, so nobody is left to decide, and a run request went unanswered long enough that the controller looks wedged.

**Devices report facts, not commands.** The device publishes what its sensors observed. It does not publish a request for the belt to run, a classification of a brick, or a fault verdict. "A brick is on its way" is a fact, because somebody has to remember a brick that has left the entry beam and not yet reached the exit, and that is a hardware timing detail. "This brick is red" is not a fact; it is a policy decision about a threshold, and it lives in MCL.

**Export only what the controller reads.** An entity that MGMT never reads is chattiness with a wire format. Raw sensor channels stay internal to the device. The whole point of the demo is that it stays responsive, and it stays responsive because one brick costs four state messages rather than a continuous telemetry stream.

## Entity contract

The firmware exports exactly four entities, because those are the four MGMT uses:

| Entity | ESPHome-facing family | Direction | Meaning |
|---|---|---|---|
| `Brick En Route` | binary sensor (template) | state | a brick is on its way to the exit |
| `Exit Red Ratio` | sensor (template) | state | red share of the brick at the exit, or negative for no trustworthy reading |
| `Conveyor Motor` | fan with H-bridge behavior | command and state | the belt |
| `Status Light` | light, RGB, declaring named effects | command and state | what the rig is saying about the brick |

Two details in that table carry real weight.

The red ratio is one number that answers two questions: whether there is a brick worth judging, and what it looks like. A negative value means no trustworthy reading, which covers an empty exit, a brick lifted off mid-capture, and a capture too dark to mean anything. A genuine ratio is a share of a positive total, so it can never be negative, and the controller never has to correlate a presence entity with a measurement entity that arrives in a separate message.

The light declares its animations and never selects one. MGMT picks by name. That keeps the animation on the device, where per-frame timing belongs, and the decision in MCL, where the demo can change it without a reflash.

The entry and exit presence sensors, the raw red/green/blue/clear channels, the run-request mirror, the enable switch, the speed number, the reset button, and the status text sensor were all exported by an earlier revision of this profile. None of them were read by the MCL. They are internal to the device now, or gone.

The two existing MGMT MCL examples remain unchanged compatibility fixtures. The conveyor is a separate example and may add a generic fan resource; it does not redefine the older switch/number contract.

## Local firmware invariants

On boot, motor output is off, and the device publishes its starting values rather than leaving them unpublished. Firmware stops the motor without waiting for MGMT when any configured condition occurs: communications timeout, maximum continuous run time, contradictory or impossible sensor state, internal fault, or physical e-stop. Reconnection never resumes motion automatically.

Losing the controller must also clear the run request, not only stop the belt. A device that keeps reporting a brick en route after the controller disappears will be believed by the next controller that connects, which then starts the belt with nothing on it.

The jam timeout has two stages on purpose. It first withdraws the run request, which is a fact, and lets the controller stop the belt under its own policy so ownership is never in question. Only if the belt is still running after a further grace period does the firmware stop the motor itself, and that is the one override outside a disconnect.

## Demonstration story

1. Start MGMT, the Go client, and either the deterministic simulator or an explicitly selected workbench device.
2. Show discovery and live state without moving hardware.
3. Drop a brick on the entry. The device reports a brick en route; MGMT starts the belt and blinks the light.
4. The brick reaches the exit. The device reports that nothing is en route; MGMT stops the belt.
5. The device publishes one averaged color reading. MGMT classifies it and the light goes solid red for a red brick or rainbow for anything else.
6. Lift the brick off. The reading goes negative and the light returns to idle white.
7. Change the red threshold in the MCL and save. The next brick is classified by the new rule, with no reflash and no restart. This is the point of the demo.
8. Introduce a network interruption. Local firmware stops the belt and clears the run request; the client reconnects with bounded backoff; MGMT observes rather than blindly replaying motion.
9. Run the same scenario against the simulator with a contradictory-sensor fault and a slow-subscriber fault.
10. Display an evidence panel separating MGMT decisions, library transport/session behavior, ESPHome local safeguards, and physical signals.

Holding a brick against the exit sensor without using the belt at all turns the rig into a color tester: nothing is en route, so the belt never moves, and the light names the brick. That is the practical way to pick the threshold for a given set of bricks and a given amount of ambient light.

The demo may be playful and visually rich. Its safety state must be boring, obvious, and locally enforced.
