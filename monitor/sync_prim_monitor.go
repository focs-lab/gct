package monitor

import (
	"fmt"
	"runtime"
	"strings"
	"context"
	// runtime_debug "runtime/debug"
	"math/rand"
	"sync"
	"time"

	config "github.com/focs-lab/gct/config"
	utils "github.com/focs-lab/gct/monitor/utils"
	runtime_types "github.com/focs-lab/gct/runtime_types"
	monitor_defs "github.com/focs-lab/gct/runtime_types/monitor_defs"
	replay "github.com/focs-lab/gct/runtime_types/replay"
	scheduler_defs "github.com/focs-lab/gct/runtime_types/scheduler_defs"
	"github.com/focs-lab/gct/runtime_types/sync_interface"
	"github.com/focs-lab/gct/runtime_types/tracing"
)

type SyncPrimitiveMonitor struct {
	scheduler scheduler_defs.Scheduler
	rnd       *rand.Rand

	// goroutine related
	goroutineTable        map[Goid]*monitor_defs.GoroutineState
	goroutines            []Goid // list of goroutine ids.
	numRunningGoroutines  int
	numCreatingGoroutines int
	mainGoroutineID       Goid
	goroutineMutex        *sync.Mutex

	// channel related
	channels map[uintptr]*monitor_defs.ChannelState
	// channelsMutex *sync.Mutex
	channelsMutex *sync.Mutex

	// mutex related
	mutexStates map[*sync.Mutex]*monitor_defs.MutexState
	mutexMutex  *sync.Mutex

	// rwmutex related
	rwmutexStates map[*sync.RWMutex]*monitor_defs.RWMutexState
	rwmutexMutex  *sync.Mutex

	// waitgroup related
	waitgroupStates map[*sync.WaitGroup]*monitor_defs.WaitGroupState
	waitgroupMutex  *sync.Mutex

	// cond var related
	condVarWaitSet map[*sync.Cond]map[string]uint64 // addr -> lid -> Wait OpId
	condVarMutex   *sync.Mutex

	// time related
	timerCToTimerState map[uintptr]*monitor_defs.TimerState // timer.C -> 
	timerMap           map[uintptr]*monitor_defs.TimerState // timer ->
	timerMutex         *sync.Mutex

	// record related
	shouldRecord bool
	recorder     replay.Recorder

	// tracing related
	shouldTrace bool
	tracer      *tracing.Tracer
}

func NewSyncPrimitiveMonitor(scheduler scheduler_defs.Scheduler, recordOption, traceOption bool) *SyncPrimitiveMonitor {
	monitor := &SyncPrimitiveMonitor{
		scheduler: scheduler,
		rnd:       rand.New(rand.NewSource(0)),

		goroutineTable:        make(map[Goid]*monitor_defs.GoroutineState),
		goroutines:            make([]Goid, 0),
		numRunningGoroutines:  0,
		numCreatingGoroutines: 0,
		goroutineMutex:        &sync.Mutex{},

		channels:      make(map[uintptr]*monitor_defs.ChannelState),
		channelsMutex: &sync.Mutex{},

		mutexStates: make(map[*sync.Mutex]*monitor_defs.MutexState),
		mutexMutex:  &sync.Mutex{},

		rwmutexStates: make(map[*sync.RWMutex]*monitor_defs.RWMutexState),
		rwmutexMutex:  &sync.Mutex{},

		waitgroupStates: make(map[*sync.WaitGroup]*monitor_defs.WaitGroupState),
		waitgroupMutex:  &sync.Mutex{},

		condVarWaitSet: make(map[*sync.Cond]map[string]uint64),
		condVarMutex:   &sync.Mutex{},

		timerCToTimerState: make(map[uintptr]*monitor_defs.TimerState),
		timerMap:           make(map[uintptr]*monitor_defs.TimerState),
		timerMutex:         &sync.Mutex{},

		shouldRecord: recordOption,
		recorder:     replay.NewBaseRecorder(),

		shouldTrace: traceOption,
		tracer:      tracing.NewTracer(),
	}

	return monitor
}

// ================================ Exposed functions to Scheduler =================================
func (monitor *SyncPrimitiveMonitor) IsNextOperationReadLike(goid Goid) bool {
	// always called with goroutineMutex held
	goState, ok := monitor.goroutineTable[goid]
	if !ok {
		return false
	}

	return goState.CurrEvent.IsReadLike()
}

func (monitor *SyncPrimitiveMonitor) IsNextOperationWriteLike(goid Goid) bool {
	// always called with goroutineMutex held
	goState, ok := monitor.goroutineTable[goid]
	if !ok {
		return false
	}

	return goState.CurrEvent.IsWriteLike()
}

func (monitor *SyncPrimitiveMonitor) CheckConflict(goid1, goid2 Goid) bool {
	// always called with goroutineMutex held
	state1, ok1 := monitor.goroutineTable[goid1]
	state2, ok2 := monitor.goroutineTable[goid2]
	if !ok1 || !ok2 {
		return false
	}

	e1 := state1.CurrEvent
	e2 := state2.CurrEvent

	opPtr1 := utils.GetPtrOf(e1.Target)
	opPtr2 := utils.GetPtrOf(e2.Target)

	if e1 == nil || e2 == nil || opPtr1 == 0 || opPtr2 == 0 {
		return false
	}

	// Two operations conflict if they target the same synchronization object.
	// For select statements, the Target is nil because it's complex,
	// so we fall back to checking the BlockingState, which contains the detailed select cases.
	if e1.EventType != e2.EventType {
		return false
	}

	if opPtr1 == opPtr2 {
		return true
	}

	return false
}

func (monitor *SyncPrimitiveMonitor) GetChannelState(chAddr uintptr) (*monitor_defs.ChannelState, bool) {
	// always called with goroutineMutex held
	monitor.channelsMutex.Lock()
	defer monitor.channelsMutex.Unlock()

	chState, ok := monitor.channels[chAddr]
	return chState, ok
}

func (monitor *SyncPrimitiveMonitor) GetExecutableChoices() ([]*SingleGoroutineOption, []*SingleGoroutineOption, []*GoroutineRendezvousPairOption) {
	waiting := monitor.GetWaitingGoroutineOptions()
	enabled := monitor.GetEnabledGoroutines()
	pairs := monitor.GetGoroutineRendezvousPair()
	return waiting, enabled, pairs
}

func (monitor *SyncPrimitiveMonitor) GetWaitingGoroutineOptions() []*SingleGoroutineOption {
	// always called with goroutineMutex held
	waitingGoId := monitor.GetGoroutinesWithStatus(monitor_defs.WAITING)
	ret := make([]*SingleGoroutineOption, 0)
	for _, goid := range waitingGoId {
		goState := monitor.goroutineTable[goid]
		if goState == nil || goState.CurrEvent == nil {
			panic("SyncPrimitiveMonitor.GetWaitingGoroutineOptions() : nil goState or CurrEvent")
		}
		currEvent := goState.CurrEvent

		var msg *monitor_defs.WakeupMessage
		switch currEvent.EventType {
		case monitor_defs.CondVarSignal, monitor_defs.CondVarBroadcast:
			// Create a wakeup message with a nil CondVarWaitInfo.
			// The OptionFilter will populate this later.
			c := currEvent.Target.(*sync.Cond)
			monitor.condVarMutex.Lock()
			waitMap := monitor.condVarWaitSet[c]
			if len(waitMap) == 0 {
				info := monitor_defs.NewNilCondVarWaitInfo()
				msg = monitor_defs.NewWakeupMessage(nil, []*monitor_defs.CondVarWaitInfo{info})
				ret = append(ret, monitor_defs.NewSingleGoroutineOption(currEvent, msg))
			} else {
				for lid, opId := range waitMap {
					info := monitor_defs.NewCondVarWaitInfo(opId, lid)
					msg = monitor_defs.NewWakeupMessage(nil, []*monitor_defs.CondVarWaitInfo{info})
					ret = append(ret, monitor_defs.NewSingleGoroutineOption(currEvent, msg))
				}
			}

			monitor.condVarMutex.Unlock()

		default:
			curr := monitor_defs.NewSingleGoroutineOption(currEvent, msg)
			ret = append(ret, curr)
		}

	}
	return ret
}

func (monitor *SyncPrimitiveMonitor) GetGoroutinesWithStatus(expected monitor_defs.GoStatus) []Goid {
	// always called with goroutineMutex held
	goroutines := make([]Goid, 0)
	for _, goid := range monitor.goroutines {
		if monitor.goroutineTable[goid].Status == expected {
			goroutines = append(goroutines, goid)
		}
	}
	return goroutines
}

func (monitor *SyncPrimitiveMonitor) GetGoroutineRendezvousPair() []*GoroutineRendezvousPairOption {
	// always called with goroutineMutex held
	// returns all pairs of possible sync channel communication pairs
	blockedGoroutinePairs := make([]*GoroutineRendezvousPairOption, 0)

	blocked := monitor.GetGoroutinesWithStatus(monitor_defs.BLOCKING)
	size := len(blocked)

	for idx1, goid1 := range blocked {
		for idx2 := idx1 + 1; idx2 < size; idx2++ {
			goid2 := blocked[idx2]

			if pair, ok := monitor.isGoroutineRendezvousPair(goid1, goid2); ok {
				blockedGoroutinePairs = append(blockedGoroutinePairs, pair...)
			}
		}
	}

	return blockedGoroutinePairs
}

// return the list of goroutines that blocks on some blocker
// but now this doesn't block any more.
// For example, g1 blocks on a mutex m, because m is taken by another goroutine g2,
// but now g2 releases m, so g1 will be enabled.
func (monitor *SyncPrimitiveMonitor) GetEnabledGoroutines() []*SingleGoroutineOption {
	// always called with gorotuineMutex held
	ret := make([]*SingleGoroutineOption, 0)

	allBlocked := monitor.GetGoroutinesWithStatus(monitor_defs.BLOCKING)

	for _, goid := range allBlocked {
		goState := monitor.goroutineTable[goid]
		BType := goState.BlockingState.BType

		switch BType {
		case monitor_defs.SELECT:
			if ls, ok := monitor.isSelectEnabled(goid); ok {
				ret = append(ret, ls...)
			}

		case monitor_defs.CHANNEL_ASYNC_RECEIVE, monitor_defs.CHANNEL_ASYNC_SEND:
			if ls, ok := monitor.isAsyncChannelEnabled(goid, BType); ok {
				ret = append(ret, ls)
			}

		case monitor_defs.CHANNEL_SYNC_RECEIVE, monitor_defs.CHANNEL_SYNC_SEND:
			if ls, ok := monitor.isSyncChannelEnabled(goid, BType); ok {
				ret = append(ret, ls)
			}

		case monitor_defs.TIMER_RECEIVE:
			if ls, ok := monitor.isTimerChannelEnabled(goid, BType, goState.BlockingState.Blocker); ok {
				ret = append(ret, ls)
			}

		case monitor_defs.MUTEX:
			if ls, ok := monitor.isMutexEnabled(goid); ok {
				ret = append(ret, ls)
			}

		case monitor_defs.RWMUTEX_READ, monitor_defs.RWMUTEX_WRITE, monitor_defs.RWMUTEX_WRITE_WAITING_FOR_READERS:
			if ls, ok := monitor.isRWMutexEnabled(goid, BType); ok {
				ret = append(ret, ls)
			}

		case monitor_defs.WAITGROUP:
			if ls, ok := monitor.isWaitGroupEnabled(goid); ok {
				ret = append(ret, ls)
			}
		case monitor_defs.GOROUTINE_CREATION:
			if ls, ok := monitor.isGoroutineCreationEnabled(goid); ok {
				ret = append(ret, ls)
			}

		case monitor_defs.COND_VAR_WAIT:
			if ls, ok := monitor.isCondVarWaitEnabled(goid); ok {
				ret = append(ret, ls)
			}
		}
	}

	// if debug {
	// 	for _, t := range ret {
	// 		fmt.Printf("Goroutine[%d] is enabled %v\n", t.First, t.Second)
	// 	}
	// }

	return ret
}

