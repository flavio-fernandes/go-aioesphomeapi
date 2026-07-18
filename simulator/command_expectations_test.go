package simulator_test

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/flavio-fernandes/go-aioesphomeapi/pb"
	"github.com/flavio-fernandes/go-aioesphomeapi/simulator"
)

func TestCommandExpectationsMatchExactOrderAndCount(t *testing.T) {
	device := simulator.New(simulator.Scenario{
		Name: "ordered-command-simulator",
		Commands: []simulator.CommandExpectation{
			{Command: &pb.SwitchCommandRequest{Key: 7, State: true}, Count: 2},
			{Command: &pb.ButtonCommandRequest{Key: 8}, Count: 1},
		},
	})
	t.Cleanup(func() { _ = device.Close() })
	client := dialSimulator(t, device)
	t.Cleanup(func() { _ = client.Close() })

	for range 2 {
		if err := client.SendCommand(&pb.SwitchCommandRequest{Key: 7, State: true}); err != nil {
			t.Fatal(err)
		}
	}
	if err := client.SendCommand(&pb.ButtonCommandRequest{Key: 8}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := device.WaitForCommandExpectations(ctx); err != nil {
		t.Fatalf("WaitForCommandExpectations: %v", err)
	}
}

func TestCommandExpectationOutcomesAreDistinct(t *testing.T) {
	tests := []struct {
		name string
		send *pb.ButtonCommandRequest
		want error
		code simulator.CommandExpectationCode
	}{
		{
			name: "out of order",
			send: &pb.ButtonCommandRequest{Key: 2},
			want: simulator.ErrCommandOutOfOrder,
			code: simulator.CommandOutOfOrder,
		},
		{
			name: "unexpected",
			send: &pb.ButtonCommandRequest{Key: 3},
			want: simulator.ErrCommandUnexpected,
			code: simulator.CommandUnexpected,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			device := simulator.New(simulator.Scenario{
				Name: "failing-command-simulator",
				Commands: []simulator.CommandExpectation{
					{Command: &pb.ButtonCommandRequest{Key: 1}, Count: 1},
					{Command: &pb.ButtonCommandRequest{Key: 2}, Count: 1},
				},
			})
			t.Cleanup(func() { _ = device.Close() })
			client := dialSimulator(t, device)
			t.Cleanup(func() { _ = client.Close() })
			if err := client.SendCommand(test.send); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			err := device.WaitForCommandExpectations(ctx)
			assertCommandExpectationError(t, err, test.want, test.code)
		})
	}
}

