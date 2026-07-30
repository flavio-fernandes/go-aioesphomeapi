package main

import (
	"fmt"
	"time"

	"github.com/flavio-fernandes/go-aioesphomeapi/pb"
	"github.com/flavio-fernandes/go-aioesphomeapi/simulator"
	"google.golang.org/protobuf/proto"
)

// Timings copied from the firmware YAML documented alongside MGMT's
// examples/lang/esphome-conveyor.mcl. They are the reason the demo looks the
// way it does, so they are named rather than inlined.
const (
	// entryDebounce is the delayed_on filter on the entry sensor. A brick has
	// to settle before it counts as a brick.
	entryDebounce = 400 * time.Millisecond
	// exitReleaseDebounce is the delayed_off filter on the exit sensor. A hand
	// lifting a brick drags the reading through the threshold more than once,
	// and each bounce would republish.
	exitReleaseDebounce = 300 * time.Millisecond
	// captureSettle plus captureSamples at captureInterval is the color
	// capture: let the belt coast and the sensor finish one integration
	// period, then average the samples.
	captureSettle   = 500 * time.Millisecond
	captureSamples  = 5
	captureInterval = 250 * time.Millisecond
	// exitHoldTime is how long a measured brick is left undisturbed before a
	// new run may start.
	exitHoldTime = 3 * time.Second
	// jamWithdraw is when a run that never arrived gives up, and jamBackstop
	// is the extra grace before the firmware overrides an unresponsive
	// controller and stops the motor itself.
	jamWithdraw = 15 * time.Second
	jamBackstop = 5 * time.Second
	// entryClearTime is how long a brick stays in the entry beam once the belt
	// starts carrying it away. This is bench geometry, not firmware.
	entryClearTime = 250 * time.Millisecond
	// betweenBricks is how long the simulated operator takes to reach for the
	// next brick. Bench pacing, not firmware.
	betweenBricks = time.Second
	// minUsableSamples matches the firmware's floor for trusting a capture.
	minUsableSamples = 3
)

// conveyorTimings collects every delay the model uses, so a test can run the
// same logic in milliseconds while the demo keeps the real firmware's pacing.
// Only scale these together: the interesting behavior is in their ratios, such
// as a capture finishing before the exit hold expires.
type conveyorTimings struct {
	entryDebounce       time.Duration
	exitReleaseDebounce time.Duration
	captureSettle       time.Duration
	captureInterval     time.Duration
	exitHold            time.Duration
	jamWithdraw         time.Duration
	jamBackstop         time.Duration
	entryClear          time.Duration
	betweenBricks       time.Duration
}

// firmwareTimings returns the delays the real firmware uses.
func firmwareTimings() conveyorTimings {
	return conveyorTimings{
		entryDebounce:       entryDebounce,
		exitReleaseDebounce: exitReleaseDebounce,
		captureSettle:       captureSettle,
		captureInterval:     captureInterval,
		exitHold:            exitHoldTime,
		jamWithdraw:         jamWithdraw,
		jamBackstop:         jamBackstop,
		entryClear:          entryClearTime,
		betweenBricks:       betweenBricks,
	}
}

// divided returns the same timings shortened by a whole factor.
func (t conveyorTimings) divided(factor time.Duration) conveyorTimings {
	return conveyorTimings{
		entryDebounce:       t.entryDebounce / factor,
		exitReleaseDebounce: t.exitReleaseDebounce / factor,
		captureSettle:       t.captureSettle / factor,
		captureInterval:     t.captureInterval / factor,
		exitHold:            t.exitHold / factor,
		jamWithdraw:         t.jamWithdraw / factor,
		jamBackstop:         t.jamBackstop / factor,
		entryClear:          t.entryClear / factor,
		betweenBricks:       t.betweenBricks / factor,
	}
}

// brick is one object the operator puts on the belt. The red ratio is what the
// exit sensor would measure for it, taken from the bench captures recorded in
// the header of esphome-conveyor.mcl.
type brick struct {
	name     string
	redRatio float32
}

// brickRotation alternates a red brick with non-red ones so a demo run
// exercises both classifications, and therefore both light outcomes.
var brickRotation = []brick{
	{name: "red", redRatio: 48.0},
	{name: "blue", redRatio: 29.0},
	{name: "yellow", redRatio: 35.0},
	{name: "green", redRatio: 26.0},
}

// script names one of the firmware's restartable scripts. Every timer belongs
// to exactly one, so restarting a script invalidates the timers already
// scheduled for it, the way ESPHome's `mode: restart` does.
type script int

const (
	scriptRequestRun script = iota
	scriptTravel
	scriptCapture
	scriptExitHold
	scriptJam
	scriptBench
	numScripts
)

// tick is a timer firing for one step of one script generation. Stale
// generations are dropped, which is what makes a restart a restart.
type tick struct {
	script script
	gen    int
	step   int
}

// devicePusher is the device surface the conveyor firmware drives.
type devicePusher interface {
	PushState(proto.Message) error
	PushLog(*pb.SubscribeLogsResponse) error
}

