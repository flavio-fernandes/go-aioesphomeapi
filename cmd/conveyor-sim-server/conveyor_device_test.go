package main

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/flavio-fernandes/go-aioesphomeapi/pb"
	"github.com/flavio-fernandes/go-aioesphomeapi/simulator"
	"google.golang.org/protobuf/proto"
)

// testTimeScale shortens every firmware delay by the same factor, so these
// tests exercise the real logic and the real ratios between the delays in
// milliseconds instead of seconds.
const testTimeScale = 100

type conveyorRecorder struct {
	mu     sync.Mutex
	events []string
}

func (r *conveyorRecorder) PushState(state proto.Message) error {
	switch value := state.(type) {
	case *pb.BinarySensorStateResponse:
		r.append(fmt.Sprintf("en_route=%t", value.State))
	case *pb.SensorStateResponse:
		r.append(fmt.Sprintf("ratio=%.1f", value.State))
	case *pb.FanStateResponse:
		r.append(fmt.Sprintf("motor=%t", value.State))
	default:
		r.append(fmt.Sprintf("state=%T", state))
	}
	return nil
}

func (r *conveyorRecorder) PushLog(entry *pb.SubscribeLogsResponse) error {
	r.append("log=" + strings.TrimSuffix(string(entry.Message), "\n"))
	return nil
}

func (r *conveyorRecorder) append(event string) {
	r.mu.Lock()
	r.events = append(r.events, event)
	r.mu.Unlock()
}

func (r *conveyorRecorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.events...)
}

// waitForCount waits until at least count events have been recorded and returns
// them, so a test can react to the device the way a controller would.
func (r *conveyorRecorder) waitForCount(t *testing.T, count int) []string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		got := r.snapshot()
		if len(got) >= count {
			return got
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %d events, got %q", count, got)
		}
		time.Sleep(time.Millisecond)
	}
}

// wantExactly waits for the expected events in order and fails on any extra.
func (r *conveyorRecorder) wantExactly(t *testing.T, expected []string) {
	t.Helper()
	got := r.waitForCount(t, len(expected))
	for index, want := range expected {
		if got[index] != want {
			t.Fatalf("event[%d] = %q, want %q (all: %q)", index, got[index], want, got)
		}
	}
	// Let a wrongly scheduled extra event show up rather than racing past it.
	time.Sleep(50 * time.Millisecond)
	if got := r.snapshot(); len(got) > len(expected) {
		t.Fatalf("unexpected extra events: %q", got[len(expected):])
	}
}

func newTestFirmware(device devicePusher, cycles int, travel, dwell time.Duration) *conveyorFirmware {
	firmware := newConveyorFirmware(device, cycles, travel, dwell)
	firmware.timings = firmwareTimings().divided(testTimeScale)
	return firmware
}

// TestConveyorFirmwareRunsOneBrick covers the whole demo story for a single
// brick: the device asks for a run, the controller owns the motor, the brick
// arrives and is measured once, and lifting it off returns the device to having
// no reading. The order of these events is what the MCL depends on.
func TestConveyorFirmwareRunsOneBrick(t *testing.T) {
	recorder := &conveyorRecorder{}
	firmware := newTestFirmware(recorder, 1, 20*time.Millisecond, 10*time.Millisecond)
	commands := make(chan proto.Message)
	done := make(chan struct{})
	go func() {
		firmware.run(commands)
		close(done)
	}()

	// Boot values, then a brick settles on the entry and the device asks for a
	// run. Nothing has touched the motor.
	recorder.waitForCount(t, 4)

	// Play the controller: a brick is on its way, so start the belt.
	commands <- &pb.FanCommandRequest{Key: simulator.ConveyorFanKey, State: true}

	// The brick arrives, so the device withdraws the request. A controller
	// stops the belt at that point; the device must not need it to.
	recorder.waitForCount(t, 5)
	commands <- &pb.FanCommandRequest{Key: simulator.ConveyorFanKey, State: false}

	recorder.wantExactly(t, []string{
		"en_route=false",
		"ratio=-1.0",
		"log=operator placed a red brick on the entry",
		"en_route=true",
		"en_route=false",
		"log=Exit red ratio 48.0% from 5 samples",
		"ratio=48.0",
		"log=operator lifted the red brick off the exit",
		"ratio=-1.0",
		"log=bench sequence finished after 1 brick",
	})

	close(commands)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("run did not stop after the command channel closed")
	}
}