func TestCommandExpectationDetectsTrailingCommand(t *testing.T) {
	device := simulator.New(simulator.Scenario{
		Name: "trailing-command-simulator",
		Commands: []simulator.CommandExpectation{{
			Command: &pb.ButtonCommandRequest{Key: 1},
			Count:   1,
		}},
	})
	t.Cleanup(func() { _ = device.Close() })
	client := dialSimulator(t, device)
	t.Cleanup(func() { _ = client.Close() })
	if err := client.SendCommand(&pb.ButtonCommandRequest{Key: 1}); err != nil {
		t.Fatal(err)
	}
	if err := client.SendCommand(&pb.ButtonCommandRequest{Key: 2}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := client.Ping(ctx); err != nil {
		t.Fatalf("Ping command barrier: %v", err)
	}
	err := device.WaitForCommandExpectations(ctx)
	assertCommandExpectationError(t, err, simulator.ErrCommandUnexpected, simulator.CommandUnexpected)
}

func TestCommandExpectationDetectsUnhandledTrailingCommandType(t *testing.T) {
	device := simulator.New(simulator.Scenario{
		Name: "unhandled-trailing-command-simulator",
		Commands: []simulator.CommandExpectation{{
			Command: &pb.ButtonCommandRequest{Key: 1},
			Count:   1,
		}},
	})
	t.Cleanup(func() { _ = device.Close() })
	client := dialSimulator(t, device)
	t.Cleanup(func() { _ = client.Close() })
	if err := client.SendCommand(&pb.ButtonCommandRequest{Key: 1}); err != nil {
		t.Fatal(err)
	}
	if err := client.SendCommand(&pb.CoverCommandRequest{Key: 2}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := client.Ping(ctx); err != nil {
		t.Fatalf("Ping command barrier: %v", err)
	}
	err := device.WaitForCommandExpectations(ctx)
	assertCommandExpectationError(t, err, simulator.ErrCommandUnexpected, simulator.CommandUnexpected)
}

func TestCommandExpectationMissingPreservesContextCause(t *testing.T) {
	device := simulator.New(simulator.Scenario{
		Commands: []simulator.CommandExpectation{{
			Command: &pb.ButtonCommandRequest{Key: 1},
			Count:   1,
		}},
	})
	t.Cleanup(func() { _ = device.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := device.WaitForCommandExpectations(ctx)
	assertCommandExpectationError(t, err, simulator.ErrCommandMissing, simulator.CommandMissing)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled in chain", err)
	}
}

func TestCommandExpectationWaitUnblocksWhenDeviceCloses(t *testing.T) {
	device := simulator.New(simulator.Scenario{
		Commands: []simulator.CommandExpectation{{
			Command: &pb.ButtonCommandRequest{Key: 1},
			Count:   1,
		}},
	})
	if err := device.Close(); err != nil {
		t.Fatal(err)
	}
	err := device.WaitForCommandExpectations(context.Background())
	assertCommandExpectationError(t, err, simulator.ErrCommandMissing, simulator.CommandMissing)
}

func TestCommandExpectationOverflowIsDistinct(t *testing.T) {
	device := simulator.New(simulator.Scenario{
		Commands: []simulator.CommandExpectation{{
			Command: &pb.ButtonCommandRequest{Key: 1},
			Count:   65,
		}},
	})
	t.Cleanup(func() { _ = device.Close() })
	client := dialSimulator(t, device)
	t.Cleanup(func() { _ = client.Close() })
	for range 65 {
		if err := client.SendCommand(&pb.ButtonCommandRequest{Key: 1}); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	err := device.WaitForCommandExpectations(ctx)
	assertCommandExpectationError(t, err, simulator.ErrCommandOverflow, simulator.CommandOverflow)
	if device.Stats().DroppedCommands != 1 {
		t.Fatalf("DroppedCommands = %d, want 1", device.Stats().DroppedCommands)
	}
}

func TestCommandExpectationIsDefensivelyCopiedAndSecretSafe(t *testing.T) {
	const privateKey = uint32(777777)
	expected := &pb.ButtonCommandRequest{Key: privateKey}
	scenario := simulator.Scenario{
		Commands: []simulator.CommandExpectation{{Command: expected, Count: 1}},
	}
	device := simulator.New(scenario)
	t.Cleanup(func() { _ = device.Close() })
	expected.Key = 9
	scenario.Commands[0] = simulator.CommandExpectation{Command: &pb.ButtonCommandRequest{Key: 10}, Count: 2}
	client := dialSimulator(t, device)
	t.Cleanup(func() { _ = client.Close() })
	if err := client.SendCommand(&pb.ButtonCommandRequest{Key: privateKey}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := client.Ping(ctx); err != nil {
		t.Fatalf("Ping command barrier: %v", err)
	}
	if err := device.WaitForCommandExpectations(ctx); err != nil {
		t.Fatalf("defensive copy was not matched: %v", err)
	}
	if err := client.SendCommand(&pb.ButtonCommandRequest{Key: privateKey + 1}); err != nil {
		t.Fatal(err)
	}
	if err := client.Ping(ctx); err != nil {
		t.Fatalf("Ping trailing-command barrier: %v", err)
	}
	err := device.WaitForCommandExpectations(ctx)
	if !errors.Is(err, simulator.ErrCommandUnexpected) {
		t.Fatalf("error = %v, want ErrCommandUnexpected", err)
	}
	if strings.Contains(err.Error(), strconv.FormatUint(uint64(privateKey+1), 10)) {
		t.Fatalf("error leaked command key: %v", err)
	}
}

func TestNoCommandExpectationsKeepsExploratoryMode(t *testing.T) {
	device := simulator.New(simulator.Scenario{})
	t.Cleanup(func() { _ = device.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := device.WaitForCommandExpectations(ctx); err != nil {
		t.Fatalf("empty command expectations should succeed: %v", err)
	}
}

func assertCommandExpectationError(t *testing.T, err, sentinel error, code simulator.CommandExpectationCode) {
	t.Helper()
	if !errors.Is(err, simulator.ErrCommandExpectation) || !errors.Is(err, sentinel) {
		t.Fatalf("error = %v, want %v and %v", err, simulator.ErrCommandExpectation, sentinel)
	}
	var expectationErr *simulator.CommandExpectationError
	if !errors.As(err, &expectationErr) || expectationErr.Code != code {
		t.Fatalf("error = %#v, want code %s", err, code)
	}
}
