package monitor_defs

import (
	"fmt"
)

type CondVarWaitInfo struct {
	OpId uint64
	Lid  string
}

func NewCondVarWaitInfo(opId uint64, lid string) *CondVarWaitInfo {
	return &CondVarWaitInfo{
		OpId: opId,
		Lid:  lid,
	}
}

func NewNilCondVarWaitInfo() *CondVarWaitInfo {
	return &CondVarWaitInfo{
		OpId: 0,
		Lid:  "",
	}
}

func (info *CondVarWaitInfo) IsNilCondVarWaitInfo() bool {
	return info.OpId == 0 && info.Lid == ""
}

// For describing a single goroutine that can execute independently
type SingleGoroutineOption struct {
	Event     *SyncEvent
	IsSelect  bool
	WakeupMsg *WakeupMessage // Maintain messages sent back to monitor, including select case id, cond var wait goid etc
}

func NewSingleGoroutineOption(event *SyncEvent, wakeupMsg *WakeupMessage) *SingleGoroutineOption {
	ret := &SingleGoroutineOption{
		Event:     event,
		IsSelect:  event.EventType == Select,
		WakeupMsg: wakeupMsg,
	}

	return ret
}

// For describing a pair of goroutines that are blocked on sync channels or
// select of sync channels
type GoroutineRendezvousPairOption struct {
	Sender   *SyncEvent
	Receiver *SyncEvent
	SendOpId uint64
	RecvOpId uint64
	SendLId  string
	RecvLId  string

	IsSenderSelect   bool
	IsReceiverSelect bool
	SenderSelectId   int
	ReceiverSelectId int

	WakeupMsgSender   *WakeupMessage
	WakeupMsgReceiver *WakeupMessage
}

func NewGoroutineRendezvousPairOption(sender, receiver *SyncEvent, sendOpId, recvOpId uint64,
	sendLId, recvLId string, isSenderSelect, isReceiverSelect bool, senderSelectId, receiverSelectId int,
	wakeupMsgSender, wakeupMsgReceiver *WakeupMessage) *GoroutineRendezvousPairOption {

	return &GoroutineRendezvousPairOption{
		Sender:            sender,
		Receiver:          receiver,
		SendOpId:          sendOpId,
		RecvOpId:          recvOpId,
		SendLId:           sendLId,
		RecvLId:           recvLId,
		IsSenderSelect:    isSenderSelect,
		IsReceiverSelect:  isReceiverSelect,
		SenderSelectId:    senderSelectId,
		ReceiverSelectId:  receiverSelectId,
		WakeupMsgSender:   wakeupMsgSender,
		WakeupMsgReceiver: wakeupMsgReceiver,
	}
}

// If wakeup message is not nil, that means
// (1) this goroutine is waken up from blocked state
// (2) the message may contain select case id
type WakeupMessage struct {
	// for select
	SelectedCase *SelectChoice

	// for cond var
	CondVarWaitInfos []*CondVarWaitInfo
}

func NewWakeupMessage(selectedCase *SelectChoice, condVarWaitInfos []*CondVarWaitInfo) *WakeupMessage {
	return &WakeupMessage{
		SelectedCase:     selectedCase,
		CondVarWaitInfos: condVarWaitInfos,
	}
}

func (w WakeupMessage) String() string {
	if w.SelectedCase == nil {
		return "WakeupMessage[No Case]"
	}
	return fmt.Sprintf("WakeupMessage[%s]", w.SelectedCase.String())
}

const (
	SCHEDULER_WAKER = 0
)

type SelectChoice struct {
	BlockingType BlockingType
	Ch           any
	CaseIdx      int
}

func (s SelectChoice) String() string {
	info := fmt.Sprintf("BlockingType = %s, CaseIdx = %d", s.BlockingType.String(), s.CaseIdx)
	return info
}
