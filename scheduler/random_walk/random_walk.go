package scheduler

import (
	"math/rand"
	"time"

	md "github.com/focs-lab/gct/runtime_types/monitor_defs"
	sd "github.com/focs-lab/gct/runtime_types/scheduler_defs"
	runtime_types "github.com/focs-lab/gct/runtime_types"
)

// This scheduler will randomly choose a goroutine to execute until termination
// it then chooses the next goroutine and so on, until all goroutines terminate
type RandomWalkScheduler struct {
	rng     *rand.Rand
	monitor md.SyncMonitorSched
	seed    int64
}

func NewRandomWalkScheduler() *RandomWalkScheduler {
	seed := time.Now().UnixNano()
	scheduler := &RandomWalkScheduler{
		rng:  rand.New(rand.NewSource(seed)),
		seed: seed,
	}

	return scheduler
}

func (scheduler *RandomWalkScheduler) MakeNextSchedulingDecision() *sd.ScheduleResult {
	// Guaranteed len(goroutines) > 0

	// only schedule when no goroutine is running or being created
	if scheduler.monitor.GetNumRunningGoroutines() > 0 || scheduler.monitor.GetNumCreatingGoroutines() > 0 {
		return nil
	}

	waiting, enabled, rendezvous := scheduler.monitor.GetExecutableChoices()

	totalNumChoices := len(waiting) + len(enabled) + len(rendezvous)

	if totalNumChoices == 0 {
		panic("Deadlock: No waiting goroutine or blocked goroutine pair to schedule.")
	}

	randIdx := scheduler.rng.Intn(totalNumChoices)
	var result *sd.ScheduleResult

	if randIdx < len(waiting) {
		// wake up a single waiting goroutine
		chosenOption := waiting[randIdx]
		result = &sd.ScheduleResult{
			IsSingleGoroutine: true,
			SingleGoroutine: chosenOption,
		}

	} else if randIdx-len(waiting) < len(enabled) {
		// wake up a single gorotuine that blocks on select
		// the selected case is one of
		// (1) enabled async channel
		// (2) closed async/sync channels
		chosenOption := enabled[randIdx-len(waiting)]


		result = &sd.ScheduleResult{
			IsSingleGoroutine: true,
			SingleGoroutine: chosenOption,
		}
	} else {
		// wake up a pair of blocked goroutines
		chosenPair := rendezvous[randIdx-len(waiting)-len(enabled)]
		result = &sd.ScheduleResult{
			IsSingleGoroutine: false,
			GoroutinePair:     chosenPair,
		}
	}

	return result
}

func (scheduler *RandomWalkScheduler) OnNewGoroutineBegin(goid runtime_types.Goid, isMain bool) {
	
}

func (scheduler *RandomWalkScheduler) SetMonitor(monitor md.SyncMonitorSched) {
	scheduler.monitor = monitor
	scheduler.monitor.RecordSeed(scheduler.seed)
}

func (scheduler *RandomWalkScheduler) OnTermination() {

}

func (scheduler *RandomWalkScheduler) OnSyncEvent(e *md.SyncEvent, result *sd.ScheduleResult) {

}
