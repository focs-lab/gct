package pct

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"sort"
	"sync"
	"time"

	config "github.com/focs-lab/gct/config"
	runtime_types "github.com/focs-lab/gct/runtime_types"
	monitor_defs "github.com/focs-lab/gct/runtime_types/monitor_defs"
	scheduler_defs "github.com/focs-lab/gct/runtime_types/scheduler_defs"
)

const debug = config.DEBUG

type Goid = runtime_types.Goid
type SingleGoroutineOption = runtime_types.Tuple[Goid, *monitor_defs.WakeupMessage]


type PCTScheduler struct {
	rnd                  *rand.Rand
	monitor              monitor_defs.SyncMonitorSched
	seed                 int64
	eventCounter         int
	maxNumEvents         int
	priorities           []Goid
	priorityChangePoints []int
	nextChangePointIdx   int
	stateMutex           *sync.Mutex
}

func NewPCTScheduler(depth, maxNumEvents int) *PCTScheduler {
	seed := time.Now().UnixNano()

	if depth <= 0 {
		fmt.Printf("Found depth = %d for PCT scheduler \n", depth)
		panic("Non-positive depth for PCT scheduler")
	}

	if maxNumEvents <= 0 {
		fmt.Printf("Found maxNumEvents = %d for PCT scheduler \n", maxNumEvents)
		panic("Non-positive maxNumEvents for PCT scheduler")
	}

	scheduler := &PCTScheduler{
		rnd:                  rand.New(rand.NewSource(seed)),
		seed:                 seed,
		eventCounter:         0,
		maxNumEvents:         maxNumEvents,
		priorities:           make([]Goid, 0),
		priorityChangePoints: make([]int, depth),
		nextChangePointIdx:   0,
		stateMutex:           &sync.Mutex{},
	}

	scheduler.initPriorityChangePoints()

	return scheduler
}

func (scheduler *PCTScheduler) initPriorityChangePoints() {
	max := scheduler.maxNumEvents
	s := scheduler.priorityChangePoints

	if len(s) > max+1 {
		panic("slice length cannot exceed max + 1 (not enough unique numbers)")
	}

	used := make(map[int]struct{})

	for i := 0; i < len(s); {
		n := scheduler.rnd.Intn(max + 1)

		if _, exists := used[n]; !exists {
			used[n] = struct{}{}
			s[i] = n
			i++
		}
	}

	sort.Ints(s)

	if debug {
		fmt.Printf("PCT scheduler chooses priority change points s = %v \n", s)
	}
}

func (scheduler *PCTScheduler) checkAndChangePriorities(chosenGoid Goid) {
	// always called with stateMutex held
	nextIdx := scheduler.nextChangePointIdx
	if nextIdx < len(scheduler.priorityChangePoints) {
		if scheduler.eventCounter == scheduler.priorityChangePoints[nextIdx] {
			for i, id := range scheduler.priorities {
				if id == chosenGoid {
					copy(scheduler.priorities[1:i+1], scheduler.priorities[0:i])
					scheduler.priorities[0] = chosenGoid
					break
				}
			}
			scheduler.nextChangePointIdx++
		}
	}
}

func (scheduler *PCTScheduler) MakeNextSchedulingDecision() *scheduler_defs.ScheduleResult {
	// Guaranteed len(goroutines) > 0

	// only schedule when no goroutine is running or being created
	if scheduler.monitor.GetNumRunningGoroutines() > 0 || scheduler.monitor.GetNumCreatingGoroutines() > 0 {
		return nil
	}

	scheduler.stateMutex.Lock()
	defer scheduler.stateMutex.Unlock()

	waiting, enabled, rendezvous := scheduler.monitor.GetExecutableChoices()

	if len(waiting)+len(enabled)+len(rendezvous) == 0 {
		panic("Deadlock: No waiting goroutine or blocked goroutine pair to schedule.")
	}

	// Create fast lookups for runnable goroutines
	waitingMap := make(map[Goid]*monitor_defs.SingleGoroutineOption, len(waiting))
	for _, option := range waiting {
		waitingMap[option.Event.GoId] = option
	}

	// The same goroutine G can be enabled with different message.
	// G blocks on select of <-ch1, <-ch2, and both channels are closed.
	enabledMap := make(map[Goid][]*monitor_defs.SingleGoroutineOption)
	for _, option := range enabled {
		enabledMap[option.Event.GoId] = append(enabledMap[option.Event.GoId], option)
	}

	rendezvousMap := make(map[Goid][]*monitor_defs.GoroutineRendezvousPairOption)
	for _, pair := range rendezvous {
		sendGoId := pair.Sender.GoId
		recvGoId := pair.Receiver.GoId
		rendezvousMap[sendGoId] = append(rendezvousMap[sendGoId], pair)
		rendezvousMap[recvGoId] = append(rendezvousMap[recvGoId], pair)
	}

	var chosenGoid Goid
	var result *scheduler_defs.ScheduleResult

	// Find the highest priority runnable goroutine
	// The end of the slice has the highest priority.
	for i := len(scheduler.priorities) - 1; i >= 0; i-- {
		goid := scheduler.priorities[i]

		if option, ok := waitingMap[goid]; ok {
			chosenGoid = goid
			result = &scheduler_defs.ScheduleResult{
				IsSingleGoroutine: true,
				SingleGoroutine:   option,
			}
			break
		}

		if options, ok := enabledMap[goid]; ok {
			chosenGoid = goid
			chosenOption := options[scheduler.rnd.Intn(len(options))]
			result = &scheduler_defs.ScheduleResult{
				IsSingleGoroutine: true,
				SingleGoroutine:   chosenOption,
			}
			break
		}

		if pairs, ok := rendezvousMap[goid]; ok {
			chosenGoid = goid
			chosenPair := pairs[scheduler.rnd.Intn(len(pairs))]
			result = &scheduler_defs.ScheduleResult{
				IsSingleGoroutine: false,
				GoroutinePair:     chosenPair,
			}
			break
		}
	}

	if result == nil {
		panic("PCT scheduler could not find a runnable goroutine among priority list")
	}

	scheduler.checkAndChangePriorities(chosenGoid)
	scheduler.eventCounter++

	if result.IsSingleGoroutine {
		scheduler.updateState(result.SingleGoroutine.Event, result)
	} else {
		scheduler.updateState(result.GoroutinePair.Sender, result)
		scheduler.updateState(result.GoroutinePair.Receiver, result)
	}

	return result
}

