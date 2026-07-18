package simulator

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/protobuf/proto"
)

// CommandExpectation declares one exact command and how many consecutive
// times it must be received. Count must be between 1 and
// MaxCommandExpectationCount.
type CommandExpectation struct {
	Command proto.Message
	Count   uint32
}

// CommandExpectationCode is a stable, secret-safe command observation result.
type CommandExpectationCode string

const (
	CommandMissing    CommandExpectationCode = "missing"
	CommandUnexpected CommandExpectationCode = "unexpected"
	CommandOutOfOrder CommandExpectationCode = "out_of_order"
	CommandOverflow   CommandExpectationCode = "overflow"
)

var (
	// ErrCommandExpectation classifies all deterministic command expectation
	// failures.
	ErrCommandExpectation = errors.New("simulator command expectation failed")
	ErrCommandMissing     = errors.New("simulator command missing")
	ErrCommandUnexpected  = errors.New("simulator command unexpected")
	ErrCommandOutOfOrder  = errors.New("simulator command out of order")
	ErrCommandOverflow    = errors.New("simulator command observation overflow")
	errSimulatorClosed    = errors.New("simulator closed")
)

// CommandExpectationError identifies a command outcome without retaining or
// displaying protobuf values, entity keys, device names, or credentials.
// ExpectationIndex is zero-based. MatchedCount is the number already matched
// within that expectation, and ObservedCommands is the total received count.
type CommandExpectationError struct {
	Code             CommandExpectationCode
	ExpectationIndex int
	MatchedCount     uint32
	ObservedCommands uint64
	cause            error
}

func (e *CommandExpectationError) Error() string {
	return fmt.Sprintf("%v: %s (expectation_index=%d matched=%d observed=%d)",
		ErrCommandExpectation, e.Code, e.ExpectationIndex, e.MatchedCount, e.ObservedCommands)
}

// Unwrap preserves the broad category, the distinct outcome, and a context
// cancellation cause for missing commands.
func (e *CommandExpectationError) Unwrap() []error {
	causes := []error{ErrCommandExpectation, commandExpectationSentinel(e.Code)}
	if e.cause != nil {
		causes = append(causes, e.cause)
	}
	return causes
}

// WaitForCommandExpectations waits until every declared command and count has
// arrived in order. It returns immediately when Scenario.Commands is empty.
// A caller deadline reports CommandMissing while preserving context.Canceled
// or context.DeadlineExceeded through errors.Is.
//
// The helper proves receipt by the simulated protocol peer, not physical
// completion. Call it after the command-producing operation is quiescent when
// exact-count checking must also include possible trailing commands; later
// violations remain visible to subsequent calls.
func (d *Device) WaitForCommandExpectations(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	for {
		d.commandMu.Lock()
		if d.commandErr != nil {
			err := d.commandErr
			d.commandMu.Unlock()
			return err
		}
		if len(d.scenario.Commands) == 0 || d.commandIndex == len(d.scenario.Commands) {
			d.commandMu.Unlock()
			return nil
		}
		notify := d.commandNotify
		d.commandMu.Unlock()

		select {
		case <-notify:
			continue
		case <-ctx.Done():
			return d.missingCommandError(ctx.Err())
		case <-d.done:
			return d.missingCommandError(errSimulatorClosed)
		}
	}
}

func (d *Device) observeCommandLocked(command proto.Message) {
	if len(d.scenario.Commands) == 0 || d.commandErr != nil {
		return
	}
	d.commandObserved++
	if d.commandIndex == len(d.scenario.Commands) {
		d.setCommandErrorLocked(CommandUnexpected)
		return
	}
	expectation := d.scenario.Commands[d.commandIndex]
	if !proto.Equal(command, expectation.Command) {
		code := CommandUnexpected
		for index := d.commandIndex + 1; index < len(d.scenario.Commands); index++ {
			if proto.Equal(command, d.scenario.Commands[index].Command) {
				code = CommandOutOfOrder
				break
			}
		}
		d.setCommandErrorLocked(code)
		return
	}
	d.commandMatched++
	if d.commandMatched == expectation.Count {
		d.commandIndex++
		d.commandMatched = 0
	}
}

func (d *Device) setCommandErrorLocked(code CommandExpectationCode) {
	if d.commandErr != nil {
		return
	}
	d.commandErr = &CommandExpectationError{
		Code:             code,
		ExpectationIndex: d.commandIndex,
		MatchedCount:     d.commandMatched,
		ObservedCommands: d.commandObserved,
	}
}

func (d *Device) missingCommandError(cause error) error {
	d.commandMu.Lock()
	defer d.commandMu.Unlock()
	if d.commandErr != nil {
		return d.commandErr
	}
	if len(d.scenario.Commands) == 0 || d.commandIndex == len(d.scenario.Commands) {
		return nil
	}
	return &CommandExpectationError{
		Code:             CommandMissing,
		ExpectationIndex: d.commandIndex,
		MatchedCount:     d.commandMatched,
		ObservedCommands: d.commandObserved,
		cause:            cause,
	}
}

func (d *Device) notifyCommandWaitersLocked() {
	close(d.commandNotify)
	d.commandNotify = make(chan struct{})
}

func commandExpectationSentinel(code CommandExpectationCode) error {
	switch code {
	case CommandMissing:
		return ErrCommandMissing
	case CommandUnexpected:
		return ErrCommandUnexpected
	case CommandOutOfOrder:
		return ErrCommandOutOfOrder
	case CommandOverflow:
		return ErrCommandOverflow
	default:
		return ErrCommandExpectation
	}
}