// ================================ Helper functions =================================

func (monitor *SyncPrimitiveMonitor) isGoroutineRendezvousPair(goid1 Goid, goid2 Goid) ([]*GoroutineRendezvousPairOption, bool) {
	// Always called with gorotuineMutex held
	// There can be multiple GoroutineRendezvousPair generated by the same goroutine pair
	// Because they can select on different channels (select cases) if one of them is blocked on select
	goState1 := monitor.goroutineTable[goid1]
	goState2 := monitor.goroutineTable[goid2]

	if goState1 == nil || goState2 == nil || goState1.CurrEvent == nil || goState2.CurrEvent == nil {
		return nil, false
	}

	currEvent1 := goState1.CurrEvent
	currEvent2 := goState2.CurrEvent

	bState1 := goState1.BlockingState
	bState2 := goState2.BlockingState

	pairs := make([]*GoroutineRendezvousPairOption, 0)
	flag := false

	if bState1.BType == monitor_defs.CHANNEL_SYNC_SEND && bState2.BType == monitor_defs.CHANNEL_SYNC_RECEIVE {
		if utils.GetPtrOf(bState1.Blocker) == utils.GetPtrOf(bState2.Blocker) {
			pair := monitor_defs.NewGoroutineRendezvousPairOption(currEvent1, currEvent2, currEvent1.OpId, currEvent2.OpId,
				goState1.Lid.StringName, goState2.Lid.StringName, false, false, -1, -1,
				monitor_defs.NewWakeupMessage(nil, nil), monitor_defs.NewWakeupMessage(nil, nil))

			pairs = append(pairs, pair)
			flag = true
		}
	} else if bState1.BType == monitor_defs.CHANNEL_SYNC_RECEIVE && bState2.BType == monitor_defs.CHANNEL_SYNC_SEND {
		if utils.GetPtrOf(bState1.Blocker) == utils.GetPtrOf(bState2.Blocker) {
			pair := monitor_defs.NewGoroutineRendezvousPairOption(currEvent2, currEvent1, currEvent2.OpId, currEvent1.OpId,
				goState2.Lid.StringName, goState1.Lid.StringName, false, false, -1, -1,
				monitor_defs.NewWakeupMessage(nil, nil), monitor_defs.NewWakeupMessage(nil, nil))
			pairs = append(pairs, pair)
			flag = true
		}
	} else if bState1.BType == monitor_defs.SELECT && bState2.BType == monitor_defs.CHANNEL_SYNC_SEND {
		ch := bState2.Blocker

		if b1SelectBlocker, ok := bState1.Blocker.(*monitor_defs.SelectBlocker); ok {
			relatedCases := b1SelectBlocker.GetCasesForChannel(ch)
			for _, c := range relatedCases {
				if c.BlockingType == monitor_defs.CHANNEL_SYNC_RECEIVE {
					pair := monitor_defs.NewGoroutineRendezvousPairOption(currEvent2, currEvent1, currEvent2.OpId, currEvent1.OpId,
						goState2.Lid.StringName, goState1.Lid.StringName, false, true, -1, c.CaseIdx,
						monitor_defs.NewWakeupMessage(nil, nil), monitor_defs.NewWakeupMessage(c, nil))
					pairs = append(pairs, pair)
					flag = true
				}
			}
		}
	} else if bState1.BType == monitor_defs.SELECT && bState2.BType == monitor_defs.CHANNEL_SYNC_RECEIVE {
		ch := bState2.Blocker

		if b1SelectBlocker, ok := bState1.Blocker.(*monitor_defs.SelectBlocker); ok {
			relatedCases := b1SelectBlocker.GetCasesForChannel(ch)
			for _, c := range relatedCases {
				if c.BlockingType == monitor_defs.CHANNEL_SYNC_SEND {
					pair := monitor_defs.NewGoroutineRendezvousPairOption(currEvent1, currEvent2, currEvent1.OpId, currEvent2.OpId,
						goState1.Lid.StringName, goState2.Lid.StringName, true, false, c.CaseIdx, -1,
						monitor_defs.NewWakeupMessage(c, nil), monitor_defs.NewWakeupMessage(nil, nil))
					pairs = append(pairs, pair)
					flag = true
				}
			}
		}
	} else if bState1.BType == monitor_defs.CHANNEL_SYNC_SEND && bState2.BType == monitor_defs.SELECT {
		ch := bState1.Blocker

		if b2SelectBlocker, ok := bState2.Blocker.(*monitor_defs.SelectBlocker); ok {
			relatedCases := b2SelectBlocker.GetCasesForChannel(ch)
			for _, c := range relatedCases {
				if c.BlockingType == monitor_defs.CHANNEL_SYNC_RECEIVE {
					pair := monitor_defs.NewGoroutineRendezvousPairOption(currEvent1, currEvent2, currEvent1.OpId, currEvent2.OpId,
						goState1.Lid.StringName, goState2.Lid.StringName, false, true, -1, c.CaseIdx,
						monitor_defs.NewWakeupMessage(nil, nil), monitor_defs.NewWakeupMessage(c, nil))
					pairs = append(pairs, pair)
					flag = true
				}
			}
		}
	} else if bState1.BType == monitor_defs.CHANNEL_SYNC_RECEIVE && bState2.BType == monitor_defs.SELECT {
		ch := bState1.Blocker

		if b2SelectBlocker, ok := bState2.Blocker.(*monitor_defs.SelectBlocker); ok {
			relatedCases := b2SelectBlocker.GetCasesForChannel(ch)
			for _, c := range relatedCases {
				if c.BlockingType == monitor_defs.CHANNEL_SYNC_SEND {
					pair := monitor_defs.NewGoroutineRendezvousPairOption(currEvent2, currEvent1, currEvent2.OpId, currEvent1.OpId,
						goState2.Lid.StringName, goState1.Lid.StringName, true, false, c.CaseIdx, -1,
						monitor_defs.NewWakeupMessage(c, nil), monitor_defs.NewWakeupMessage(nil, nil))
					pairs = append(pairs, pair)
					flag = true
				}
			}
		}
	} else if bState1.BType == monitor_defs.SELECT && bState2.BType == monitor_defs.SELECT {
		if b1SelectBlocker, ok := bState1.Blocker.(*monitor_defs.SelectBlocker); ok {
			if b2SelectBlocker, ok := bState2.Blocker.(*monitor_defs.SelectBlocker); ok {
				for _, c1 := range b1SelectBlocker.Cases {
					for _, c2 := range b2SelectBlocker.Cases {
						if utils.GetPtrOf(c1.Ch) == utils.GetPtrOf(c2.Ch) {
							if c1.BlockingType == monitor_defs.CHANNEL_SYNC_RECEIVE && c2.BlockingType == monitor_defs.CHANNEL_SYNC_SEND {
								pair := monitor_defs.NewGoroutineRendezvousPairOption(currEvent2, currEvent1, currEvent2.OpId, currEvent1.OpId,
									goState2.Lid.StringName, goState1.Lid.StringName, true, true, c2.CaseIdx, c1.CaseIdx,
									monitor_defs.NewWakeupMessage(c2, nil), monitor_defs.NewWakeupMessage(c1, nil))
								pairs = append(pairs, pair)
								flag = true
							} else if c1.BlockingType == monitor_defs.CHANNEL_SYNC_SEND && c2.BlockingType == monitor_defs.CHANNEL_SYNC_RECEIVE {
								pair := monitor_defs.NewGoroutineRendezvousPairOption(currEvent1, currEvent2, currEvent1.OpId, currEvent2.OpId,
									goState1.Lid.StringName, goState2.Lid.StringName, true, true, c1.CaseIdx, c2.CaseIdx,
									monitor_defs.NewWakeupMessage(c1, nil), monitor_defs.NewWakeupMessage(c2, nil))
								pairs = append(pairs, pair)
								flag = true
							}
						}
					}
				}
			}
		}
	}

	return pairs, flag
}

func (monitor *SyncPrimitiveMonitor) PrintGoroutineStatus() {
	for curr_id, curr_state := range monitor.goroutineTable {
		if curr_state.Status == monitor_defs.BLOCKING {
			if curr_state.BlockingState.BType == monitor_defs.SELECT && curr_state.BlockingState.Blocker != nil {
				if sb, ok := curr_state.BlockingState.Blocker.(*monitor_defs.SelectBlocker); ok {
					var cases []string
					for _, c := range sb.Cases {
						if c.CaseIdx == -1 {
							cases = append(cases, "default")
						} else {
							cases = append(cases, fmt.Sprintf("case %d (%s on ch %p)", c.CaseIdx, c.BlockingType, c.Ch))
						}
					}
					fmt.Printf("Goroutine[%d] is at status %s, blockingState: %s on cases: [%s]\n", curr_id,
						curr_state.Status, curr_state.BlockingState.BType, strings.Join(cases, ", "))
				}
			} else {
				fmt.Printf("Goroutine[%d] is at status %s, blockingState: %s, blocker: %v \n", curr_id,
					curr_state.Status, curr_state.BlockingState.BType, curr_state.BlockingState.Blocker)
			}
		} else {
			fmt.Printf("Goroutine[%d] is at status %s \n", curr_id, curr_state.Status)
		}
	}
}

