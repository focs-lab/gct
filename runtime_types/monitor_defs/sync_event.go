package monitor_defs

import (
	"encoding/json"
	"fmt"
	"bytes"
	"encoding/gob"

	"github.com/focs-lab/gct/monitor/utils"
	"github.com/focs-lab/gct/runtime_types"
	"github.com/focs-lab/gct/runtime_types/replay"
)

type EventType int

const (
	// Mutex
	MutexLockCreation EventType = iota
	MutexLock 
	MutexUnlock

	// RWMutex
	RWMutexLockCreation
	RWMutexLockSetFlag // acquire write flag
	RWMutexLock        // actually acquire the write mutex
	RWMutexUnlock
	RWMutexRLock
	RWMutexRUnlock

	// CondVar
	CondVarCreation
	CondVarWait
	CondVarSignal
	CondVarBroadcast

	// WaitGroup
	WaitGroupCreation
	WaitGroupAdd
	WaitGroupDone
	WaitGroupWait

	// Channel
	ChannelSend
	ChannelReceive
	ChannelClose
	ChannelCreation

	// Goroutine
	GoroutineCreation
	GoroutineEnd
	NewGoroutineBegin
	MainGoroutineBegin
	MainGoroutineEnd

	// Select
	Select

	// Assertion
	Assertion

	// NoOp
	NoOp
)

func (e EventType) String() string {
	switch e {
	case MutexLock:
		return "Lock"
	case MutexUnlock:
		return "Unlock"
	case RWMutexLock:
		return "WLock"
	case RWMutexUnlock:
		return "WUnlock"
	case RWMutexRLock:
		return "RLock"
	case RWMutexRUnlock:
		return "RUnlock"
	case WaitGroupAdd:
		return "WGAdd"
	case WaitGroupDone:
		return "WGDone"
	case WaitGroupWait:
		return "WGWait"
	case ChannelSend:
		return "Send"
	case ChannelReceive:
		return "Receive"
	case ChannelClose:
		return "Close"
	case ChannelCreation:
		return "NewCh"
	case GoroutineCreation:
		return "fork"
	case NewGoroutineBegin:
		return "begin"
	case Select:
		return "Select"
	case CondVarCreation:
		return "NewCond"
	case CondVarWait:
		return "CondWait"
	case CondVarSignal:
		return "CondSignal"
	case CondVarBroadcast:
		return "CondBroadcast"
	default:
		return fmt.Sprintf("Unknown EventType %d", e)
	}
}

type SyncEvent struct {
	GoId runtime_types.Goid
	Lid  string

	IsBefore  bool // True if in BeforeXXX, False if in AfterXXX
	EventType EventType

	Target    any 
	OpId      uint64

	MetaInfo  any // Other Infomation
}

func NewSyncEvent(goid runtime_types.Goid, lid string, eventType EventType, target any, 
	opId uint64, isBefore bool) *SyncEvent {
	return &SyncEvent{
		GoId:       goid,
		Lid:        lid,
		EventType:  eventType,
		Target:     target, 
		OpId:       opId,
		IsBefore:   isBefore,
		MetaInfo:   nil,
	}
}

type gobSyncEvent struct {
	GoId      runtime_types.Goid
	Lid       string
	EventType EventType
	OpId      uint64
}

// Overwrite the default gob encoding to handle the non-serializable fields 
// in SyncEvent (e.g., Mutex, RWMutex, Channel, etc.)
func (e *SyncEvent) GobEncode() ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)

	// Use GobSyncEvent to serialize only the necessary fields.
	toEncode := gobSyncEvent{
		GoId:      e.GoId,
		Lid:       e.Lid,
		EventType: e.EventType,
		OpId:      e.OpId,
	}

	err := enc.Encode(toEncode)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (e *SyncEvent) GobDecode(data []byte) error {
	buf := bytes.NewBuffer(data)
	dec := gob.NewDecoder(buf)

	// Decode into a GobSyncEvent struct and then copy the fields back.
	var decoded gobSyncEvent
	if err := dec.Decode(&decoded); err != nil {
		return err
	}

	e.GoId = decoded.GoId
	e.Lid = decoded.Lid
	e.EventType = decoded.EventType
	e.OpId = decoded.OpId

	return nil
}

func (e *SyncEvent) getTargetString() string {
	target := e.Target

	if target == nil {
		return "nil"
	}

	switch underlying := target.(type) {
	case *replay.LogicalId:
		return underlying.String()

	default: // mutex, rwmutex, waitgroup, channel. - return pointer address
		return fmt.Sprintf("0x%x", utils.GetPtrOf(underlying))
	}
}

func (e *SyncEvent) String() string {
	// A temporary struct for JSON marshaling to handle fields that are not directly serializable.
	type jsonEvent struct {
		LID        string   `json:"lid"`
		EventType  string   `json:"event_type"`
		Target     string   `json:"target"`
		CaseID     int      `json:"case_id,omitempty"`
		HasDefault bool     `json:"has_default,omitempty"`
		Cases      []string `json:"cases,omitempty"`
	}

	je := jsonEvent{
		LID:       e.Lid,
		EventType: e.EventType.String(),
		Target:    e.getTargetString(),
	}

	b, err := json.Marshal(je)
	if err != nil {
		return fmt.Sprintf("{\"error\": \"failed to marshal SyncEvent to JSON: %v\"}", err)
	}

	return string(b)
}

func (e *SyncEvent) IsReadLike() bool {
	// Lock + Recv
	switch e.EventType {
	case RWMutexRLock, RWMutexLock:
		return true
	case ChannelReceive:
		return true

	case Select:
		// Select is read like iff one of the case is a recv
		sb, ok := e.Target.(*SelectBlocker)
		if !ok {
			return false
		}
		for _, currCase := range sb.Cases {
			switch currCase.BlockingType {
			case CHANNEL_ASYNC_RECEIVE, CHANNEL_SYNC_RECEIVE:
				return true
			default:
				continue
			}
		}
		return false

	default:
		return false
	}
}

func (e *SyncEvent) IsWriteLike() bool {
	switch e.EventType {
	case MutexUnlock:
		return true
	case RWMutexUnlock, RWMutexRUnlock:
		return true
	case ChannelSend, ChannelClose:
		return true
	case Select:
		return true
	default:
		return false
	}
}
