package main

import (
	"fmt"
	"time"

	"github.com/flavio-fernandes/go-aioesphomeapi/pb"
	"github.com/flavio-fernandes/go-aioesphomeapi/simulator"
	"google.golang.org/protobuf/proto"
)

// blinkRelightDelay matches the three-second delay documented in the firmware
// YAML embedded in MGMT's examples/lang/esphome-blink.mcl.
const blinkRelightDelay = 3 * time.Second

// statePusher is the device surface the blink firmware drives.
type statePusher interface {
	PushState(proto.Message) error
	PushLog(*pb.SubscribeLogsResponse) error
}

// blinkFirmware reproduces the on-device automations from the firmware YAML
// documented in esphome-blink.mcl: the writable switch is mirrored into a
// read-only binary sensor, and whenever the LED is turned off the device turns
// it back on after a fixed delay. Together with MGMT turning the LED off
// whenever it sees it on, this produces the endless cooperative blink loop.
type blinkFirmware struct {
	device  statePusher
	relight time.Duration
}

// run consumes received commands until the channel closes. It owns all blink
// state, so no other goroutine may touch the timer or the LED belief.
func (f *blinkFirmware) run(commands <-chan proto.Message) {
	ledOn := true // BlinkScenario starts with the LED on.
	var timer *time.Timer
	var relight <-chan time.Time
	stopTimer := func() {
		if timer != nil && !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		relight = nil
	}
	defer stopTimer()

	for {
		select {
		case command, ok := <-commands:
			if !ok {
				return
			}
			printCommand(command)
			request, isSwitch := command.(*pb.SwitchCommandRequest)
			if !isSwitch || request.Key != simulator.BlinkSwitchKey {
				continue
			}
			ledOn = request.State
			if request.State {
				stopTimer()
				f.mirror(true)
				f.log("LED turned on; waiting for mgmt to turn it off")
				continue
			}
			f.mirror(false)
			f.log("LED turned off; turning it back on in three seconds")
			stopTimer()
			if timer == nil {
				timer = time.NewTimer(f.relight)
			} else {
				timer.Reset(f.relight)
			}
			relight = timer.C
		case <-relight:
			relight = nil
			if ledOn {
				continue
			}
			ledOn = true
			f.push(&pb.SwitchStateResponse{Key: simulator.BlinkSwitchKey, State: true})
			f.mirror(true)
			f.log("LED is still off; turning it on")
			fmt.Println("simulated firmware relit the LED")
		}
	}
}

// mirror copies the switch state into the template binary sensor, like the
// firmware's onboard_led_state lambda.
func (f *blinkFirmware) mirror(state bool) {
	f.push(&pb.BinarySensorStateResponse{Key: simulator.BlinkSensorKey, State: state})
}

func (f *blinkFirmware) push(state proto.Message) {
	// A push can only fail on device shutdown, which ends run via the closed
	// commands channel.
	_ = f.device.PushState(state)
}

func (f *blinkFirmware) log(message string) {
	_ = f.device.PushLog(&pb.SubscribeLogsResponse{
		Level:   pb.LogLevel_LOG_LEVEL_INFO,
		Message: []byte(message + "\n"),
	})
}

func printCommand(command proto.Message) {
	switch value := command.(type) {
	case *pb.SwitchCommandRequest:
		fmt.Printf("received switch command: key=%d state=%t\n", value.Key, value.State)
	case *pb.NumberCommandRequest:
		fmt.Printf("received number command: key=%d value=%g\n", value.Key, value.State)
	case *pb.ButtonCommandRequest:
		fmt.Printf("received button command: key=%d\n", value.Key)
	}
}