func (monitor *SyncPrimitiveMonitor) PrintSchedulingChoices(waiting []*SingleGoroutineOption, enabled []*SingleGoroutineOption, pairs []*GoroutineRendezvousPairOption) {
	fmt.Println("========================================\n")
	if len(waiting) == 0 && len(enabled) == 0 && len(pairs) == 0 {
		fmt.Println("  No scheduling choices available.")
	} else {
		if len(waiting) > 0 {
			fmt.Println("\n --- Waiting Goroutines (always runnable) ---")
			for _, option := range waiting {
				fmt.Printf("  - Goroutine %d\n", option.Event.GoId)
			}
		}

		if len(enabled) > 0 {
			fmt.Println("--- Enabled Goroutines (newly unblocked) ---")
			for _, option := range enabled {
				if option.WakeupMsg != nil && option.WakeupMsg.SelectedCase != nil {
					fmt.Printf("  - Goroutine %d (Select case %d)\n", option.Event.GoId, option.WakeupMsg.SelectedCase.CaseIdx)
				} else {
					fmt.Printf("  - Goroutine %d\n", option.Event.GoId)
				}
			}
		}

		if len(pairs) > 0 {
			fmt.Println("--- Rendezvous Pairs (sync channel communication) ---")
			for _, pair := range pairs {
				fmt.Printf("  - Sender: Goroutine %d <-> Receiver: Goroutine %d\n", pair.Sender, pair.Receiver)
			}
		}
	}

	fmt.Println("========================================\n")
}

func (monitor *SyncPrimitiveMonitor) isCondVarWaitEnabled(goid Goid) (*SingleGoroutineOption, bool) {
	// always called with goroutineMutex held
	var ret *SingleGoroutineOption
	ret = nil
	flag := false

	monitor.condVarMutex.Lock()
	defer monitor.condVarMutex.Unlock()

	goState := monitor.goroutineTable[goid]
	currEvent := goState.CurrEvent
	c := goState.BlockingState.Blocker.(*sync.Cond)
	waitMap, ok := monitor.condVarWaitSet[c]
	if !ok {
		panic("Untracked cond var!")
	}

	if _, ok := waitMap[goState.Lid.StringName]; !ok {
		msg := monitor_defs.NewWakeupMessage(nil, nil)
		ret = monitor_defs.NewSingleGoroutineOption(currEvent, msg)
		flag = true
	}

	return ret, flag
}

func (monitor *SyncPrimitiveMonitor) isGoroutineCreationEnabled(goid Goid) (*SingleGoroutineOption, bool) {
	// always called with goroutineMutex held
	// returns true if no goroutine is being created
	var enabled = false
	var ret *SingleGoroutineOption = nil

	if monitor.numCreatingGoroutines == 0 {
		enabled = true
		goState := monitor.goroutineTable[goid]
		currEvent := goState.CurrEvent
		msg := monitor_defs.NewWakeupMessage(nil, nil)
		ret = monitor_defs.NewSingleGoroutineOption(currEvent, msg)
	}

	return ret, enabled
}

func (monitor *SyncPrimitiveMonitor) isWaitGroupEnabled(goid Goid) (*SingleGoroutineOption, bool) {
	// always called with goroutineMutex held
	// returns true if this wg.Wait() is enabled
	goState := monitor.goroutineTable[goid]
	wg := goState.BlockingState.Blocker.(*sync.WaitGroup)
	enabled := false

	var ret *SingleGoroutineOption = nil
	monitor.waitgroupMutex.Lock()
	wgState := monitor.waitgroupStates[wg]
	count := wgState.Count
	monitor.waitgroupMutex.Unlock()

	if count == 0 {
		enabled = true
		currEvent := goState.CurrEvent
		msg := monitor_defs.NewWakeupMessage(nil, nil)
		ret = monitor_defs.NewSingleGoroutineOption(currEvent, msg)
	}

	return ret, enabled
}

func (monitor *SyncPrimitiveMonitor) isRWMutexEnabled(goid Goid, bType monitor_defs.BlockingType) (*SingleGoroutineOption, bool) {
	// always called with goroutineMutex held
	// returns true if this RWMutex is enabled
	goState := monitor.goroutineTable[goid]
	rwMutex := goState.BlockingState.Blocker.(*sync.RWMutex)
	enabled := false

	var ret *SingleGoroutineOption = nil

	monitor.rwmutexMutex.Lock()
	rwMutexState := monitor.rwmutexStates[rwMutex]
	numReaders := rwMutexState.Readers
	writerOwnedGoId := rwMutexState.WriterOwnedGoId
	writerFlagGoId := rwMutexState.WriterFlagGoId
	monitor.rwmutexMutex.Unlock()

	switch bType {
	case monitor_defs.RWMUTEX_READ:
		if writerFlagGoId == 0 && writerOwnedGoId == 0 {
			enabled = true
		}

	case monitor_defs.RWMUTEX_WRITE:
		if writerFlagGoId == 0 && writerOwnedGoId == 0 {
			enabled = true

			// Some goroutines still hold RLock, but we can enable this write lock attempt,
			// because RWMutex allows Write Lock to decrement the Readers count,
			// and acquire the write lock when #Readers becomes 0.
			// This will block later RLock attempts.

			// But we need to reschedule once, to make sure if this is chosen to run,
			// the system doesn't actually block.
		}

	case monitor_defs.RWMUTEX_WRITE_WAITING_FOR_READERS:
		if writerFlagGoId == goid && numReaders == 0 {
			enabled = true
		}
	}

	if enabled {
		currEvent := goState.CurrEvent
		msg := monitor_defs.NewWakeupMessage(nil, nil)
		ret = monitor_defs.NewSingleGoroutineOption(currEvent, msg)
	}

	return ret, enabled
}

func (monitor *SyncPrimitiveMonitor) isMutexEnabled(goid Goid) (*SingleGoroutineOption, bool) {
	// always called with goroutineMutex held
	// returns true if this async send/recv is enabled
	monitor.mutexMutex.Lock()
	defer monitor.mutexMutex.Unlock()

	goState := monitor.goroutineTable[goid]
	mutex := goState.BlockingState.Blocker.(*sync.Mutex)
	enabled := !monitor.mutexStates[mutex].IsAcquired

	var ret *SingleGoroutineOption = nil

	if enabled {
		currEvent := goState.CurrEvent
		msg := monitor_defs.NewWakeupMessage(nil, nil)
		ret = monitor_defs.NewSingleGoroutineOption(currEvent, msg)
	}

	return ret, enabled
}

func (monitor *SyncPrimitiveMonitor) isUntrackedChannelEnabled(goid Goid, bType monitor_defs.BlockingType, ch any) bool {
	// always called with goroutineMutex held
	cap := utils.GetChanCap(ch)
	len := utils.GetChanLen(ch)

	enabled := false

	if utils.IsChanClosed(ch) {
		return true
	}
	switch bType {
	case monitor_defs.CHANNEL_ASYNC_RECEIVE:
		if cap > 0 && len > 0 {
			enabled = true
		}

	case monitor_defs.CHANNEL_ASYNC_SEND:
		if cap > 0 && cap > len {
			enabled = true
		}

	case monitor_defs.CHANNEL_SYNC_RECEIVE:
		hasSend, _ := utils.CheckSyncChanWaiters(ch)
		enabled = hasSend

	case monitor_defs.CHANNEL_SYNC_SEND:
		_, hasRecv := utils.CheckSyncChanWaiters(ch)
		enabled = hasRecv

	default:
		panic("Unexpected bType for isUntrackedChannelEnabled")
	}

	println("Untracked channel with cap", cap, "and len", len, "enabled:", enabled)

	return enabled
}

func (monitor *SyncPrimitiveMonitor) isTimerChannelEnabled(goid Goid, bType monitor_defs.BlockingType, ch any) (*SingleGoroutineOption, bool) {
	// always called with goroutineMutex held
	goState := monitor.goroutineTable[goid]
	chPtr := utils.GetPtrOf(ch)
	enabled := false
	var ret *SingleGoroutineOption = nil

	monitor.timerMutex.Lock()
	timerState := monitor.timerCToTimerState[chPtr]

	if ch != nil && chPtr != 0 {
		if timerState.Avaiable {
			enabled = true
		}
	}

	if enabled {
		currEvent := goState.CurrEvent
		msg := monitor_defs.NewWakeupMessage(nil, nil)
		ret = monitor_defs.NewSingleGoroutineOption(currEvent, msg)
	}

	monitor.timerMutex.Unlock()

	return ret, enabled
}

func (monitor *SyncPrimitiveMonitor) isSyncChannelEnabled(goid Goid, bType monitor_defs.BlockingType) (*SingleGoroutineOption, bool) {
	// always called with goroutineMutex held
	// returns true if this sync send/recv is enabled
	goState := monitor.goroutineTable[goid]
	ch := goState.BlockingState.Blocker
	chPtr := utils.GetPtrOf(ch)
	enabled := false
	var ret *SingleGoroutineOption = nil

	monitor.channelsMutex.Lock()
	chState := monitor.channels[chPtr]

	// send/recv of nil channel is blocked forever
	if ch != nil && chPtr != 0 {
		if chState == nil {
			// untracked, manual check
			enabled = monitor.isUntrackedChannelEnabled(goid, bType, ch)
		} else if chState.IsClosed {
			// send/recv to closed channels are enabled
			enabled = true
		}
	}

	if enabled {
		currEvent := goState.CurrEvent
		msg := monitor_defs.NewWakeupMessage(nil, nil)
		ret = monitor_defs.NewSingleGoroutineOption(currEvent, msg)
	}

	monitor.channelsMutex.Unlock()

	return ret, enabled
}

func (monitor *SyncPrimitiveMonitor) isAsyncChannelEnabled(goid Goid, bType monitor_defs.BlockingType) (*SingleGoroutineOption, bool) {
	// always called with goroutineMutex held
	// returns true if this async send/recv is enabled
	goState := monitor.goroutineTable[goid]
	ch := goState.BlockingState.Blocker
	chPtr := utils.GetPtrOf(ch)
	enabled := false
	var ret *SingleGoroutineOption = nil

	monitor.channelsMutex.Lock()
	chState := monitor.channels[chPtr]
	numMsgs := len(chState.Sends)
	cap := chState.Capacity
	isClosed := chState.IsClosed
	monitor.channelsMutex.Unlock()

	// send/recv of nil channel is blocked forever
	if ch != nil && chPtr != 0 {
		if chState == nil {
			// untracked, assume to be enabled
			enabled = monitor.isUntrackedChannelEnabled(goid, bType, ch)
		} else if isClosed {
			// send/recv to closed channels are enabled
			enabled = true
		} else if bType == monitor_defs.CHANNEL_ASYNC_RECEIVE && numMsgs > 0 {
			enabled = true
		} else if bType == monitor_defs.CHANNEL_ASYNC_SEND && numMsgs < cap {
			enabled = true
		}
	}

	if enabled {
		currEvent := goState.CurrEvent
		msg := monitor_defs.NewWakeupMessage(nil, nil)
		ret = monitor_defs.NewSingleGoroutineOption(currEvent, msg)
	}

	return ret, enabled
}