// TestConveyorFirmwareRotatesBrickColours checks that a demo run exercises both
// classifications. The acceptance test asserts a red brick and a non-red one,
// so the rotation order is part of that contract.
func TestConveyorFirmwareRotatesBrickColours(t *testing.T) {
	recorder := &conveyorRecorder{}
	firmware := newTestFirmware(recorder, 2, 20*time.Millisecond, 10*time.Millisecond)
	commands := make(chan proto.Message)
	done := make(chan struct{})
	go func() {
		firmware.run(commands)
		close(done)
	}()

	// Follow the run request for as long as the bench keeps producing bricks.
	go func() {
		seen := 0
		for {
			select {
			case <-done:
				return
			default:
			}
			events := recorder.snapshot()
			for ; seen < len(events); seen++ {
				var state bool
				switch events[seen] {
				case "en_route=true":
					state = true
				case "en_route=false":
					state = false
				default:
					continue
				}
				select {
				case commands <- &pb.FanCommandRequest{Key: simulator.ConveyorFanKey, State: state}:
				case <-done:
					return
				}
			}
			time.Sleep(time.Millisecond)
		}
	}()

	deadline := time.Now().Add(5 * time.Second)
	for {
		events := recorder.snapshot()
		if containsAll(events, "log=Exit red ratio 48.0% from 5 samples",
			"log=Exit red ratio 29.0% from 5 samples",
			"log=bench sequence finished after 2 bricks") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("both brick colours were not measured: %q", events)
		}
		time.Sleep(time.Millisecond)
	}

	// The device must end with no reading, so an idle controller shows nothing.
	events := recorder.snapshot()
	var lastRatio string
	for _, event := range events {
		if strings.HasPrefix(event, "ratio=") {
			lastRatio = event
		}
	}
	if lastRatio != "ratio=-1.0" {
		t.Fatalf("device did not end without a reading: %q (all: %q)", lastRatio, events)
	}

	close(commands)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("run did not stop after the command channel closed")
	}
}

// TestConveyorFirmwareJamWithdrawsThenBacksStop covers the two-stage jam
// timeout: the device first withdraws the run request, which is the normal path
// because the controller then stops the belt on its own policy, and only
// overrides the motor when the controller does not react at all.
func TestConveyorFirmwareJamWithdrawsThenBacksStop(t *testing.T) {
	recorder := &conveyorRecorder{}
	// A travel time far beyond the jam timeout means the brick never arrives.
	firmware := newTestFirmware(recorder, 1, 10*time.Second, 10*time.Millisecond)
	commands := make(chan proto.Message)
	done := make(chan struct{})
	go func() {
		firmware.run(commands)
		close(done)
	}()

	recorder.waitForCount(t, 4)
	// Start the belt and then stop reacting, the way a wedged controller would.
	commands <- &pb.FanCommandRequest{Key: simulator.ConveyorFanKey, State: true}

	recorder.wantExactly(t, []string{
		"en_route=false",
		"ratio=-1.0",
		"log=operator placed a red brick on the entry",
		"en_route=true",
		"en_route=false",
		"log=No brick reached the exit in 15s; withdrawing the run request",
		"log=Belt stopped by the firmware backstop: mgmt did not react to the withdrawn run request",
		"motor=false",
	})

	close(commands)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("run did not stop after the command channel closed")
	}
}

func containsAll(events []string, wanted ...string) bool {
	for _, want := range wanted {
		found := false
		for _, event := range events {
			if event == want {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
