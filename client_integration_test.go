package aioesphomeapi_test

import (
	"context"
	"errors"
	"testing"
	"time"

	api "github.com/flavio-fernandes/go-aioesphomeapi"
	"github.com/flavio-fernandes/go-aioesphomeapi/pb"
	"github.com/flavio-fernandes/go-aioesphomeapi/simulator"
	"google.golang.org/protobuf/proto"
)

func TestSecureConveyorRoundTrip(t *testing.T) {
	device := simulator.New(simulator.ConveyorScenario())
	t.Cleanup(func() { _ = device.Close() })
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := api.DialWithContext(ctx, "simulator:6053", time.Second, device.ClientOptions()...)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	if client.Name() != "conveyor-simulator" {
		t.Fatalf("unexpected name %q", client.Name())
	}

	descriptors, err := client.ListEntities()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(descriptors) != 13 {
		t.Fatalf("got %d descriptors, want 13", len(descriptors))
	}
	if got := client.Entities().Fans(); len(got) != 1 || got[0].ObjectID != "conveyor_motor" {
		t.Fatalf("unexpected fans: %#v", got)
	}
	if got := client.Entities().Lights(); len(got) != 1 || got[0].ObjectID != "status_light" {
		t.Fatalf("unexpected lights: %#v", got)
	}

	states := make(chan proto.Message, 32)
	unsubscribe, err := client.SubscribeStates(func(message proto.Message) { states <- message })
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	t.Cleanup(unsubscribe)
	waitForType[*pb.FanStateResponse](t, states)
	waitForType[*pb.BinarySensorStateResponse](t, states)
	waitForType[*pb.SensorStateResponse](t, states)
	waitForType[*pb.SwitchStateResponse](t, states)
	waitForType[*pb.NumberStateResponse](t, states)
	waitForType[*pb.TextSensorStateResponse](t, states)

	if err := client.SetFan(simulator.ConveyorFanKey, api.FanCommandOpts{HasState: true, State: true, HasSpeedLevel: true, SpeedLevel: 42, HasDirection: true, Direction: pb.FanDirection_FAN_DIRECTION_REVERSE}); err != nil {
		t.Fatalf("fan command: %v", err)
	}
	fanState := waitForType[*pb.FanStateResponse](t, states)
	if !fanState.State || fanState.SpeedLevel != 42 || fanState.Direction != pb.FanDirection_FAN_DIRECTION_REVERSE {
		t.Fatalf("unexpected fan state: %#v", fanState)
	}

	if err := client.SetLight(simulator.StatusLightKey, api.LightCommandOpts{HasState: true, State: true, HasColorMode: true, ColorMode: pb.ColorMode_COLOR_MODE_RGB, HasRGB: true, Red: 1, Green: 0.25, Blue: 0}); err != nil {
		t.Fatalf("light command: %v", err)
	}
	lightState := waitForType[*pb.LightStateResponse](t, states)
	if !lightState.State || lightState.Red != 1 || lightState.Green != 0.25 || lightState.Blue != 0 {
		t.Fatalf("unexpected light state: %#v", lightState)
	}
	if err := client.SetSwitch(simulator.EnableSwitchKey, true); err != nil {
		t.Fatalf("switch command: %v", err)
	}
	if state := waitForType[*pb.SwitchStateResponse](t, states); !state.State {
		t.Fatalf("switch did not turn on: %#v", state)
	}
	if err := client.SetNumber(simulator.SpeedNumberKey, 35); err != nil {
		t.Fatalf("number command: %v", err)
	}
	if state := waitForType[*pb.NumberStateResponse](t, states); state.State != 35 {
		t.Fatalf("number state is %v", state.State)
	}
	if err := client.PressButton(simulator.ResetButtonKey); err != nil {
		t.Fatalf("button command: %v", err)
	}

	logs := make(chan *pb.SubscribeLogsResponse, 1)
	stopLogs, err := client.SubscribeLogs(pb.LogLevel_LOG_LEVEL_INFO, func(message *pb.SubscribeLogsResponse) { logs <- message })
	if err != nil {
		t.Fatalf("logs: %v", err)
	}
	t.Cleanup(stopLogs)
	select {
	case entry := <-logs:
		if string(entry.Message) != "conveyor simulator ready\n" {
			t.Fatalf("unexpected log %q", entry.Message)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for log")
	}
}

func TestPlaintextRequiresExplicitOptIn(t *testing.T) {
	_, err := api.DialWithContext(context.Background(), "unused", time.Millisecond)
	if !errors.Is(err, api.ErrTransportPolicy) {
		t.Fatalf("got %v, want transport policy error", err)
	}
	device := simulator.New(simulator.ConveyorScenario(), simulator.WithPlaintext())
	t.Cleanup(func() { _ = device.Close() })
	client, err := api.DialWithContext(context.Background(), "simulator:6053", time.Second, device.ClientOptions()...)
	if err != nil {
		t.Fatalf("explicit plaintext dial: %v", err)
	}
	_ = client.Close()
}

func TestWrongNoiseKeyIsRedacted(t *testing.T) {
	device := simulator.New(simulator.ConveyorScenario())
	t.Cleanup(func() { _ = device.Close() })
	options := []api.Option{api.WithDialContext(device.DialContext), api.WithEncryptionKey("d3Jvbmctc2ltdWxhdG9yLXRlc3Qta2V5LTAwMDAwMDE=")}
	_, err := api.DialWithContext(context.Background(), "private-device-name.local:6053", 100*time.Millisecond, options...)
	if err == nil {
		t.Fatal("wrong key unexpectedly succeeded")
	}
	if got := err.Error(); got == "" || contains(got, "private-device-name") || contains(got, "d3Jvbm") {
		t.Fatalf("error leaks connection material: %q", got)
	}
}

func waitForType[T proto.Message](t *testing.T, messages <-chan proto.Message) T {
	t.Helper()
	var zero T
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	for {
		select {
		case message := <-messages:
			if typed, ok := message.(T); ok {
				return typed
			}
		case <-timer.C:
			t.Fatalf("timed out waiting for %T", zero)
			return zero
		}
	}
}

func contains(value, fragment string) bool {
	if len(fragment) == 0 {
		return true
	}
	for i := 0; i+len(fragment) <= len(value); i++ {
		if value[i:i+len(fragment)] == fragment {
			return true
		}
	}
	return false
}
