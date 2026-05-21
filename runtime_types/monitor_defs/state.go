package monitor_defs

import (
	"fmt"

	rt "github.com/focs-lab/gct/runtime_types"
	replay "github.com/focs-lab/gct/runtime_types/replay"
	"github.com/focs-lab/gct/scheduler/utils"
)

// ================================= Goroutine State =================================

type GoStatus int

const (
	WAITING GoStatus = iota // able to run but currently waiting for the scheduler to schedule it
	RUNNING
	BLOCKING // blocked on sync primitives
)

func (t GoStatus) String() string {
	switch t {
	case WAITING:
		return "WAITING"
	case RUNNING:
		return "RUNNING"
	case BLOCKING:
		return "BLOCKING"
	default:
		return fmt.Sprintf("Unknown BlockingType %d", t)
	}
}

type GoroutineState struct {
	GoId          rt.Goid
	WaitChan      chan *WakeupMessage
	Status        GoStatus       // for recording current status
	BlockingState *BlockingState // for recording blocking primitive and status
	Lid           *replay.LogicalId
	CurrEvent     *SyncEvent    // Next event this goroutine is about to take
	GoSpawnChan   chan struct{} // for synchronizing child gorotuines
}

func NewGoroutineState(goid rt.Goid, parent *replay.LogicalId) *GoroutineState {
	newLid := replay.NewLogicalId(parent)
	ret := &GoroutineState{
		GoId:     goid,
		WaitChan: make(chan *WakeupMessage, 1),
		Status:   RUNNING,
		BlockingState: &BlockingState{
			BType:   NOTBLOCKING,
			Blocker: nil,
		},
		Lid: newLid,
		CurrEvent: &SyncEvent{
			GoId:      goid,
			Lid:       newLid.StringName,
			EventType: NoOp,
		},
		GoSpawnChan: make(chan struct{}, 1),
	}

	return ret
}

// ================================= Blocking State =================================

type BlockingType int

const (
	MUTEX BlockingType = iota

	RWMUTEX_READ
	RWMUTEX_WRITE
	RWMUTEX_WRITE_WAITING_FOR_READERS

	CHANNEL_ASYNC_SEND
	CHANNEL_ASYNC_RECEIVE
	CHANNEL_SYNC_SEND
	CHANNEL_SYNC_RECEIVE
	NIL_CHANNEL_SEND_RECV
	TIMER_RECEIVE

	WAITGROUP

	NOTBLOCKING

	GOROUTINE_CREATION

	SELECT

	COND_VAR_WAIT
)

func (t BlockingType) String() string {
	switch t {
	case MUTEX:
		return "MUTEX"
	case RWMUTEX_READ:
		return "RWMUTEX_READ"
	case RWMUTEX_WRITE:
		return "RWMUTEX_WRITE"
	case RWMUTEX_WRITE_WAITING_FOR_READERS:
		return "RWMUTEX_WRITE_WAITING_FOR_READERS"
	case CHANNEL_ASYNC_SEND:
		return "CHANNEL_ASYNC_SEND"
	case CHANNEL_ASYNC_RECEIVE:
		return "CHANNEL_ASYNC_RECEIVE"
	case CHANNEL_SYNC_SEND:
		return "CHANNEL_SYNC_SEND"
	case CHANNEL_SYNC_RECEIVE:
		return "CHANNEL_SYNC_RECEIVE"
	case WAITGROUP:
		return "WAITGROUP"
	case NOTBLOCKING:
		return "NOTBLOCKING"
	case GOROUTINE_CREATION:
		return "GOROUTINE_CREATION"
	case SELECT:
		return "SELECT"
	case COND_VAR_WAIT:
		return "COND_VAR_WAIT"
	case NIL_CHANNEL_SEND_RECV:
		return "NIL_CHANNEL_SEND_RECV"
	case TIMER_RECEIVE:
		return "TIMER_RECEIVE"
	default:
		return fmt.Sprintf("Unknown BlockingType %d", t)
	}
}

type BlockingState struct {
	BType   BlockingType
	Blocker any
}

type SelectBlocker struct {
	Cases      []*SelectChoice
	HasDefault bool
}

func (s *SelectBlocker) GetCasesForChannel(ch any) []*SelectChoice {
	related := make([]*SelectChoice, 0)

	chPtr := utils.GetPtrOf(ch)

	for _, c := range s.Cases {
		currPtr := utils.GetPtrOf(c.Ch)
		if currPtr == chPtr {
			related = append(related, c)
		}
	}
	return related
}

// ================================= Sync Prim State =================================
type ChannelSendInfo struct {
	GoId rt.Goid
	OpId uint64
	Lid  string
}

func NewChannelSendInfo(goid rt.Goid, opid uint64, lid string) *ChannelSendInfo {
	return &ChannelSendInfo{
		GoId: goid,
		OpId: opid,
		Lid:  lid,
	}
}

type ChannelState struct {
	IsClosed   bool
	ClosedGoId rt.Goid
	ClosedLid  string
	ClosedOpId uint64
	Sends      []*ChannelSendInfo
	Capacity   int
	SendIdx    uint32
	StaticId   uint64 // OpId assigned at creation site
}

func NewChannelState(cap int, staticId uint64) *ChannelState {
	return &ChannelState{
		IsClosed:   false,
		ClosedGoId: 0,
		ClosedLid:  "",
		ClosedOpId: 0,
		Sends:      make([]*ChannelSendInfo, 0),
		Capacity:   cap,
		SendIdx:    0,
		StaticId:   staticId,
	}
}

type TimerType int

const (
	TIME_TICKER TimerType = iota
	TIME_TIMER
)

type TimerState struct {
	TType    TimerType
	Avaiable bool
	StaticId uint64 // OpId assigned at creation site
}

func NewTimerState() *TimerState {
	return &TimerState{
		TType:    TIME_TIMER,
		Avaiable: true,
		StaticId: 0,
	}
}

func NewTickerState() *TimerState {
	return &TimerState{
		TType:    TIME_TICKER,
		Avaiable: true,
		StaticId: 0,
	}
}

type MutexState struct {
	IsAcquired bool
}

func NewMutexState() *MutexState {
	return &MutexState{
		IsAcquired: false,
	}
}

type RWMutexState struct {
	WriterOwnedGoId rt.Goid
	WriterFlagGoId  rt.Goid
	Readers         int
}

func NewRWMutexState() *RWMutexState {
	return &RWMutexState{
		WriterOwnedGoId: 0,
		WriterFlagGoId:  0,
		Readers:         0,
	}
}

type WaitGroupState struct {
	Count int
}

func NewWaitGroupState() *WaitGroupState {
	return &WaitGroupState{
		Count: 0,
	}
}