func (monitor *SyncPrimitiveMonitor) isSelectEnabled(goid Goid) ([]*SingleGoroutineOption, bool) {
	// Always called with goroutineMutex held
	// Returns all cases that can be executed independently, such as
	// (1) async send/recv, (2) send/recv to closed sync/async channels
	ret := make([]*SingleGoroutineOption, 0)

	goState := monitor.goroutineTable[goid]
	currEvent := goState.CurrEvent
	selectBlocker := goState.BlockingState.Blocker.(*monitor_defs.SelectBlocker)

	var defaultCase *monitor_defs.SelectChoice

	monitor.channelsMutex.Lock()
	defer monitor.channelsMutex.Unlock()

	// first search for non-default, enabled cases
	for idx, c := range selectBlocker.Cases {
		ch := c.Ch
		chPtr := utils.GetPtrOf(ch)
		enabled := false

		if c.BlockingType == monitor_defs.TIMER_RECEIVE {
			_, enabled = monitor.isTimerChannelEnabled(goid, c.BlockingType, ch)
		} else {
			chState := monitor.channels[chPtr]
			if chPtr == 0 {
				if c.BlockingType == monitor_defs.NOTBLOCKING && c.CaseIdx == -1 {
					defaultCase = c // default case
					continue
				}
				continue // nil channel, not enabled
			} else if chState == nil { // untracked channel
				enabled = monitor.isUntrackedChannelEnabled(goid, c.BlockingType, ch)
			} else if chState.IsClosed { // closed channel can always be sent/received
				enabled = true
			} else if c.BlockingType == monitor_defs.CHANNEL_ASYNC_RECEIVE && len(chState.Sends) > 0 {
				enabled = true
			} else if c.BlockingType == monitor_defs.CHANNEL_ASYNC_SEND && len(chState.Sends) < chState.Capacity {
				enabled = true
			}
		}

		if enabled {
			msg := monitor_defs.NewWakeupMessage(c, nil)
			newOption := monitor_defs.NewSingleGoroutineOption(currEvent, msg)
			ret = append(ret, newOption)
		}

		if debug {
			if !enabled {
				fmt.Printf("Select case %d, %v of Goroutine %d is not enabled\n", c.CaseIdx, c.BlockingType, goid)
			} else {
				fmt.Printf("Select case %d, %v of Goroutine %d is enabled\n", c.CaseIdx, c.BlockingType, goid)
			}

			if idx == len(selectBlocker.Cases)-1 {
				fmt.Println()
			}
		}
	}

	enabled := len(ret) > 0

	// then consider default case if not enabled
	if !enabled && defaultCase != nil {
		msg := monitor_defs.NewWakeupMessage(defaultCase, nil)
		newOption := monitor_defs.NewSingleGoroutineOption(currEvent, msg)
		ret = append(ret, newOption)
		enabled = true
	}

	// if debug {
	// for _, t := range ret {
	// 	if t.Second != nil && t.Second.SelectedCase != nil {
	// 		fmt.Printf("Select case %d is enabled for Goroutine %d\n", t.Second.SelectedCase.CaseIdx, t.First)
	// 	}
	// }
	// }

	return ret, enabled
}

func (monitor *SyncPrimitiveMonitor) setCurrEvent(goid Goid, eventType monitor_defs.EventType, isBefore bool, target any, opId uint64, metaInfo ...any) {
	// assumes goroutineMutex is held
	if goState, ok := monitor.goroutineTable[goid]; ok {
		goState.CurrEvent = monitor_defs.NewSyncEvent(goid, goState.Lid.StringName, eventType, target, opId, isBefore)
		if len(metaInfo) > 0 {
			goState.CurrEvent.MetaInfo = metaInfo[0]
		}
	}
}

func (monitor *SyncPrimitiveMonitor) yieldAsWaiting(goid Goid) *WakeupMessage {
	// must be called without holding the goroutineMutex
	monitor.goroutineMutex.Lock()
	goState := monitor.goroutineTable[goid]

	goState.Status = monitor_defs.WAITING
	monitor.numRunningGoroutines--

	monitor.goroutineMutex.Unlock()

	go monitor.schedule()

	msg := <-goState.WaitChan

	return msg
}

func (monitor *SyncPrimitiveMonitor) yieldAsBlocking(goid Goid, BType monitor_defs.BlockingType, blocker any) *WakeupMessage {
	monitor.goroutineMutex.Lock()
	goState := monitor.goroutineTable[goid]

	goState.Status = monitor_defs.BLOCKING
	goState.BlockingState.BType = BType
	goState.BlockingState.Blocker = blocker

	monitor.numRunningGoroutines--
	monitor.goroutineMutex.Unlock()

	go monitor.schedule()

	msg := <-goState.WaitChan

	return msg
}

func (monitor *SyncPrimitiveMonitor) wakeupGoroutinePair(pair *GoroutineRendezvousPairOption) {
	// always called with goroutineMutex locked
	senderGoid := pair.Sender.GoId
	receiverGoid := pair.Receiver.GoId

	monitor.wakeupGoroutine(senderGoid, pair.WakeupMsgSender)
	monitor.wakeupGoroutine(receiverGoid, pair.WakeupMsgReceiver)
}

func (monitor *SyncPrimitiveMonitor) wakeupGoroutine(targetId Goid, wakeupMsg *WakeupMessage) {
	// always called with goroutineMutex locked
	// msg is not nil only for goroutines blocked on select stmt
	goState := monitor.goroutineTable[targetId]
	goState.Status = monitor_defs.RUNNING
	goState.BlockingState.BType = monitor_defs.NOTBLOCKING
	goState.BlockingState.Blocker = nil
	monitor.numRunningGoroutines++

	goState.WaitChan <- wakeupMsg
}

func (monitor *SyncPrimitiveMonitor) removeGoroutine(goid Goid) {
	// always called with goroutineMutex locked
	idx := utils.GetPositionInSlice(monitor.goroutines, goid)
	if idx == -1 {
		panic("Goroutine ending but not found in goroutine list")
	}
	monitor.goroutines = append(monitor.goroutines[:idx], monitor.goroutines[idx+1:]...)
	goState := monitor.goroutineTable[goid]
	if goState.Status == monitor_defs.RUNNING {
		monitor.numRunningGoroutines--
	}
	delete(monitor.goroutineTable, goid)
}

// ================================= Goroutines =================================

func (monitor *SyncPrimitiveMonitor) BeforeGoroutineCreation(goid Goid) Goid {
	// if debug {
	// 	fmt.Printf("Goroutine [%d] creating new goroutine. Stack:\n%s\n", goid, string(runtime_debug.Stack()))
	// }
	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.GoroutineCreation, true, nil, 0)
	monitor.numCreatingGoroutines++
	monitor.goroutineMutex.Unlock()

	monitor.LogSyncEvent(goid)
	return goid
}

func (monitor *SyncPrimitiveMonitor) AfterNewGoroutineCreation(goid Goid, parentGoid Goid) {
	// CurrEvent is set when the goroutine is created, before it runs
	monitor.goroutineMutex.Lock()

	// setup Lid of children
	parentState := monitor.goroutineTable[parentGoid]
	parentLid := parentState.Lid
	parentGoSpawnCh := parentState.GoSpawnChan
	goState := monitor_defs.NewGoroutineState(goid, parentLid)

	monitor.setCurrEvent(goid, monitor_defs.NewGoroutineBegin, false, nil, 0)

	monitor.goroutineTable[goid] = goState
	monitor.numRunningGoroutines++
	monitor.goroutines = append(monitor.goroutines, goid)
	monitor.numCreatingGoroutines--

	monitor.goroutineMutex.Unlock()

	monitor.scheduler.OnNewGoroutineBegin(goid, false)

	parentGoSpawnCh <- struct{}{}

	monitor.LogSyncEvent(goid)

	monitor.yieldAsWaiting(goid)
}

func (monitor *SyncPrimitiveMonitor) AfterNewGoroutineCreationCreator(goid Goid) {
	// wait until no other goroutines are being created
	// if debug {
	// 	fmt.Printf("Goroutine [%d] AfterNewGoroutineCreationCreator. Stack:\n%s\n", goid, string(runtime_debug.Stack()))
	// }
	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.GoroutineCreation, false, nil, 0)

	goState := monitor.goroutineTable[goid]
	spawnCh := goState.GoSpawnChan
	monitor.goroutineMutex.Unlock()

	<-spawnCh // block until child gorotuine finishes creation

	// when wake up, increment ChildCnt of parentLid
	monitor.goroutineMutex.Lock()
	Lid := monitor.GetLid(goid)
	if Lid != nil {
		Lid.IncrChildCnt()
	}
	monitor.goroutineMutex.Unlock()

	monitor.yieldAsWaiting(goid)
}

func (monitor *SyncPrimitiveMonitor) AfterMainGoroutineCreation(goid Goid) {
	goState := monitor_defs.NewGoroutineState(goid, nil)

	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.MainGoroutineBegin, false, nil, 0)
	monitor.mainGoroutineID = goid
	monitor.goroutineTable[goid] = goState
	monitor.goroutines = append(monitor.goroutines, goid)
	monitor.numRunningGoroutines++

	monitor.goroutineMutex.Unlock()

	monitor.scheduler.OnNewGoroutineBegin(goid, true)
}

func (monitor *SyncPrimitiveMonitor) BeforeTRun(goid Goid) {
	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.GoroutineCreation, true, nil, 0)
	monitor.numCreatingGoroutines++
	monitor.goroutineMutex.Unlock()

	monitor.LogSyncEvent(goid)
}

func (monitor *SyncPrimitiveMonitor) AfterTRun(goid Goid, parentGoid Goid) {
	monitor.goroutineMutex.Lock()

	parentState := monitor.goroutineTable[parentGoid]
	goState := monitor_defs.NewGoroutineState(goid, parentState.Lid)

	monitor.setCurrEvent(goid, monitor_defs.NewGoroutineBegin, false, nil, 0)

	monitor.goroutineTable[goid] = goState
	monitor.numRunningGoroutines++
	monitor.goroutines = append(monitor.goroutines, goid)
	monitor.numCreatingGoroutines--

	monitor.goroutineMutex.Unlock()

	monitor.scheduler.OnNewGoroutineBegin(goid, false)

	monitor.yieldAsWaiting(goid)
}

func (monitor *SyncPrimitiveMonitor) BeforeGoroutineEnd(goid Goid) {
	monitor.goroutineMutex.Lock()
	defer monitor.goroutineMutex.Unlock()

	monitor.setCurrEvent(goid, monitor_defs.GoroutineEnd, true, goid, 0)

	monitor.removeGoroutine(goid)

	numAlive := len(monitor.goroutines)
	if numAlive > 0 {
		go monitor.schedule()
	}
}