// conveyorFirmware reproduces the on-device automations of the conveyor
// firmware: the entry sensor asks for a run, the exit sensor reports arrival
// and measures the brick, and a jam timeout withdraws a run that never
// finished. It also plays the operator, because a belt with nobody putting
// bricks on it has nothing to demonstrate.
//
// It never steers the motor while a controller is connected. MGMT decides when
// the belt runs; this only reports what the sensors would see. The single
// exception is the jam backstop, exactly as on the real device.
type conveyorFirmware struct {
	device devicePusher
	ticks  chan tick
	gen    [numScripts]int

	// The physical world.
	entryOccupied bool
	exitOccupied  bool
	motorOn       bool
	onBelt        brick
	atExit        brick

	// Published entity values. Republishing an unchanged value is avoided the
	// way a template sensor with no lambda avoids one, so the message count
	// per brick stays at a handful.
	enRoute      bool
	enRouteKnown bool
	redRatio     float32
	ratioKnown   bool

	// Color capture accumulator.
	captureSampled int

	// The bench sequence.
	cycles    int
	completed int
	travel    time.Duration
	dwell     time.Duration
	timings   conveyorTimings
	done      chan struct{}
}

func newConveyorFirmware(device devicePusher, cycles int, travel, dwell time.Duration) *conveyorFirmware {
	return &conveyorFirmware{
		device:   device,
		ticks:    make(chan tick, 32),
		redRatio: simulator.ExitRedRatioNoReading,
		cycles:   cycles,
		travel:   travel,
		dwell:    dwell,
		timings:  firmwareTimings(),
		done:     make(chan struct{}),
	}
}

// after schedules one step of the current generation of a script.
func (f *conveyorFirmware) after(s script, step int, delay time.Duration) {
	gen := f.gen[s]
	time.AfterFunc(delay, func() {
		select {
		case f.ticks <- tick{script: s, gen: gen, step: step}:
		case <-f.done:
		}
	})
}

// restart bumps a script's generation, so every timer already scheduled for it
// is ignored, and then schedules its first step.
func (f *conveyorFirmware) restart(s script, step int, delay time.Duration) {
	f.gen[s]++
	f.after(s, step, delay)
}

// stop bumps a script's generation without scheduling anything.
func (f *conveyorFirmware) stop(s script) { f.gen[s]++ }

// run owns every field above. No other goroutine may touch them.
func (f *conveyorFirmware) run(commands <-chan proto.Message) {
	defer close(f.done)

	// Publish the boot values the firmware publishes, so MGMT starts from
	// known state rather than from unpublished state.
	f.publishEnRoute(false)
	f.publishRatio(simulator.ExitRedRatioNoReading)
	f.restart(scriptBench, benchPlace, f.timings.betweenBricks)

	for {
		select {
		case command, ok := <-commands:
			if !ok {
				return
			}
			printCommand(command)
			if request, isFan := command.(*pb.FanCommandRequest); isFan && request.Key == simulator.ConveyorFanKey {
				f.onFanCommand(request.State)
			}
		case t := <-f.ticks:
			if t.gen != f.gen[t.script] {
				continue // a restarted or stopped script
			}
			f.onTick(t)
		}
	}
}

// Steps of the bench sequence and the travel model.
const (
	benchPlace = iota
	benchLift
	benchReleased
)

const (
	travelClearEntry = iota
	travelArrive
)

func (f *conveyorFirmware) onTick(t tick) {
	switch t.script {
	case scriptBench:
		f.onBench(t.step)
	case scriptRequestRun:
		// The brick may have been picked back up while the exit hold ran, so
		// re-check rather than trusting the reason we were started.
		if f.entryOccupied {
			f.publishEnRoute(true)
		}
	case scriptTravel:
		f.onTravel(t.step)
	case scriptCapture:
		f.onCapture(t.step)
	case scriptExitHold:
		// Expiring is the whole point: a run request that was held back
		// because a measured brick was still sitting at the exit may proceed.
		if f.entryOccupied && !f.enRoute {
			f.restart(scriptRequestRun, 0, 0)
		}
	case scriptJam:
		f.onJam(t.step)
	}
}

// onBench plays the operator: place a brick, let the light show what it made of
// it, lift it off, repeat.
func (f *conveyorFirmware) onBench(step int) {
	switch step {
	case benchPlace:
		f.onBelt = brickRotation[f.completed%len(brickRotation)]
		f.entryOccupied = true
		f.log(pb.LogLevel_LOG_LEVEL_INFO,
			fmt.Sprintf("operator placed a %s brick on the entry", f.onBelt.name))
		// The entry sensor's on_press, after its delayed_on filter.
		f.restart(scriptRequestRun, 0, f.timings.entryDebounce)
	case benchLift:
		f.log(pb.LogLevel_LOG_LEVEL_INFO,
			fmt.Sprintf("operator lifted the %s brick off the exit", f.atExit.name))
		f.exitOccupied = false
		f.stop(scriptCapture)
		f.restart(scriptBench, benchReleased, f.timings.exitReleaseDebounce)
	case benchReleased:
		// The exit sensor's on_release, after its delayed_off filter.
		f.publishRatio(simulator.ExitRedRatioNoReading)
		f.completed++
		if f.cycles <= 0 || f.completed < f.cycles {
			f.after(scriptBench, benchPlace, f.timings.betweenBricks)
			return
		}
		// Stop putting bricks on the belt, but keep being a device. The
		// controller still has an idle light to settle, and a real device
		// would still be listening.
		noun := "bricks"
		if f.completed == 1 {
			noun = "brick"
		}
		f.log(pb.LogLevel_LOG_LEVEL_INFO,
			fmt.Sprintf("bench sequence finished after %d %s", f.completed, noun))
	}
}

