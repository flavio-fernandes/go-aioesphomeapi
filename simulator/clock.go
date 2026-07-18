package simulator

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	// ErrClockBackwards classifies attempts to move deterministic simulator time
	// backwards.
	ErrClockBackwards = errors.New("simulator clock cannot move backwards")
	// ErrClockOverflow classifies a duration addition that cannot be represented.
	ErrClockOverflow = errors.New("simulator clock overflow")
)

// ManualClock owns deterministic virtual time for one or more Devices. Time
// starts at zero and advances only when the caller explicitly requests it.
// Advance and AdvanceTo synchronously apply every due state event before they
// return.
type ManualClock struct {
	advanceMu sync.Mutex
	mu        sync.Mutex
	now       time.Duration
	devices   []*Device
}

// NewManualClock creates a stopped clock at virtual time zero.
func NewManualClock() *ManualClock {
	return &ManualClock{}
}

// Now returns the current virtual time.
func (c *ManualClock) Now() time.Duration {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

// Advance moves virtual time forward by delta.
func (c *ManualClock) Advance(delta time.Duration) error {
	if delta < 0 {
		return ErrClockBackwards
	}
	return c.advance(func(now time.Duration) (time.Duration, error) {
		if delta > time.Duration(1<<63-1)-now {
			return 0, ErrClockOverflow
		}
		return now + delta, nil
	})
}

// AdvanceTo moves virtual time to target.
func (c *ManualClock) AdvanceTo(target time.Duration) error {
	return c.advance(func(now time.Duration) (time.Duration, error) {
		if target < now {
			return 0, ErrClockBackwards
		}
		return target, nil
	})
}

func (c *ManualClock) advance(target func(time.Duration) (time.Duration, error)) error {
	if c == nil {
		return errors.New("nil simulator clock")
	}
	c.advanceMu.Lock()
	defer c.advanceMu.Unlock()

	c.mu.Lock()
	next, err := target(c.now)
	if err != nil {
		c.mu.Unlock()
		return err
	}
	c.now = next
	devices := append([]*Device(nil), c.devices...)
	c.mu.Unlock()

	for _, device := range devices {
		if err := device.advanceTimeline(next); err != nil {
			return fmt.Errorf("advance simulator timeline: %w", err)
		}
	}
	return nil
}

func (c *ManualClock) register(device *Device) {
	c.mu.Lock()
	c.devices = append(c.devices, device)
	c.mu.Unlock()
}

func (c *ManualClock) unregister(device *Device) {
	c.mu.Lock()
	for index, candidate := range c.devices {
		if candidate == device {
			copy(c.devices[index:], c.devices[index+1:])
			c.devices[len(c.devices)-1] = nil
			c.devices = c.devices[:len(c.devices)-1]
			break
		}
	}
	c.mu.Unlock()
}