func (monitor *SyncPrimitiveMonitor) BeforeMainGoroutineEnd(goid Goid) {
	if monitor.shouldRecord {
		defer monitor.recorder.ToFile()
	}

	if monitor.shouldTrace {
		defer monitor.tracer.ToFile()
	}

	defer monitor.scheduler.OnTermination()

	monitor.goroutineMutex.Lock()

	monitor.setCurrEvent(goid, monitor_defs.MainGoroutineEnd, true, goid, 0)

	if len(monitor.goroutines) == 1 {
		monitor.goroutineMutex.Unlock()
		return
	}

	// There are other goroutines still running, wait for them to finish or timeout
	monitor.removeGoroutine(goid)
	terminateCh := make(chan struct{}, 1)
	monitor.goroutineMutex.Unlock()

	go monitor.schedule()

	go func() {
		for {
			monitor.goroutineMutex.Lock()
			if len(monitor.goroutines) == 0 {
				monitor.goroutineMutex.Unlock()
				break
			}
			monitor.goroutineMutex.Unlock()
			time.Sleep(100 * time.Millisecond)
		}
		terminateCh <- struct{}{}
	}()

	select {
	case <-terminateCh:
		if debug {
			fmt.Printf("Main Goroutine exits as all gorotuines have finished \n")
		}

	case <-time.After(config.TESTING_TIME_OUT_SECONDS * time.Second):
		if debug {
			fmt.Printf("Main Goroutine exits timeout = %ds is reached \n", config.TESTING_TIME_OUT_SECONDS)
		}
	}
}

func (monitor *SyncPrimitiveMonitor) GetGoroutineState(goid runtime_types.Goid) *monitor_defs.GoroutineState {
	// always called with goroutineMutex held
	return monitor.goroutineTable[goid]
}

func (monitor *SyncPrimitiveMonitor) GetNumRunningGoroutines() int {
	// always called with goroutineMutex held
	return monitor.numRunningGoroutines
}

func (monitor *SyncPrimitiveMonitor) GetNumAliveGoroutines() int {
	// always called with goroutineMutex held
	return len(monitor.goroutines)
}

func (monitor *SyncPrimitiveMonitor) GetNumCreatingGoroutines() int {
	// always called with goroutineMutex held
	return monitor.numCreatingGoroutines
}

func (monitor *SyncPrimitiveMonitor) RecordSeed(seed int64) {
	if monitor.shouldRecord {
		monitor.recorder.RecordSeed(seed)
	}
}

func (monitor *SyncPrimitiveMonitor) schedule() {
	defer func() {
		if r := recover(); r != nil {
			if monitor.shouldRecord {
				monitor.recorder.ToFile()
			}
			monitor.scheduler.OnTermination()
			panic(r)
		}
	}()
	monitor.goroutineMutex.Lock()
	defer monitor.goroutineMutex.Unlock()

	if len(monitor.goroutines) == 0 {
		return
	}

	if debug {
		monitor.PrintGoroutineStatus()
		// a, b, c := monitor.GetExecutableChoices()
		// monitor.PrintSchedulingChoices(a, b, c)
	}
	res := monitor.scheduler.MakeNextSchedulingDecision()

	// scheduler decides to not do anything
	if res == nil {
		return
	} else if res.IsSingleGoroutine {
		if debug {
			fmt.Printf("Scheduler chooses Goroutine [%d] to execute \n", res.SingleGoroutine.Event.GoId)
		}

		schedOp := res.SingleGoroutine
		if monitor.shouldRecord {
			var caseIdx int
			var hasCase bool
			if schedOp.WakeupMsg != nil && schedOp.WakeupMsg.SelectedCase != nil {
				caseIdx = schedOp.WakeupMsg.SelectedCase.CaseIdx
				hasCase = true
			}
			monitor.recorder.Record(monitor.GetLid(schedOp.Event.GoId), caseIdx, hasCase)
		}
		monitor.wakeupGoroutine(schedOp.Event.GoId, schedOp.WakeupMsg)
	} else {
		pair := res.GoroutinePair

		if debug {
			fmt.Printf("Scheduler chooses Goroutine Pair [%v] to execute \n", res.GoroutinePair)
		}

		senderLid := monitor.GetLid(pair.Sender.GoId)
		recverLid := monitor.GetLid(pair.Receiver.GoId)
		if monitor.shouldRecord {
			var sIdx, rIdx int
			var sHas, rHas bool
			if pair.WakeupMsgSender != nil && pair.WakeupMsgSender.SelectedCase != nil {
				sIdx = pair.WakeupMsgSender.SelectedCase.CaseIdx
				sHas = true
			}
			if pair.WakeupMsgReceiver != nil && pair.WakeupMsgReceiver.SelectedCase != nil {
				rIdx = pair.WakeupMsgReceiver.SelectedCase.CaseIdx
				rHas = true
			}
			monitor.recorder.RecordPair(senderLid, recverLid, sIdx, sHas, rIdx, rHas)
		}
		monitor.wakeupGoroutinePair(pair)
	}
}

// ================================= Mutexes =================================
func (monitor *SyncPrimitiveMonitor) AfterMutexCreation(goid Goid, m *sync.Mutex, id uint64) {
	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.MutexLockCreation, false, m, id)
	monitor.goroutineMutex.Unlock()
	monitor.yieldAsWaiting(goid)
}

func (monitor *SyncPrimitiveMonitor) BeforeMutexLock(goid Goid, m *sync.Mutex, id uint64) {
	// if debug {
	// 	fmt.Printf("Goroutine [%d] BeforeMutexLock. Stack:\n%s\n", goid, string(runtime_debug.Stack()))
	// }

	monitor.mutexMutex.Lock()
	if _, ok := monitor.mutexStates[m]; !ok {
		monitor.mutexStates[m] = monitor_defs.NewMutexState()
	}
	monitor.mutexMutex.Unlock()

	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.MutexLock, true, m, id)
	monitor.goroutineMutex.Unlock()
	monitor.yieldAsBlocking(goid, monitor_defs.MUTEX, m)
}

func (monitor *SyncPrimitiveMonitor) AfterMutexLock(goid Goid, m *sync.Mutex, id uint64) {
	monitor.LogSyncEvent(goid)

	monitor.mutexMutex.Lock()
	if _, ok := monitor.mutexStates[m]; !ok {
		monitor.mutexStates[m] = monitor_defs.NewMutexState()
	}
	monitor.mutexStates[m].IsAcquired = true
	monitor.mutexMutex.Unlock()

	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.MutexLock, false, m, id)
	monitor.goroutineMutex.Unlock()
	monitor.yieldAsWaiting(goid)
}

func (monitor *SyncPrimitiveMonitor) BeforeMutexUnlock(goid Goid, m *sync.Mutex, id uint64) {
	monitor.LogSyncEvent(goid)

	monitor.mutexMutex.Lock()
	if _, ok := monitor.mutexStates[m]; !ok {
		monitor.mutexStates[m] = monitor_defs.NewMutexState()
	}
	monitor.mutexMutex.Unlock()

	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.MutexUnlock, true, m, id)
	monitor.goroutineMutex.Unlock()
	monitor.yieldAsWaiting(goid)
}

func (monitor *SyncPrimitiveMonitor) AfterMutexUnlock(goid Goid, m *sync.Mutex, id uint64) {
	monitor.mutexMutex.Lock()
	if _, ok := monitor.mutexStates[m]; !ok {
		monitor.mutexStates[m] = monitor_defs.NewMutexState()
	}
	monitor.mutexStates[m].IsAcquired = false
	monitor.mutexMutex.Unlock()

	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.MutexUnlock, false, m, id)
	monitor.goroutineMutex.Unlock()
	monitor.yieldAsWaiting(goid)
}

func (monitor *SyncPrimitiveMonitor) BeforeMutexTryLock(goid Goid, m *sync.Mutex, id uint64) {
	monitor.mutexMutex.Lock()
	if _, ok := monitor.mutexStates[m]; !ok {
		monitor.mutexStates[m] = monitor_defs.NewMutexState()
	}
	monitor.mutexMutex.Unlock()

	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.MutexLock, true, m, id)
	monitor.goroutineMutex.Unlock()
	monitor.yieldAsWaiting(goid)
}

func (monitor *SyncPrimitiveMonitor) AfterMutexTryLock(goid Goid, m *sync.Mutex, id uint64) {
	monitor.LogSyncEvent(goid)

	monitor.mutexMutex.Lock()
	if _, ok := monitor.mutexStates[m]; !ok {
		monitor.mutexStates[m] = monitor_defs.NewMutexState()
	}
	monitor.mutexStates[m].IsAcquired = true
	monitor.mutexMutex.Unlock()

	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.MutexLock, false, m, id)
	monitor.goroutineMutex.Unlock()
	monitor.yieldAsWaiting(goid)
}

// ================================= RWMutexes =================================
func (monitor *SyncPrimitiveMonitor) AfterRWMutexCreation(goid Goid, m *sync.RWMutex, id uint64) {
	monitor.rwmutexMutex.Lock()
	if _, ok := monitor.rwmutexStates[m]; !ok {
		monitor.rwmutexStates[m] = monitor_defs.NewRWMutexState()
	}
	monitor.rwmutexMutex.Unlock()

	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.RWMutexLockCreation, false, m, id)
	monitor.goroutineMutex.Unlock()
	monitor.yieldAsWaiting(goid)
}

func (monitor *SyncPrimitiveMonitor) BeforeRWMutexTryLock(goid Goid, m *sync.RWMutex, id uint64) {
	monitor.rwmutexMutex.Lock()
	if _, ok := monitor.rwmutexStates[m]; !ok {
		monitor.rwmutexStates[m] = monitor_defs.NewRWMutexState()
	}
	monitor.rwmutexMutex.Unlock()

	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.RWMutexLock, true, m, id)
	monitor.goroutineMutex.Unlock()
	monitor.yieldAsWaiting(goid)
}

func (monitor *SyncPrimitiveMonitor) AfterRWMutexTryLock(goid Goid, m *sync.RWMutex, id uint64) {
	monitor.rwmutexMutex.Lock()
	if _, ok := monitor.rwmutexStates[m]; !ok {
		monitor.rwmutexStates[m] = monitor_defs.NewRWMutexState()
	}
	monitor.rwmutexStates[m].WriterOwnedGoId = goid
	monitor.rwmutexStates[m].WriterFlagGoId = 0
	monitor.rwmutexMutex.Unlock()

	monitor.LogSyncEvent(goid)
	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.RWMutexLock, false, m, id)
	monitor.goroutineMutex.Unlock()
	monitor.yieldAsWaiting(goid)
}

func (monitor *SyncPrimitiveMonitor) BeforeRWMutexTryRLock(goid Goid, m *sync.RWMutex, id uint64) {
	monitor.rwmutexMutex.Lock()
	if _, ok := monitor.rwmutexStates[m]; !ok {
		monitor.rwmutexStates[m] = monitor_defs.NewRWMutexState()
	}
	monitor.rwmutexMutex.Unlock()

	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.RWMutexRLock, true, m, id)
	monitor.goroutineMutex.Unlock()
	monitor.yieldAsWaiting(goid)
}

