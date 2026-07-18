package simulator

import (
	"net"
	"sync"
	"time"
)

// NetworkAction identifies one deterministic server-to-client wire condition.
// Actions are armed at a protocol trigger and apply to the next response frame.
type NetworkAction string

const (
	// NetworkFragmentFrame splits the next response frame into one-byte writes.
	NetworkFragmentFrame NetworkAction = "fragment-frame"
	// NetworkCoalesceSegments combines every raw write used by the next response
	// frame into one underlying connection buffer before flushing it completely.
	NetworkCoalesceSegments NetworkAction = "coalesce-segments"
	// NetworkDelayReply holds the next response frame until virtual time advances
	// by NetworkFault.Delay.
	NetworkDelayReply NetworkAction = "delay-reply"

	// MaxNetworkDelay bounds one declared virtual network delay.
	MaxNetworkDelay = 24 * time.Hour
)

// NetworkFault applies one named network action at an exact protocol trigger.
// Delay is required only for NetworkDelayReply. Unknown actions have no effect,
// allowing a newer scenario to remain safe on an older simulator.
type NetworkFault struct {
	Trigger FaultTrigger
	Action  NetworkAction
	Delay   time.Duration
}

type networkConn struct {
	net.Conn
	device    *Device
	clock     *ManualClock
	closed    chan struct{}
	closeOnce sync.Once

	mu              sync.Mutex
	fragmentNext    bool
	coalesceNext    bool
	delayNextTarget time.Duration
	fragmentActive  bool
	coalesceActive  bool
	delayActive     time.Duration
	coalesced       []byte
	coalescedWrites uint64
}

func newNetworkConn(connection net.Conn, device *Device) *networkConn {
	return &networkConn{
		Conn:   connection,
		device: device,
		clock:  device.clock,
		closed: make(chan struct{}),
	}
}

func (c *networkConn) arm(action NetworkAction, delay time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	switch action {
	case NetworkFragmentFrame:
		c.fragmentNext = true
	case NetworkCoalesceSegments:
		c.coalesceNext = true
	case NetworkDelayReply:
		c.delayNextTarget = c.clock.Now() + delay
	}
}

func (c *networkConn) beginFrame() {
	c.mu.Lock()
	c.fragmentActive = c.fragmentNext
	c.fragmentNext = false
	c.coalesceActive = c.coalesceNext
	c.coalesceNext = false
	c.delayActive = c.delayNextTarget
	c.delayNextTarget = 0
	c.coalesced = c.coalesced[:0]
	c.coalescedWrites = 0
	fragmented := c.fragmentActive
	coalesced := c.coalesceActive
	delayed := c.delayActive > 0
	c.mu.Unlock()

	if fragmented {
		c.device.recordFragmentedFrame()
	}
	if coalesced {
		c.device.recordCoalescedFrame()
	}
	if delayed {
		c.device.recordDelayedFrame()
	}
}

func (c *networkConn) endFrame(frameErr error) error {
	c.mu.Lock()
	coalesced := c.coalesceActive
	buffer := append([]byte(nil), c.coalesced...)
	segments := c.coalescedWrites
	c.fragmentActive = false
	c.coalesceActive = false
	c.delayActive = 0
	c.coalesced = c.coalesced[:0]
	c.coalescedWrites = 0
	c.mu.Unlock()

	if frameErr != nil {
		return frameErr
	}
	if !coalesced {
		return nil
	}
	c.device.recordCoalescedSegments(segments)
	return c.writeAll(buffer)
}

func (c *networkConn) Write(payload []byte) (int, error) {
	for {
		select {
		case <-c.closed:
			return 0, net.ErrClosed
		default:
		}

		c.mu.Lock()
		if c.delayActive > 0 {
			target := c.delayActive
			c.delayActive = 0
			c.mu.Unlock()
			c.device.recordDelayedWait(1)
			err := c.clock.waitUntil(target, c.closed)
			c.device.recordDelayedWait(-1)
			if err != nil {
				return 0, err
			}
			continue
		}
		if c.coalesceActive {
			c.coalesced = append(c.coalesced, payload...)
			c.coalescedWrites++
			c.mu.Unlock()
			return len(payload), nil
		}
		fragmented := c.fragmentActive
		c.mu.Unlock()

		writePayload := payload
		if fragmented && len(writePayload) > 1 {
			writePayload = writePayload[:1]
		}
		if fragmented {
			c.device.recordFragmentedSegment()
		}
		return c.Conn.Write(writePayload)
	}
}

func (c *networkConn) writeAll(payload []byte) error {
	for len(payload) > 0 {
		select {
		case <-c.closed:
			return net.ErrClosed
		default:
		}
		written, err := c.Conn.Write(payload)
		if err != nil {
			return err
		}
		if written <= 0 {
			return net.ErrClosed
		}
		payload = payload[written:]
	}
	return nil
}

func (c *networkConn) Close() error {
	c.closeOnce.Do(func() { close(c.closed) })
	return c.Conn.Close()
}

func (d *Device) triggerNetwork(session *deviceSession, trigger FaultTrigger) {
	for _, fault := range d.scenario.Network {
		if fault.Trigger == trigger {
			session.network.arm(fault.Action, fault.Delay)
		}
	}
}

func (d *Device) recordFragmentedFrame() {
	d.networkMu.Lock()
	d.fragmentedFrames++
	d.networkMu.Unlock()
}

func (d *Device) recordFragmentedSegment() {
	d.networkMu.Lock()
	d.fragmentedSegments++
	d.networkMu.Unlock()
}

func (d *Device) recordCoalescedFrame() {
	d.networkMu.Lock()
	d.coalescedFrames++
	d.networkMu.Unlock()
}

func (d *Device) recordCoalescedSegments(segments uint64) {
	d.networkMu.Lock()
	d.coalescedSegments += segments
	d.networkMu.Unlock()
}

func (d *Device) recordDelayedFrame() {
	d.networkMu.Lock()
	d.delayedFrames++
	d.networkMu.Unlock()
}

func (d *Device) recordDelayedWait(delta int) {
	d.networkMu.Lock()
	if delta > 0 {
		d.pendingDelayedFrames += uint64(delta)
	} else if d.pendingDelayedFrames > 0 {
		d.pendingDelayedFrames--
	}
	d.networkMu.Unlock()
}
