package simulator

import (
	"github.com/flavio-fernandes/go-aioesphomeapi/pb"
)

// FaultTrigger identifies the exact deterministic protocol point at which a
// simulated fault occurs.
type FaultTrigger string

const (
	FaultAfterHello         FaultTrigger = "after-hello"
	FaultBeforeEntitiesDone FaultTrigger = "before-entities-done"
	FaultAfterInitialStates FaultTrigger = "after-initial-states"
)

// FaultAction identifies one hostile-peer or network behavior. Actions do not
// contain real device data, random timing, or hidden retries.
type FaultAction string

const (
	FaultDropConnection        FaultAction = "drop-connection"
	FaultMalformedProtobuf     FaultAction = "malformed-protobuf"
	FaultUnknownMessage        FaultAction = "unknown-message"
	FaultDuplicateEntitiesDone FaultAction = "duplicate-entities-done"
	FaultStall                 FaultAction = "stall"
)

// Fault combines one named action with one exact trigger. If several faults
// share a trigger, they run in declaration order until a terminating action.
type Fault struct {
	Trigger FaultTrigger
	Action  FaultAction
}

func (d *Device) triggerFault(session *deviceSession, trigger FaultTrigger) bool {
	for _, fault := range d.scenario.Faults {
		if fault.Trigger != trigger {
			continue
		}
		switch fault.Action {
		case FaultDropConnection:
			return true
		case FaultMalformedProtobuf:
			// PingResponse is a known message ID, while 0x80 is a truncated
			// protobuf varint. The client must reject it without panicking.
			_ = session.writeFrame(8, []byte{0x80})
			return true
		case FaultUnknownMessage:
			// 65000 is outside the pinned ESPHome message inventory but is
			// valid in both framing type fields. A bounded unknown message is
			// forward-compatible and must not terminate the connection.
			if err := session.writeFrame(65000, nil); err != nil {
				return true
			}
			continue
		case FaultDuplicateEntitiesDone:
			// A buggy peer may complete discovery more than once. Both frames
			// traverse the real wire path; the normal completion follows too.
			if session.send(&pb.ListEntitiesDoneResponse{}) != nil {
				return true
			}
			if session.send(&pb.ListEntitiesDoneResponse{}) != nil {
				return true
			}
			continue
		case FaultStall:
			// Use no timer here. The caller's operation deadline is the only
			// real-time boundary. Device.Close or dropping this individual
			// connection releases the owned session task.
			select {
			case <-d.done:
			case <-session.network.closed:
			}
			return true
		}
	}
	return false
}