func (monitor *SyncPrimitiveMonitor) AfterRWMutexTryRLock(goid Goid, m *sync.RWMutex, id uint64) {
	monitor.LogSyncEvent(goid)

	monitor.rwmutexMutex.Lock()
	if _, ok := monitor.rwmutexStates[m]; !ok {
		monitor.rwmutexStates[m] = monitor_defs.NewRWMutexState()
	}
	monitor.rwmutexStates[m].Readers += 1
	monitor.rwmutexMutex.Unlock()

	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.RWMutexRLock, false, m, id)
	monitor.goroutineMutex.Unlock()
	monitor.yieldAsWaiting(goid)
}

func (monitor *SyncPrimitiveMonitor) BeforeRWMutexLock(goid Goid, m *sync.RWMutex, id uint64) {
	monitor.rwmutexMutex.Lock()
	if _, ok := monitor.rwmutexStates[m]; !ok {
		monitor.rwmutexStates[m] = monitor_defs.NewRWMutexState()
	}
	monitor.rwmutexMutex.Unlock()

	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.RWMutexLockSetFlag, true, m, id)
	monitor.goroutineMutex.Unlock()
	monitor.yieldAsBlocking(goid, monitor_defs.RWMUTEX_WRITE, m)

	// chosen to set flag, but not acquire the lock yet
	monitor.rwmutexMutex.Lock()
	monitor.rwmutexStates[m].WriterFlagGoId = goid
	monitor.rwmutexMutex.Unlock()

	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.RWMutexLockSetFlag, false, m, id)
	monitor.goroutineMutex.Unlock()

	monitor.yieldAsBlocking(goid, monitor_defs.RWMUTEX_WRITE_WAITING_FOR_READERS, m)
}

func (monitor *SyncPrimitiveMonitor) AfterRWMutexLock(goid Goid, m *sync.RWMutex, id uint64) {
	monitor.LogSyncEvent(goid)

	monitor.rwmutexMutex.Lock()
	if _, ok := monitor.rwmutexStates[m]; !ok {
		monitor.rwmutexStates[m] = monitor_defs.NewRWMutexState()
	}
	monitor.rwmutexStates[m].WriterOwnedGoId = goid
	monitor.rwmutexStates[m].WriterFlagGoId = 0
	monitor.rwmutexMutex.Unlock()

	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.RWMutexLock, false, m, id)
	monitor.goroutineMutex.Unlock()
	monitor.yieldAsWaiting(goid)
}

func (monitor *SyncPrimitiveMonitor) BeforeRWMutexUnlock(goid Goid, m *sync.RWMutex, id uint64) {
	monitor.rwmutexMutex.Lock()
	if _, ok := monitor.rwmutexStates[m]; !ok {
		monitor.rwmutexStates[m] = monitor_defs.NewRWMutexState()
	}
	monitor.rwmutexMutex.Unlock()

	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.RWMutexUnlock, true, m, id)
	monitor.goroutineMutex.Unlock()
	monitor.yieldAsWaiting(goid)
}

func (monitor *SyncPrimitiveMonitor) AfterRWMutexUnlock(goid Goid, m *sync.RWMutex, id uint64) {
	monitor.LogSyncEvent(goid)

	monitor.rwmutexMutex.Lock()
	if _, ok := monitor.rwmutexStates[m]; !ok {
		monitor.rwmutexStates[m] = monitor_defs.NewRWMutexState()
	}
	monitor.rwmutexStates[m].WriterOwnedGoId = 0
	monitor.rwmutexMutex.Unlock()

	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.RWMutexUnlock, false, m, id)
	monitor.goroutineMutex.Unlock()
	monitor.yieldAsWaiting(goid)
}

func (monitor *SyncPrimitiveMonitor) BeforeRWMutexRLock(goid Goid, m *sync.RWMutex, id uint64) {
	monitor.rwmutexMutex.Lock()
	if _, ok := monitor.rwmutexStates[m]; !ok {
		monitor.rwmutexStates[m] = monitor_defs.NewRWMutexState()
	}
	monitor.rwmutexMutex.Unlock()

	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.RWMutexRLock, true, m, id)
	monitor.goroutineMutex.Unlock()
	monitor.yieldAsBlocking(goid, monitor_defs.RWMUTEX_READ, m)
}

func (monitor *SyncPrimitiveMonitor) AfterRWMutexRLock(goid Goid, m *sync.RWMutex, id uint64) {
	monitor.LogSyncEvent(goid)

	monitor.rwmutexMutex.Lock()
	if _, ok := monitor.rwmutexStates[m]; !ok {
		monitor.rwmutexStates[m] = monitor_defs.NewRWMutexState()
	}
	monitor.rwmutexStates[m].Readers += 1
	monitor.rwmutexMutex.Unlock()

	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.RWMutexRLock, false, m, id)
	monitor.goroutineMutex.Unlock()
	monitor.yieldAsWaiting(goid)
}

func (monitor *SyncPrimitiveMonitor) BeforeRWMutexRUnlock(goid Goid, m *sync.RWMutex, id uint64) {
	monitor.rwmutexMutex.Lock()
	if _, ok := monitor.rwmutexStates[m]; !ok {
		monitor.rwmutexStates[m] = monitor_defs.NewRWMutexState()
	}
	monitor.rwmutexMutex.Unlock()

	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.RWMutexRUnlock, true, m, id)
	monitor.goroutineMutex.Unlock()
	monitor.yieldAsWaiting(goid)
}

func (monitor *SyncPrimitiveMonitor) AfterRWMutexRUnlock(goid Goid, m *sync.RWMutex, id uint64) {
	monitor.LogSyncEvent(goid)

	monitor.rwmutexMutex.Lock()
	if _, ok := monitor.rwmutexStates[m]; !ok {
		monitor.rwmutexStates[m] = monitor_defs.NewRWMutexState()
	}
	monitor.rwmutexStates[m].Readers -= 1
	monitor.rwmutexMutex.Unlock()

	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.RWMutexRUnlock, false, m, id)
	monitor.goroutineMutex.Unlock()
	monitor.yieldAsWaiting(goid)
}

// ================================= Channels =================================

func (monitor *SyncPrimitiveMonitor) BeforeChannelSend(goid Goid, ch any, isSelect, isTimer bool, id uint64) {
	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.ChannelSend, true, ch, id)
	monitor.goroutineMutex.Unlock()

	monitor.channelsMutex.Lock()
	cap := utils.GetChanCap(ch)
	chPtr := utils.GetPtrOf(ch)

	var BType monitor_defs.BlockingType
	var blocker any

	// send to nil blocks forever
	if chPtr == 0 {
		BType = monitor_defs.NIL_CHANNEL_SEND_RECV
		blocker = nil
	} else if cap == 0 {
		BType = monitor_defs.CHANNEL_SYNC_SEND
		blocker = ch
	} else {
		BType = monitor_defs.CHANNEL_ASYNC_SEND
		blocker = ch
	}

	monitor.channelsMutex.Unlock()

	// For select channel events, we don't yeild,
	// because the enabledness is checked in isSelectEnabled.
	if !isSelect {
		monitor.yieldAsBlocking(goid, BType, blocker)
	}

	monitor.goroutineMutex.Lock()
	goState := monitor.goroutineTable[goid]
	lidString := goState.Lid.StringName
	monitor.goroutineMutex.Unlock()

	monitor.channelsMutex.Lock()
	chState := monitor.channels[chPtr]
	if chState != nil && chState.Capacity > 0 {
		sendInfo := monitor_defs.NewChannelSendInfo(goid, id, lidString)
		chState.Sends = append(chState.Sends, sendInfo)
	}
	monitor.channelsMutex.Unlock()
}

func (monitor *SyncPrimitiveMonitor) AfterChannelSend(goid Goid, ch any, id uint64) {
	monitor.LogSyncEvent(goid)

	chPtr := utils.GetPtrOf(ch)
	sendIdx := uint32(0)

	monitor.channelsMutex.Lock()
	chState := monitor.channels[chPtr]
	if chState != nil && chState.Capacity > 0 {
		sendIdx = chState.SendIdx
		chState.SendIdx += 1
	}
	monitor.channelsMutex.Unlock()

	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.ChannelSend, false, ch, id, sendIdx)
	monitor.goroutineMutex.Unlock()
	monitor.yieldAsWaiting(goid)
}

func (monitor *SyncPrimitiveMonitor) BeforeChannelReceive(goid Goid, ch any, isSelect, isTimer bool, id uint64) {
	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.ChannelReceive, true, ch, id)
	monitor.goroutineMutex.Unlock()

	monitor.channelsMutex.Lock()
	cap := utils.GetChanCap(ch)
	chPtr := utils.GetPtrOf(ch)

	var BType monitor_defs.BlockingType
	var blocker any

	if chPtr == 0 {
		BType = monitor_defs.NIL_CHANNEL_SEND_RECV
		blocker = nil
	} else if isTimer {
		BType = monitor_defs.TIMER_RECEIVE
		blocker = ch
	} else if cap == 0 {
		BType = monitor_defs.CHANNEL_SYNC_RECEIVE
		blocker = ch
	} else {
		BType = monitor_defs.CHANNEL_ASYNC_RECEIVE
		blocker = ch
	}

	monitor.channelsMutex.Unlock()

	if !isSelect {
		monitor.yieldAsBlocking(goid, BType, blocker)
	}

	if isTimer {
		monitor.timerMutex.Lock()
		tState := monitor.timerCToTimerState[chPtr]
		if tState == nil {
			panic("BeforeChannelReceive: timerState is nil")
		}
		if tState.TType == monitor_defs.TIME_TIMER {
			tState.Avaiable = false
		}
		monitor.timerMutex.Unlock()
	}
}

func (monitor *SyncPrimitiveMonitor) AfterChannelReceive(goid Goid, ch any, isSelect, isTimer bool, id uint64) {
	monitor.LogSyncEvent(goid)
	var sendInfo *monitor_defs.ChannelSendInfo

	monitor.channelsMutex.Lock()
	chPtr := utils.GetPtrOf(ch)
	chState := monitor.channels[chPtr]
	if chState != nil && chState.Capacity > 0 {
		if len(chState.Sends) > 0 {
			sendInfo = chState.Sends[0]
			chState.Sends = chState.Sends[1:]
		} else if chState.IsClosed {
			// if channel is closed, then recv can also be unblocked,
			// but there is no matching send, so using the close op as matching send
			goid := chState.ClosedGoId
			lid := chState.ClosedLid
			opId := chState.ClosedOpId
			sendInfo = monitor_defs.NewChannelSendInfo(goid, opId, lid)
		} else {
			panic("AfterChannelReceive: len(chState.Sends) = 0, cannot find sendInfo")
		}
	}
	monitor.channelsMutex.Unlock()

	monitor.goroutineMutex.Lock()
	if chState != nil && chState.Capacity > 0 {
		monitor.setCurrEvent(goid, monitor_defs.ChannelReceive, false, ch, id, sendInfo)
	} else {
		// MetaInfo of sync recv will be assigned inside the scheduler
		// Because we cannot guarantee the execution order of AfterChannelReceive and
		// AfterChannelSend
		monitor.setCurrEvent(goid, monitor_defs.ChannelReceive, false, ch, id)
	}
	monitor.goroutineMutex.Unlock()

	monitor.yieldAsWaiting(goid)
}