func (scheduler *PCTScheduler) OnNewGoroutineBegin(goid runtime_types.Goid, isMain bool) {
	scheduler.stateMutex.Lock()
	defer scheduler.stateMutex.Unlock()

	// Randomly pick a position and insert the goroutine id into priorities array.
	insertPos := 0
	if len(scheduler.priorities) > 0 {
		insertPos = scheduler.rnd.Intn(len(scheduler.priorities) + 1)
	}

	scheduler.priorities = append(scheduler.priorities, 0)
	copy(scheduler.priorities[insertPos+1:], scheduler.priorities[insertPos:])
	scheduler.priorities[insertPos] = goid
}

func (scheduler *PCTScheduler) updateState(e *monitor_defs.SyncEvent, result *scheduler_defs.ScheduleResult) {
	switch e.EventType {
	case monitor_defs.GoroutineEnd, monitor_defs.MainGoroutineEnd:
		if e.IsBefore {
			goid := e.GoId

			scheduler.stateMutex.Lock()
			defer scheduler.stateMutex.Unlock()

			for i, id := range scheduler.priorities {
				if id == goid {
					scheduler.priorities = append(scheduler.priorities[:i], scheduler.priorities[i+1:]...)
					break
				}
			}
		}
	}
}

func (scheduler *PCTScheduler) SetMonitor(monitor monitor_defs.SyncMonitorSched) {
	scheduler.monitor = monitor
	scheduler.monitor.RecordSeed(scheduler.seed)
}

func (scheduler *PCTScheduler) OnTermination() {
	// Record
	scheduler.stateMutex.Lock()
	defer scheduler.stateMutex.Unlock()

	envFilePath := os.Getenv(config.ENV_CONFIG_FILE_NAME)
	if envFilePath == "" {
		fmt.Printf("[WARN] %s environment variable is not set. Cannot record PCT calibration data.\n", config.ENV_CONFIG_FILE_NAME)
		return
	}

	env := make(map[string]int)

	existingContent, err := os.ReadFile(envFilePath)
	if err == nil && len(existingContent) > 0 {
		unmarshalErr := json.Unmarshal(existingContent, &env)
		if unmarshalErr != nil {
			fmt.Printf("[WARN] Could not parse existing env file %s: %v. Starting with empty env for PCT calibration.\n", envFilePath, unmarshalErr)
			env = make(map[string]int)
		}
	} else if err != nil && !os.IsNotExist(err) {
		fmt.Printf("[WARN] Could not read existing env file %s: %v. Starting with empty env for PCT calibration.\n", envFilePath, err)
		env = make(map[string]int)
	}

	currentMaxEvents := env[config.MAX_PCT_EVENTS]

	if scheduler.eventCounter > currentMaxEvents {
		env[config.MAX_PCT_EVENTS] = scheduler.eventCounter
		if debug {
			fmt.Printf("[INFO] Updated %s to %d in %s\n", config.MAX_PCT_EVENTS, scheduler.eventCounter, envFilePath)
		}
	}

	toWrite, marshalErr := json.MarshalIndent(env, "", " ")
	if marshalErr != nil {
		fmt.Printf("[ERROR] Could not marshal env data for %s: %v\n", envFilePath, marshalErr)
		return
	}

	writeErr := os.WriteFile(envFilePath, toWrite, 0644)
	if writeErr != nil {
		fmt.Printf("[ERROR] Could not write env file %s: %v\n", envFilePath, writeErr)
	}
}

func (scheduler *PCTScheduler) OnSyncEvent(e *monitor_defs.SyncEvent, result *scheduler_defs.ScheduleResult) {

}
