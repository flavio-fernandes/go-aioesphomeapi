# ADR 0014: One owner per actuator, and export only what the controller reads

- Status: accepted
- Date: 2026-07-30

## Context

The conveyor scenario advertised thirteen entities: a fan, an RGB light, three
binary sensors, four raw colour sensors, a switch, a number, a button, and a text
sensor. The MGMT conveyor MCL read four of them. The other nine existed because
the acceptance profile listed them as illustrative, not because any controller
used them.

That was not only waste. On the bench it was a correctness problem in two
distinct ways.

First, both sides steered the motor. The firmware stopped the belt on arrival
and MGMT stopped it too, from its own policy. When they disagreed about who had
stopped it, a brick placed after one was removed never restarted the belt. The
observable was a wedged demo; the cause was that the motor had two owners and no
tiebreak.

Second, the exported raw colour channels made the controller correlate values
that arrive as separate messages. A presence sensor and four channel sensors
describing the same brick can be observed in any order and at any spacing, so
MGMT had to decide when it had a complete picture. That is a hard problem the
device can make disappear.

The exported set also set the message rate. Raw sensor entities publish at their
update interval whether anything happened or not, which is a telemetry stream
the controller has to filter to stay responsive.

## Decision

Three rules govern what the device exports and what it decides.

**One owner per actuator.** While a controller is connected, only the controller
commands the motor and the light. The firmware holds no competing desired state,
so there is nothing to converge against. It acts on the motor in exactly two
cases: it lost the controller, so nobody is left to decide; and a run request
went unanswered past a timeout, so the controller looks wedged. The second case
is staged, and the first stage is not an override: the device withdraws the run
request, which is a fact, and the controller stops the belt under its own
policy. Only if the belt is still running after a further grace period does the
firmware touch the motor.

Losing the controller must also clear the run request, not only stop the belt. A
device that keeps reporting a brick en route will be believed by the next
controller to connect, which then starts the belt with nothing on it.

**Devices report facts, not commands or verdicts.** "A brick is on its way" is a
fact: something has to remember a brick that left the entry beam and has not
reached the exit, and that is hardware timing. "This brick is red" is not a
fact; it is a threshold, and thresholds are policy that belongs in MCL where
they can change without a reflash.

**Export only what the controller reads.** `ConveyorScenario` now advertises
exactly four entities: `Conveyor Motor`, `Status Light`, `Brick En Route`, and
`Exit Red Ratio`. Raw channels stay internal to the device.

Two consequences of those rules are load-bearing enough to record.

The red ratio is a single number that answers two questions, because a negative
value means "no trustworthy reading". That covers an empty exit, a brick lifted
mid-capture, and a capture too dark to mean anything. A real ratio is a share of
a positive total and can never be negative, so one entity carries both presence
and measurement and there is nothing to correlate.

The light declares its effects and never selects one. MGMT picks by name from
the declared set. Per-frame animation timing stays on the device, where it
belongs, and the decision stays in MCL. This is why `ConveyorScenario` now
advertises an `Effects` list, and why the acceptance test asserts both selecting
an effect and clearing it.

The thirteen-entity scenario is not deleted, because it was serving a second
purpose: the client integration test used it to exercise every generic
per-domain command path. That set is now `EntityFamilyScenario`, named for what
it actually is, with its own key block. It deliberately describes no appliance.

## Consequences

`simulator.ConveyorScenario` changes shape, and the constants
`EntrySensorKey`, `ExitSensorKey`, `RunRequestKey`, `RedSensorKey`,
`GreenSensorKey`, `BlueSensorKey`, `ClearSensorKey`, `EnableSwitchKey`,
`SpeedNumberKey`, and `ResetButtonKey` are replaced by `BrickEnRouteKey`,
`ExitRedRatioKey`, and the `Family*` keys. `ConveyorFanKey` and
`StatusLightKey` keep their values. This is a source-breaking change to the
simulator package for any external caller that referenced a removed key. The
simulator is test and demo scaffolding rather than the client API, and the
support matrix records no external consumer, so the break is accepted rather
than aliased. A caller that wants the old shape uses `EntityFamilyScenario`.

`cmd/conveyor-sim-server` gains a firmware model instead of a static device, so
the acceptance test exercises the contract rather than only the transport. The
previous scenario let the demo pass while advertising none of the entities the
MCL read: MGMT silently observed zero values and still converged. That is the
failure mode this ADR is most concerned with, and the new assertions close it.

The model keeps the firmware's real delays, so an acceptance run takes about
twenty seconds instead of three. Its timings are injectable so unit tests
exercise the same logic and the same ratios between delays in milliseconds.

The conveyor MCL and the simulated firmware are now explicitly one contract. A
change to entity names, declared effects, or the red threshold requires a
matching change to `ConveyorScenario` and to the assertions in
`tools/test-mgmt-conveyor.sh`. The MCL hash pin in that script is what forces
the pairing to be noticed.

Nothing here is hardware evidence. The red ratios the simulator publishes are
values from an earlier bench capture, not measurements taken during a run.