func (monitor *SyncPrimitiveMonitor) BeforeChannelClose(goid Goid, ch any, id uint64) {
	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.ChannelClose, true, ch, id)
	lidString := monitor.GetLid(goid).StringName
	monitor.goroutineMutex.Unlock()

	monitor.yieldAsWaiting(goid)

	chPtr := utils.GetPtrOf(ch)

	monitor.channelsMutex.Lock()
	chState := monitor.channels[chPtr]
	if chState != nil && chState.Capacity > 0 {
		sendInfo := monitor_defs.NewChannelSendInfo(goid, id, lidString)
		chState.Sends = append(chState.Sends, sendInfo)
	}
	monitor.channelsMutex.Unlock()
}

func (monitor *SyncPrimitiveMonitor) AfterChannelClose(goid Goid, ch any, id uint64) {
	monitor.LogSyncEvent(goid)

	monitor.goroutineMutex.Lock()
	lid := monitor.goroutineTable[goid].Lid.StringName
	monitor.setCurrEvent(goid, monitor_defs.ChannelClose, false, ch, id)
	monitor.goroutineMutex.Unlock()

	monitor.channelsMutex.Lock()
	chPtr := utils.GetPtrOf(ch)
	chState := monitor.channels[chPtr]
	if chState != nil {
		chState.IsClosed = true
		chState.ClosedLid = lid
		chState.ClosedOpId = id
		chState.ClosedGoId = goid
	}
	monitor.channelsMutex.Unlock()
	monitor.yieldAsWaiting(goid)
}

func (monitor *SyncPrimitiveMonitor) AfterChannelCreation(goid Goid, ch any, cap int, id uint64) {
	// This is an "After" hook, so we don't set the NextOp.
	// It's immediately followed by a yield, after which we will reset.
	monitor.LogSyncEvent(goid)

	monitor.channelsMutex.Lock()

	if debug {
		fmt.Printf("Goroutine [%d] creating channel with capacity %d \n", goid, cap)
	}
	chPtr := utils.GetPtrOf(ch)

	monitor.channels[chPtr] = monitor_defs.NewChannelState(cap, id)

	monitor.channelsMutex.Unlock()

	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.ChannelCreation, false, ch, id)
	monitor.goroutineMutex.Unlock()

	monitor.yieldAsWaiting(goid)
}

// ================================= CondVar =================================

func (monitor *SyncPrimitiveMonitor) AfterCondVarCreation(goid Goid, c *sync.Cond, id uint64) {
	monitor.condVarMutex.Lock()
	monitor.condVarWaitSet[c] = make(map[string]uint64)
	monitor.condVarMutex.Unlock()

	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.CondVarCreation, false, c, id)
	monitor.goroutineMutex.Unlock()

	monitor.yieldAsWaiting(goid)
}

func (monitor *SyncPrimitiveMonitor) BeforeCondVarWait(goid Goid, c *sync.Cond, id uint64) {
	monitor.LogSyncEvent(goid)

	monitor.goroutineMutex.Lock()
	goState := monitor.goroutineTable[goid]
	lid := goState.Lid.StringName
	monitor.setCurrEvent(goid, monitor_defs.CondVarWait, true, c, id)
	monitor.goroutineMutex.Unlock()

	monitor.condVarMutex.Lock()
	waitMap, ok := monitor.condVarWaitSet[c]
	if !ok {
		monitor.condVarWaitSet[c] = make(map[string]uint64)
		waitMap = monitor.condVarWaitSet[c]
	}
	waitMap[lid] = id
	monitor.condVarMutex.Unlock()

	BeforeInterfaceUnlock(c.L, 0)
	c.L.Unlock() // release the c.L, then block
	AfterInterfaceUnlock(c.L, 0)

	monitor.yieldAsBlocking(goid, monitor_defs.COND_VAR_WAIT, c)

	BeforeInterfaceLock(c.L, 0)
	c.L.Lock() // Re-acquire lock upon waking up and set the mutexState / rwmutexState of c.L
	AfterInterfaceLock(c.L, 0)
}

func (monitor *SyncPrimitiveMonitor) AfterCondVarWait(goid Goid, c *sync.Cond, id uint64) {
	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.CondVarWait, false, c, id)
	monitor.goroutineMutex.Unlock()

	monitor.yieldAsWaiting(goid)
}

func (monitor *SyncPrimitiveMonitor) BeforeCondVarSignal(goid Goid, c *sync.Cond, id uint64) {
	monitor.condVarMutex.Lock()
	waitMap, ok := monitor.condVarWaitSet[c]
	if !ok {
		monitor.condVarWaitSet[c] = make(map[string]uint64)
		waitMap = monitor.condVarWaitSet[c]
	}
	monitor.condVarMutex.Unlock()

	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.CondVarSignal, true, c, id, waitMap)
	monitor.goroutineMutex.Unlock()

	msg := monitor.yieldAsWaiting(goid)

	if msg == nil || msg.CondVarWaitInfos == nil || len(msg.CondVarWaitInfos) != 1 {
		panic("BeforeCondVarSignal: msg is nil, cannot find Lid")
	}

	waitInfo := msg.CondVarWaitInfos[0]

	fmt.Printf("======= received waitInfo = %v \n", waitInfo)

	if !waitInfo.IsNilCondVarWaitInfo() {
		monitor.condVarMutex.Lock()
		delete(waitMap, waitInfo.Lid)
		monitor.condVarMutex.Unlock()
	}
}

func (monitor *SyncPrimitiveMonitor) AfterCondVarSignal(goid Goid, c *sync.Cond, id uint64) {
	monitor.LogSyncEvent(goid)

	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.CondVarSignal, false, c, id)
	monitor.goroutineMutex.Unlock()

	monitor.yieldAsWaiting(goid)
}

func (monitor *SyncPrimitiveMonitor) BeforeCondVarBroadcast(goid Goid, c *sync.Cond, id uint64) {
	monitor.condVarMutex.Lock()
	waitMap, ok := monitor.condVarWaitSet[c]
	if !ok {
		monitor.condVarWaitSet[c] = make(map[string]uint64)
		waitMap = monitor.condVarWaitSet[c]
	}
	monitor.condVarMutex.Unlock()

	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.CondVarBroadcast, true, c, id, waitMap)
	monitor.goroutineMutex.Unlock()

	monitor.yieldAsWaiting(goid)
}

func (monitor *SyncPrimitiveMonitor) AfterCondVarBroadcast(goid Goid, c *sync.Cond, id uint64) {
	monitor.LogSyncEvent(goid)
	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.CondVarBroadcast, false, c, id)
	monitor.goroutineMutex.Unlock()

	monitor.condVarMutex.Lock()
	waitMap := monitor.condVarWaitSet[c]
	clear(waitMap)
	monitor.condVarMutex.Unlock()

	monitor.yieldAsWaiting(goid)
}

// ================================= Select =================================

func (monitor *SyncPrimitiveMonitor) buildCases(sends []any, recvs []any, isSendTimer, isRecvTimer []bool, sendIdxs []int,
	recvIdxs []int, hasDefault bool) []*monitor_defs.SelectChoice {
	monitor.channelsMutex.Lock()

	allCases := make([]*monitor_defs.SelectChoice, 0)

	if hasDefault {
		defaultCase := &monitor_defs.SelectChoice{
			BlockingType: monitor_defs.NOTBLOCKING,
			Ch:           nil,
			CaseIdx:      -1,
		}
		allCases = append(allCases, defaultCase)
	}

	for i := 0; i < len(sends); i++ {
		var BType monitor_defs.BlockingType
		ch := sends[i]

		if utils.GetChanCap(ch) == 0 {
			BType = monitor_defs.CHANNEL_SYNC_SEND
		} else {
			BType = monitor_defs.CHANNEL_ASYNC_SEND
		}

		currCase := &monitor_defs.SelectChoice{
			BlockingType: BType,
			Ch:           sends[i],
			CaseIdx:      sendIdxs[i],
		}
		allCases = append(allCases, currCase)
	}

	for i := 0; i < len(recvs); i++ {
		var BType monitor_defs.BlockingType
		ch := recvs[i]
		isTimer := isRecvTimer[i]

		if isTimer {
			BType = monitor_defs.TIMER_RECEIVE
		} else if utils.GetChanCap(ch) == 0 {
			BType = monitor_defs.CHANNEL_SYNC_RECEIVE
		} else {
			BType = monitor_defs.CHANNEL_ASYNC_RECEIVE
		}

		currCase := &monitor_defs.SelectChoice{
			BlockingType: BType,
			Ch:           recvs[i],
			CaseIdx:      recvIdxs[i],
		}
		allCases = append(allCases, currCase)
	}

	monitor.channelsMutex.Unlock()
	return allCases
}

func (monitor *SyncPrimitiveMonitor) BeforeSelect(goid Goid, sends []any, recvs []any, isSendTimer, isRecvTimer []bool,
	sendIdxs []int, recvIdxs []int, hasDefault bool, opId uint64) int {
	// build cases except for default
	allCases := monitor.buildCases(sends, recvs, isSendTimer, isRecvTimer, sendIdxs, recvIdxs, hasDefault)
	selectBlocker := &monitor_defs.SelectBlocker{
		Cases:      allCases,
		HasDefault: hasDefault,
	}

	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.Select, true, selectBlocker, opId)
	monitor.goroutineMutex.Unlock()

	msg := monitor.yieldAsBlocking(goid, monitor_defs.SELECT, selectBlocker)

	if msg == nil {
		panic("WakeupMessage is nil in BeforeSelect")
	}

	if !hasDefault && msg.SelectedCase.CaseIdx == -1 {
		fmt.Printf("Goroutine [%d] selects case -1 when it has no default \n", goid)
		panic("Select default error")
	}

	if debug {
		fmt.Printf("Goroutine [%d] selects case %d \n", goid, msg.SelectedCase.CaseIdx)
	}

	monitor.LogSyncEvent(goid)

	return msg.SelectedCase.CaseIdx
}

func (monitor *SyncPrimitiveMonitor) AfterSelect(goid Goid, sends []any, recvs []any, isSendTimer, isRecvTimer []bool,
	sendIdxs []int, recvIdxs []int, hasDefault bool, selectedCase int, id uint64) {
	caseLen := len(sends) + len(recvs)
	if hasDefault {
		caseLen += 1
	}

	metaInfo := make(map[string]int, 3)
	if hasDefault {
		metaInfo["hasDefault"] = 1
	} else {
		metaInfo["hasDefault"] = 0
	}
	metaInfo["totalCases"] = caseLen
	metaInfo["selectedCase"] = selectedCase
	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.Select, false, nil, id, metaInfo)
	e := monitor.goroutineTable[goid].CurrEvent
	monitor.goroutineMutex.Unlock()

	// Do not yield, but update feedback manually
	monitor.scheduler.OnSyncEvent(e, nil)
}