// onTravel carries the brick from the entry beam to the exit sensor.
func (f *conveyorFirmware) onTravel(step int) {
	switch step {
	case travelClearEntry:
		f.entryOccupied = false
		f.after(scriptTravel, travelArrive, f.travel)
	case travelArrive:
		f.onArrival()
	}
}

// onArrival is the exit sensor's on_press: the brick is no longer on its way,
// so stop the jam clock, report that fact, and start measuring. The firmware
// deliberately does not touch the motor here. That is MGMT's call, and the
// capture tolerates the belt coasting for a moment.
func (f *conveyorFirmware) onArrival() {
	f.exitOccupied = true
	f.atExit = f.onBelt
	f.stop(scriptJam)
	f.publishEnRoute(false)
	f.captureSampled = 0
	f.restart(scriptCapture, 0, f.timings.captureSettle)
	f.restart(scriptExitHold, 0, f.timings.exitHold)
}

// onCapture averages stationary samples and publishes one reading, or the "no
// trustworthy reading" value when too few samples were usable.
func (f *conveyorFirmware) onCapture(step int) {
	if step < captureSamples {
		if f.exitOccupied {
			f.captureSampled++
		}
		f.after(scriptCapture, step+1, f.timings.captureInterval)
		return
	}
	if f.captureSampled < minUsableSamples {
		f.log(pb.LogLevel_LOG_LEVEL_WARN,
			fmt.Sprintf("Color capture rejected: %d usable samples", f.captureSampled))
		f.publishRatio(simulator.ExitRedRatioNoReading)
		return
	}
	f.log(pb.LogLevel_LOG_LEVEL_INFO,
		fmt.Sprintf("Exit red ratio %.1f%% from %d samples", f.atExit.redRatio, f.captureSampled))
	f.publishRatio(f.atExit.redRatio)
	// Give the light its moment before the operator reaches in.
	f.after(scriptBench, benchLift, f.dwell)
}

// onJam withdraws a run that never arrived, then backstops a controller that
// did not react to the withdrawal.
func (f *conveyorFirmware) onJam(step int) {
	switch step {
	case 0:
		f.publishEnRoute(false)
		f.log(pb.LogLevel_LOG_LEVEL_INFO,
			"No brick reached the exit in 15s; withdrawing the run request")
		f.after(scriptJam, 1, f.timings.jamBackstop)
	case 1:
		if !f.motorOn {
			return
		}
		f.motorOn = false
		f.log(pb.LogLevel_LOG_LEVEL_WARN,
			"Belt stopped by the firmware backstop: mgmt did not react to the withdrawn run request")
		_ = f.device.PushState(&pb.FanStateResponse{Key: simulator.ConveyorFanKey, State: false,
			Direction: pb.FanDirection_FAN_DIRECTION_FORWARD})
	}
}

// onFanCommand reacts to MGMT owning the motor. The belt carries whatever is on
// it; the firmware's only motor automation is the jam clock.
func (f *conveyorFirmware) onFanCommand(state bool) {
	if state == f.motorOn {
		return
	}
	f.motorOn = state
	if !state {
		// on_turn_off stops the jam clock. A brick still between the sensors
		// stops with the belt; if MGMT still wants a run it will start it
		// again, and travel restarts from the entry.
		f.stop(scriptJam)
		f.stop(scriptTravel)
		return
	}
	f.restart(scriptJam, 0, f.timings.jamWithdraw)
	if f.entryOccupied {
		f.restart(scriptTravel, travelClearEntry, f.timings.entryClear)
	}
}

func (f *conveyorFirmware) publishEnRoute(state bool) {
	if f.enRouteKnown && state == f.enRoute {
		return
	}
	f.enRoute = state
	f.enRouteKnown = true
	_ = f.device.PushState(&pb.BinarySensorStateResponse{Key: simulator.BrickEnRouteKey, State: state})
}

func (f *conveyorFirmware) publishRatio(ratio float32) {
	if f.ratioKnown && ratio == f.redRatio {
		return
	}
	f.redRatio = ratio
	f.ratioKnown = true
	_ = f.device.PushState(&pb.SensorStateResponse{Key: simulator.ExitRedRatioKey, State: ratio})
}

func (f *conveyorFirmware) log(level pb.LogLevel, message string) {
	_ = f.device.PushLog(&pb.SubscribeLogsResponse{Level: level, Message: []byte(message + "\n")})
}
