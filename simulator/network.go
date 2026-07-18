package simulator

import (
	"errors"
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

	// networkFrameQueueSize bounds complete frames waiting behind a delayed
	// response. At the protocol maximum this caps queued ciphertext near 4 MiB.
	networkFrameQueueSize = 64
)

var errNetworkFrameQueueFull = errors.New("simulator network frame queue is full")

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
	device     *Device
	clock      *ManualClock
	closed     chan struct{}
	closeOnce  sync.Once
	closeErr   error
	frames     chan networkFrame
	writerDone sync.WaitGroup

	mu              sync.Mutex
	stopped         bool
	fragmentNext    bool
	coalesceNext    bool
	delayNextTarget time.Duration
	frameActive     bool
	fragmentActive  bool
	coalesceActive  bool
	delayActive     time.Duration
	segments        [][]byte
	asyncFrames     uint64
}

type networkFrame struct {
	segments     [][]byte
	fragment     bool
	coalesce     bool
	delayTarget  time.Duration
	asynchronous bool
	done         chan error
}

func newNetworkConn(connection net.Conn, device *Device) *networkConn {
	result := &networkConn{
		Conn:   connection,
		device: device,
		clock:  device.clock,
		closed: make(chan struct{}),
		frames: make(chan networkFrame, networkFrameQueueSize),
	}
	result.writerDone.Add(1)
	go result.writeLoop()
	return result
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
	c.frameActive = true
	c.fragmentActive = c.fragmentNext
	c.fragmentNext = false
	c.coalesceActive = c.coalesceNext
	c.coalesceNext = false
	c.delayActive = c.delayNextTarget
	c.delayNextTarget = 0
	c.segments = c.segments[:0]
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
	frame := networkFrame{
		segments:    c.segments,
		fragment:    c.fragmentActive,
		coalesce:    c.coalesceActive,
		delayTarget: c.delayActive,
	}
	frame.asynchronous = frame.delayTarget > 0 || c.asyncFrames > 0
	if frame.asynchronous {
		c.asyncFrames++
	} else {
		frame.done = make(chan error, 1)
	}
	c.frameActive = false
	c.fragmentActive = false
	c.coalesceActive = false
	c.delayActive = 0
	c.segments = nil
	c.mu.Unlock()

	if frameErr != nil {
		c.finishAsyncFrame(frame)
		return frameErr
	}
	if frame.coalesce {
		c.device.recordCoalescedSegments(uint64(len(frame.segments)))
	}
	if err := c.enqueue(frame); err != nil {
		c.finishAsyncFrame(frame)
		if errors.Is(err, errNetworkFrameQueueFull) {
			_ = c.stop()
		}
		return err
	}
	if frame.asynchronous {
		return nil
	}
	select {
	case err := <-frame.done:
		return err
	case <-c.closed:
		return net.ErrClosed
	}
}

func (c *networkConn) Write(payload []byte) (int, error) {
	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()
		return 0, net.ErrClosed
	}
	if c.frameActive {
		c.segments = append(c.segments, append([]byte(nil), payload...))
		c.mu.Unlock()
		return len(payload), nil
	}
	c.mu.Unlock()
	return c.Conn.Write(payload)
}

func (c *networkConn) enqueue(frame networkFrame) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.stopped {
		return net.ErrClosed
	}
	select {
	case c.frames <- frame:
		return nil
	default:
		return errNetworkFrameQueueFull
	}
}

func (c *networkConn) writeLoop() {
	defer c.writerDone.Done()
	for {
		select {
		case <-c.closed:
			return
		case frame := <-c.frames:
			err := c.writeFrame(frame)
			if frame.done != nil {
				frame.done <- err
			}
			c.finishAsyncFrame(frame)
			if err != nil {
				_ = c.stop()
				return
			}
		}
	}
}

func (c *networkConn) writeFrame(frame networkFrame) error {
	if frame.delayTarget > 0 {
		c.device.recordDelayedWait(1)
		err := c.clock.waitUntil(frame.delayTarget, c.closed)
		c.device.recordDelayedWait(-1)
		if err != nil {
			return err
		}
	}
	if frame.coalesce {
		var size int
		for _, segment := range frame.segments {
			size += len(segment)
		}
		payload := make([]byte, 0, size)
		for _, segment := range frame.segments {
			payload = append(payload, segment...)
		}
		return c.writeAll(payload)
	}
	if frame.fragment {
		for _, segment := range frame.segments {
			for index := range segment {
				if err := c.writeAll(segment[index : index+1]); err != nil {
					return err
				}
				c.device.recordFragmentedSegment()
			}
		}
		return nil
	}
	for _, segment := range frame.segments {
		if err := c.writeAll(segment); err != nil {
			return err
		}
	}
	return nil
}

func (c *networkConn) finishAsyncFrame(frame networkFrame) {
	if !frame.asynchronous {
		return
	}
	c.mu.Lock()
	if c.asyncFrames > 0 {
		c.asyncFrames--
	}
	c.mu.Unlock()
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
	err := c.stop()
	c.writerDone.Wait()
	return err
}

func (c *networkConn) stop() error {
	c.closeOnce.Do(func() {
		c.mu.Lock()
		c.stopped = true
		close(c.closed)
		c.mu.Unlock()
		c.closeErr = c.Conn.Close()
	})
	return c.closeErr
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
	d.notifyResourceChange()
}