// ================================= Context =================================

func (monitor *SyncPrimitiveMonitor) AfterContextCreation(goid Goid, ctx context.Context) {
	doneCh := ctx.Done()
	chPtr := utils.GetPtrOf(doneCh)

	monitor.LogSyncEvent(goid)

	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.ChannelCreation, false, doneCh, 0)
	monitor.goroutineMutex.Unlock()

	monitor.channelsMutex.Lock()
	if doneCh != nil {
		chState := monitor_defs.NewChannelState(0, 1)
		monitor.channels[chPtr] = chState
	}
	monitor.channelsMutex.Unlock()

	monitor.yieldAsWaiting(goid)
}

func (monitor *SyncPrimitiveMonitor) BeforeContextCancel(goid Goid, ctx context.Context) {
	// This is a non-blocking operation, but we can still record it.
	// Since it's non-blocking, there's no corresponding After hook where we can reliably reset.
	// For now, we won't set NextOp for non-blocking operations.
	// No yield needed.
	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.ChannelClose, true, ctx, 0)
	monitor.goroutineMutex.Unlock()

	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	fmt.Printf("%s\n", buf[:n])

	monitor.yieldAsWaiting(goid)
}

func (monitor *SyncPrimitiveMonitor) AfterContextCancel(goid Goid, ctx context.Context) {
	// This is a non-blocking operation, but we can still record it.

	monitor.goroutineMutex.Lock()
	monitor.channelsMutex.Lock()

	monitor.setCurrEvent(goid, monitor_defs.ChannelClose, false, nil, 0)

	doneCh := ctx.Done()
	chPtr := utils.GetPtrOf(doneCh)

	if doneCh != nil {
		chState := monitor.channels[chPtr]
		if chState != nil {
			chState.IsClosed = true
		}
	}

	monitor.channelsMutex.Unlock()
	monitor.goroutineMutex.Unlock()
	monitor.yieldAsWaiting(goid)
}

// ================================= WaitGroup =================================
func (monitor *SyncPrimitiveMonitor) AfterWaitGroupCreation(goid Goid, wg *sync.WaitGroup, id uint64) {
	monitor.waitgroupMutex.Lock()
	if _, ok := monitor.waitgroupStates[wg]; !ok {
		monitor.waitgroupStates[wg] = monitor_defs.NewWaitGroupState()
	}
	monitor.waitgroupMutex.Unlock()

	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.WaitGroupCreation, false, wg, id)
	monitor.goroutineMutex.Unlock()
	monitor.yieldAsWaiting(goid)
}

func (monitor *SyncPrimitiveMonitor) BeforeWaitGroupAdd(goid Goid, wg *sync.WaitGroup, delta int, id uint64) {
	monitor.waitgroupMutex.Lock()
	if _, ok := monitor.waitgroupStates[wg]; !ok {
		monitor.waitgroupStates[wg] = monitor_defs.NewWaitGroupState()
	}
	monitor.waitgroupMutex.Unlock()

	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.WaitGroupAdd, true, wg, id)
	monitor.goroutineMutex.Unlock()
	monitor.yieldAsWaiting(goid)
}

func (monitor *SyncPrimitiveMonitor) AfterWaitGroupAdd(goid Goid, wg *sync.WaitGroup, delta int, id uint64) {
	monitor.LogSyncEvent(goid)

	monitor.waitgroupMutex.Lock()
	wgState, ok := monitor.waitgroupStates[wg]
	if !ok {
		monitor.waitgroupStates[wg] = monitor_defs.NewWaitGroupState()
		wgState = monitor.waitgroupStates[wg]
	}
	wgState.Count += 1
	monitor.waitgroupMutex.Unlock()

	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.WaitGroupAdd, false, wg, id)
	monitor.goroutineMutex.Unlock()
	monitor.yieldAsWaiting(goid)
}

func (monitor *SyncPrimitiveMonitor) BeforeWaitGroupDone(goid Goid, wg *sync.WaitGroup, id uint64) {
	monitor.waitgroupMutex.Lock()
	if _, ok := monitor.waitgroupStates[wg]; !ok {
		monitor.waitgroupStates[wg] = monitor_defs.NewWaitGroupState()
	}
	monitor.waitgroupMutex.Unlock()

	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.WaitGroupDone, true, wg, id)
	monitor.goroutineMutex.Unlock()
	monitor.yieldAsWaiting(goid)
}

func (monitor *SyncPrimitiveMonitor) AfterWaitGroupDone(goid Goid, wg *sync.WaitGroup, id uint64) {
	monitor.LogSyncEvent(goid)

	monitor.waitgroupMutex.Lock()
	wgState, ok := monitor.waitgroupStates[wg]
	if !ok {
		monitor.waitgroupStates[wg] = monitor_defs.NewWaitGroupState()
		wgState = monitor.waitgroupStates[wg]
	}
	wgState.Count -= 1
	monitor.waitgroupMutex.Unlock()

	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.WaitGroupDone, false, wg, id)
	monitor.goroutineMutex.Unlock()
	monitor.yieldAsWaiting(goid)
}

func (monitor *SyncPrimitiveMonitor) BeforeWaitGroupWait(goid Goid, wg *sync.WaitGroup, id uint64) {
	monitor.waitgroupMutex.Lock()
	if _, ok := monitor.waitgroupStates[wg]; !ok {
		monitor.waitgroupStates[wg] = monitor_defs.NewWaitGroupState()
	}
	monitor.waitgroupMutex.Unlock()

	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.WaitGroupWait, true, wg, id)
	monitor.goroutineMutex.Unlock()
	monitor.yieldAsBlocking(goid, monitor_defs.WAITGROUP, wg)
}

func (monitor *SyncPrimitiveMonitor) AfterWaitGroupWait(goid Goid, wg *sync.WaitGroup, id uint64) {
	monitor.LogSyncEvent(goid)

	monitor.waitgroupMutex.Lock()
	if _, ok := monitor.waitgroupStates[wg]; !ok {
		monitor.waitgroupStates[wg] = monitor_defs.NewWaitGroupState()
	}
	monitor.waitgroupMutex.Unlock()

	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.WaitGroupWait, false, wg, id)
	monitor.goroutineMutex.Unlock()
	monitor.yieldAsWaiting(goid)
}

func (monitor *SyncPrimitiveMonitor) AfterWaitGroupGoRun(goid Goid, wg *sync.WaitGroup, id uint64) {
	monitor.waitgroupMutex.Lock()
	wgState, ok := monitor.waitgroupStates[wg]
	if !ok {
		monitor.waitgroupStates[wg] = monitor_defs.NewWaitGroupState()
		wgState = monitor.waitgroupStates[wg]
	}
	wgState.Count += 1
	monitor.waitgroupMutex.Unlock()

	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.WaitGroupAdd, false, wg, id)
	monitor.goroutineMutex.Unlock()
	monitor.yieldAsWaiting(goid)
}

func (monitor *SyncPrimitiveMonitor) BeforeWaitGroupGoEnd(goid Goid, wg *sync.WaitGroup, id uint64) {
	monitor.waitgroupMutex.Lock()
	wgState, ok := monitor.waitgroupStates[wg]
	if !ok {
		monitor.waitgroupStates[wg] = monitor_defs.NewWaitGroupState()
		wgState = monitor.waitgroupStates[wg]
	}
	wgState.Count -= 1
	monitor.waitgroupMutex.Unlock()

	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.WaitGroupDone, true, wg, id)
	monitor.goroutineMutex.Unlock()
	monitor.yieldAsWaiting(goid)
}

// ================================= Assertion =================================
func (monitor *SyncPrimitiveMonitor) BeforeAssertion(goid Goid) {
	monitor.goroutineMutex.Lock()
	monitor.setCurrEvent(goid, monitor_defs.Assertion, true, struct{}{}, 0)
	monitor.goroutineMutex.Unlock()
	monitor.yieldAsWaiting(goid)
}

// ================================= Loop =================================
// This is to handle the following program:
// It prevents the following loop from entering default endlessly
// without hitting scheduling point
/* for {
 	select {
	case <-ch:
	default:
   }                     */
func (monitor *SyncPrimitiveMonitor) OnEachLoopIteration(goid Goid) {
	monitor.yieldAsWaiting(goid)
}

// ================================= Time =================================
func (monitor *SyncPrimitiveMonitor) AfterTimerCreation(goid runtime_types.Goid, t any) {
	monitor.timerMutex.Lock()
	tPtr := utils.GetPtrOf(t)
	var chPtr uintptr

	var tState *monitor_defs.TimerState

	switch typed := t.(type) {
	case sync_interface.Timer:
		tState = monitor_defs.NewTimerState()
		chPtr = utils.GetPtrOf(typed.GetC())

	case sync_interface.Ticker:
		tState = monitor_defs.NewTickerState()
		chPtr = utils.GetPtrOf(typed.GetC())

	default:
		panic("Unknown timer type")
	}

	monitor.timerMap[tPtr] = tState
	monitor.timerCToTimerState[chPtr] = tState
	monitor.timerMutex.Unlock()
}

func (monitor *SyncPrimitiveMonitor) BeforeTimerReset(goid runtime_types.Goid, t any) {
	monitor.yieldAsWaiting(goid)
}

func (monitor *SyncPrimitiveMonitor) AfterTimerReset(goid runtime_types.Goid, t any) {
	monitor.timerMutex.Lock()
	tPtr := utils.GetPtrOf(t)
	tState := monitor.timerMap[tPtr]

	if tState == nil {
		panic("AfterTimerReset: tState is nil")
	}

	tState.Avaiable = true
	monitor.timerMutex.Unlock()

	monitor.yieldAsWaiting(goid)
}

func (monitor *SyncPrimitiveMonitor) BeforeTimerStop(goid runtime_types.Goid, t any) {
	monitor.yieldAsWaiting(goid)
}

func (monitor *SyncPrimitiveMonitor) AfterTimerStop(goid runtime_types.Goid, t any) {
	monitor.timerMutex.Lock()
	tPtr := utils.GetPtrOf(t)
	tState := monitor.timerMap[tPtr]

	if tState == nil {
		panic("AfterTimerStop: tState is nil")
	}

	tState.Avaiable = false
	monitor.timerMutex.Unlock()

	monitor.yieldAsWaiting(goid)
}

// ================================= Recorder =================================

func (monitor *SyncPrimitiveMonitor) GetLid(goid Goid) *replay.LogicalId {
	// always called with goroutineMutex held
	if state, ok := monitor.goroutineTable[goid]; ok {
		return state.Lid
	}
	return nil
}

func (monitor *SyncPrimitiveMonitor) LogSyncEvent(goid Goid) {
	if !monitor.shouldTrace {
		return
	}
	monitor.goroutineMutex.Lock()
	event := monitor.goroutineTable[goid].CurrEvent
	monitor.goroutineMutex.Unlock()
	monitor.tracer.TraceEvent(event)
}
