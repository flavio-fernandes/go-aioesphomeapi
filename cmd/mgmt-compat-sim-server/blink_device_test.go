package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	api "github.com/flavio-fernandes/go-aioesphomeapi"
	"github.com/flavio-fernandes/go-aioesphomeapi/pb"
	"github.com/flavio-fernandes/go-aioesphomeapi/simulator"
	"google.golang.org/protobuf/proto"
)

type recordingPusher struct {
	mu     sync.Mutex
	events []string
}

func (r *recordingPusher) PushState(state proto.Message) error {
	switch value := state.(type) {
	case *pb.SwitchStateResponse:
		r.append(fmt.Sprintf("switch=%t", value.State))
	case *pb.BinarySensorStateResponse:
		r.append(fmt.Sprintf("sensor=%t", value.State))
	default:
		r.append(fmt.Sprintf("state=%T", state))
	}
	return nil
}

func (r *recordingPusher) PushLog(entry *pb.SubscribeLogsResponse) error {
	r.append("log=" + strings.TrimSuffix(string(entry.Message), "\n"))
	return nil
}

func (r *recordingPusher) append(event string) {
	r.mu.Lock()
	r.events = append(r.events, event)
	r.mu.Unlock()
}

func (r *recordingPusher) waitForEvents(t *testing.T, expected []string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for {
		r.mu.Lock()
		got := append([]string(nil), r.events...)
		r.mu.Unlock()
		if len(got) >= len(expected) {
			for index, want := range expected {
				if got[index] != want {
					t.Fatalf("event[%d] = %q, want %q (all: %q)", index, got[index], want, got)
				}
			}
			if len(got) > len(expected) {
				t.Fatalf("unexpected extra events: %q", got[len(expected):])
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %q, got %q", expected, got)
		}
		time.Sleep(2 * time.Millisecond)
	}
}

func TestBlinkFirmwareRelightsAfterDelay(t *testing.T) {
	pusher := &recordingPusher{}
	firmware := &blinkFirmware{device: pusher, relight: 20 * time.Millisecond}
	commands := make(chan proto.Message)
	done := make(chan struct{})
	go func() {
		firmware.run(commands)
		close(done)
	}()

	commands <- &pb.ButtonCommandRequest{Key: 9}
	commands <- &pb.SwitchCommandRequest{Key: simulator.BlinkSwitchKey, State: false}
	pusher.waitForEvents(t, []string{
		"switch=false",
		"sensor=false",
		"log=LED turned off; turning it back on in three seconds",
		"switch=true",
		"sensor=true",
		"log=LED is still off; turning it on",
	})

	close(commands)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("run did not stop after the command channel closed")
	}
}

func TestBlinkFirmwareOnCommandCancelsPendingRelight(t *testing.T) {
	pusher := &recordingPusher{}
	firmware := &blinkFirmware{device: pusher, relight: 50 * time.Millisecond}
	commands := make(chan proto.Message)
	done := make(chan struct{})
	go func() {
		firmware.run(commands)
		close(done)
	}()

	commands <- &pb.SwitchCommandRequest{Key: simulator.BlinkSwitchKey, State: false}
	commands <- &pb.SwitchCommandRequest{Key: simulator.BlinkSwitchKey, State: true}
	time.Sleep(150 * time.Millisecond)
	pusher.waitForEvents(t, []string{
		"switch=false",
		"sensor=false",
		"log=LED turned off; turning it back on in three seconds",
		"switch=true",
		"sensor=true",
		"log=LED turned on; waiting for mgmt to turn it off",
	})

	close(commands)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("run did not stop after the command channel closed")
	}
}

// TestBlinkDeviceLoopAgainstRealClient drives the complete cooperative cycle a
// real MGMT process performs: observe the LED on, command it off, watch the
// mirrored sensor follow, and watch the device relight the LED by itself.
func TestBlinkDeviceLoopAgainstRealClient(t *testing.T) {
	device := simulator.New(simulator.BlinkScenario())
	t.Cleanup(func() { _ = device.Close() })
	firmware := &blinkFirmware{device: device, relight: 50 * time.Millisecond}
	go firmware.run(device.Commands())

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := api.DialWithContext(ctx, "esphome-blink:6053", time.Second, device.ClientOptions()...)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	type observation struct {
		kind  string
		value bool
	}
	states := make(chan observation, 16)
	unsubscribe, err := client.SubscribeStates(func(message proto.Message) {
		switch value := message.(type) {
		case *pb.BinarySensorStateResponse:
			states <- observation{kind: "sensor", value: value.State}
		case *pb.SwitchStateResponse:
			states <- observation{kind: "switch", value: value.State}
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(unsubscribe)

	logs := make(chan string, 16)
	unsubscribeLogs, err := client.SubscribeLogs(pb.LogLevel_LOG_LEVEL_DEBUG, func(entry *pb.SubscribeLogsResponse) {
		logs <- strings.TrimSuffix(string(entry.Message), "\n")
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(unsubscribeLogs)

	expectState := func(want observation) {
		t.Helper()
		for {
			select {
			case got := <-states:
				if got == want {
					return
				}
			case <-time.After(2 * time.Second):
				t.Fatalf("timed out waiting for %+v", want)
			}
		}
	}
	expectLog := func(want string) {
		t.Helper()
		for {
			select {
			case got := <-logs:
				if got == want {
					return
				}
			case <-time.After(2 * time.Second):
				t.Fatalf("timed out waiting for log %q", want)
			}
		}
	}

	expectState(observation{kind: "sensor", value: true})
	expectState(observation{kind: "switch", value: true})
	expectLog("blink simulator ready")

	if err := client.SendCommand(&pb.SwitchCommandRequest{Key: simulator.BlinkSwitchKey, State: false}); err != nil {
		t.Fatal(err)
	}
	expectState(observation{kind: "switch", value: false})
	expectState(observation{kind: "sensor", value: false})
	expectLog("LED turned off; turning it back on in three seconds")

	expectState(observation{kind: "switch", value: true})
	expectState(observation{kind: "sensor", value: true})
	expectLog("LED is still off; turning it on")
}
