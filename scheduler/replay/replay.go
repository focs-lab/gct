package replay_scheduler

import (
	"fmt"
	"strings"
	"strconv"

	runtime_types "github.com/focs-lab/gct/runtime_types"
	monitor_defs "github.com/focs-lab/gct/runtime_types/monitor_defs"
	replay "github.com/focs-lab/gct/runtime_types/replay"
	scheduler_defs "github.com/focs-lab/gct/runtime_types/scheduler_defs"
)

type Goid = runtime_types.Goid
type SingleGoroutineOption = monitor_defs.SingleGoroutineOption
type GoroutineRendezvousPairOption = monitor_defs.GoroutineRendezvousPairOption

// ReplayScheduler makes scheduling decisions based on a pre-recorded trace.
type ReplayScheduler struct {
	recorder   replay.Recorder
	traceIndex int
	monitor    monitor_defs.SyncMonitorSched
}

// NewReplayScheduler creates a new scheduler that replays the given trace file.
func NewReplayScheduler(traceLoc string) *ReplayScheduler {
	recorder := replay.NewBaseRecorder()
	recorder.BuildFromTrace(traceLoc)

	scheduler := &ReplayScheduler{
		recorder:   recorder,
		traceIndex: 0,
	}

	return scheduler
}

func (scheduler *ReplayScheduler) MakeNextSchedulingDecision() *scheduler_defs.ScheduleResult {
	if scheduler.monitor.GetNumRunningGoroutines() > 0 || scheduler.monitor.GetNumCreatingGoroutines() > 0 {
		return nil
	}

	waiting, enabled, pairs := scheduler.monitor.GetExecutableChoices()

	if scheduler.traceIndex >= scheduler.recorder.GetTotalNumEvents() {
		numAlive := scheduler.monitor.GetNumAliveGoroutines()

		if len(waiting) == 0 && len(enabled) == 0 && len(pairs) == 0 {
			if numAlive == 0 {
				return nil // Normal termination, no more goroutines to schedule
			} else {
				panic("Deadlock: No waiting goroutine or blocked goroutine pair to schedule.")
			}
		}

		panic("Replay trace finished, but there are still schedulable goroutines.")
	}

	nextLine := scheduler.recorder.GetEvent(scheduler.traceIndex)
	if strings.HasPrefix(nextLine, "Seed: ") {
		scheduler.traceIndex++
		return scheduler.MakeNextSchedulingDecision()
	}

	scheduler.traceIndex++

	traceParts := strings.Split(nextLine, ",")

	if len(traceParts) == 1 {
		targetLidStr, caseIdx, hasCase := parseLidAndCase(traceParts[0])
		return scheduler.findSingleGoroutine(waiting, enabled, targetLidStr, caseIdx, hasCase)
	} else if len(traceParts) == 2 {
		senderLidStr, senderCaseIdx, senderHasCase := parseLidAndCase(traceParts[0])
		receiverLidStr, receiverCaseIdx, receiverHasCase := parseLidAndCase(traceParts[1])
		return scheduler.findGoroutinePair(pairs, senderLidStr, receiverLidStr, senderCaseIdx, receiverCaseIdx, senderHasCase, receiverHasCase)
	}

	panic(fmt.Sprintf("Invalid trace line: %s", nextLine))
}

func parseLidAndCase(raw string) (string, int, bool) {
	parts := strings.Split(raw, ":")
	if len(parts) == 1 {
		return parts[0], 0, false // no case index
	}
	caseIdx, err := strconv.Atoi(parts[1])
	if err != nil {
		panic(fmt.Sprintf("Invalid case index in trace: %s", raw))
	}
	return parts[0], caseIdx, true
}

func (scheduler *ReplayScheduler) findSingleGoroutine(waiting []*SingleGoroutineOption, enabled []*SingleGoroutineOption, targetLidStr string, caseIdx int, hasCase bool) *scheduler_defs.ScheduleResult {
	// 1. Check waiting goroutines
	for _, option := range waiting {
		if lid := scheduler.monitor.GetLid(option.Event.GoId); lid != nil && lid.String() == targetLidStr {
			if hasCase {
				continue // WAITING goroutines don't have a select case
			}
			return &scheduler_defs.ScheduleResult{
				IsSingleGoroutine: true,
				SingleGoroutine: option,
			}
		}
	}

	// 2. Check other enabled goroutines
	for _, tuple := range enabled {
		if lid := scheduler.monitor.GetLid(tuple.Event.GoId); lid != nil && lid.String() == targetLidStr {
			if hasCase {
				// This is a select. We need to match the case index.
				if tuple.WakeupMsg != nil && tuple.WakeupMsg.SelectedCase != nil && tuple.WakeupMsg.SelectedCase.CaseIdx == caseIdx {
					return &scheduler_defs.ScheduleResult{
						IsSingleGoroutine: true,
						SingleGoroutine:   tuple,
					}
				}
			} else {
				// Not a select.
				if tuple.WakeupMsg == nil || tuple.WakeupMsg.SelectedCase == nil {
					return &scheduler_defs.ScheduleResult{
						IsSingleGoroutine: true,
						SingleGoroutine:   tuple,
					}
				}
			}
		}
	}

	panic(fmt.Sprintf("Could not find schedulable single goroutine with Lid %s", targetLidStr))
}

func (scheduler *ReplayScheduler) findGoroutinePair(rendezvousPairs []*monitor_defs.GoroutineRendezvousPairOption, senderLidStr, receiverLidStr string, senderCaseIdx, receiverCaseIdx int, senderHasCase, receiverHasCase bool) *scheduler_defs.ScheduleResult {
	for _, pair := range rendezvousPairs {
		if senderLid := scheduler.monitor.GetLid(pair.Sender.GoId); senderLid != nil && senderLid.String() == senderLidStr {
			if receiverLid := scheduler.monitor.GetLid(pair.Receiver.GoId); receiverLid != nil && receiverLid.String() == receiverLidStr {
				var senderMatch bool
				if senderHasCase {
					senderMatch = pair.WakeupMsgSender != nil && pair.WakeupMsgSender.SelectedCase != nil && pair.WakeupMsgSender.SelectedCase.CaseIdx == senderCaseIdx
				} else {
					senderMatch = pair.WakeupMsgSender == nil || pair.WakeupMsgSender.SelectedCase == nil
				}

				var receiverMatch bool
				if receiverHasCase {
					receiverMatch = pair.WakeupMsgReceiver != nil && pair.WakeupMsgReceiver.SelectedCase != nil && pair.WakeupMsgReceiver.SelectedCase.CaseIdx == receiverCaseIdx
				} else {
					receiverMatch = pair.WakeupMsgReceiver == nil || pair.WakeupMsgReceiver.SelectedCase == nil
				}

				if senderMatch && receiverMatch {
					return &scheduler_defs.ScheduleResult{
						IsSingleGoroutine: false,
						GoroutinePair:     pair,
					}
				}
			}
		}
	}
	panic(fmt.Sprintf("Could not find schedulable goroutine pair with sender Lid %s and receiver Lid %s", senderLidStr, receiverLidStr))
}

func (scheduler *ReplayScheduler) OnNewGoroutineBegin(goid runtime_types.Goid, isMain bool) {

}

func (scheduler *ReplayScheduler) SetMonitor(monitor monitor_defs.SyncMonitorSched) {
	scheduler.monitor = monitor
}

func (scheduler *ReplayScheduler) OnTermination() {

}

func (scheduler *ReplayScheduler) OnSyncEvent(e *monitor_defs.SyncEvent, result *scheduler_defs.ScheduleResult) {

}