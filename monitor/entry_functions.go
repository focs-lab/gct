package monitor

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/focs-lab/gct/config"
	"github.com/focs-lab/gct/monitor/utils"
	runtime_types "github.com/focs-lab/gct/runtime_types"
	monitor_defs "github.com/focs-lab/gct/runtime_types/monitor_defs"
	scheduler_defs "github.com/focs-lab/gct/runtime_types/scheduler_defs"
	pct "github.com/focs-lab/gct/scheduler/pct"
	"github.com/timandy/routine"

	// pos "github.com/focs-lab/gct/scheduler/pos"
	rand_walk "github.com/focs-lab/gct/scheduler/random_walk"
	replay "github.com/focs-lab/gct/scheduler/replay"
)

const debug = config.DEBUG

// const debug = false

var (
	globalChannelStates map[uintptr]*monitor_defs.ChannelState = make(map[uintptr]*monitor_defs.ChannelState)
	globalChannelMutex  sync.Mutex
)

var (
	initOnce            sync.Once
	syncMonitor         monitor_defs.SyncMonitorEntry
	hasInitialized      atomic.Bool
)

func lazyInitSyncMonitor() {
	initOnce.Do(func() {
		schedulerName := os.Getenv(config.SCHEDULER_NAME)

		recordOption := os.Getenv(config.RECORD_FLAG) == "true"
		traceOption := os.Getenv(config.TRACE_FLAG) == "true"

		scheduler := initScheduler(schedulerName)

		_monitor := NewSyncPrimitiveMonitor(scheduler, recordOption, traceOption)

		globalChannelMutex.Lock()
		for ptr, chState := range globalChannelStates {
			_monitor.channels[ptr] = chState
		}
		globalChannelMutex.Unlock()

		syncMonitor = _monitor
		scheduler.SetMonitor(_monitor)

		hasInitialized.Store(true)
	})
}

func initScheduler(schedulerName string) scheduler_defs.Scheduler {
	traceLoc := os.Getenv(config.TRACE_LOC)

	var scheduler scheduler_defs.Scheduler

	switch schedulerName {
	case "random_walk":
		fmt.Println("Initialized Scheduler: random_walk")
		scheduler = rand_walk.NewRandomWalkScheduler()

	case "replay":
		if traceLoc == "" {
			panic("TraceLoc is not set")
		}
		fmt.Printf("Initialized Scheduler: replay, TraceLoc: %s\n", traceLoc)
		scheduler = replay.NewReplayScheduler(traceLoc)

	case "pct":
		num, err := strconv.Atoi(os.Getenv(config.MAX_PCT_EVENTS))
		if err != nil {
			fmt.Println("The maximal events of PCT is not set. Please set it to be the default value 500.")
			num = config.DEFAULT_MAX_PCT_EVENTS
		}

		depth, err := strconv.Atoi(os.Getenv(config.PCT_BUG_DEPTH))
		if err != nil {
			fmt.Println("The bug depth of PCT is not set. Please set it to be the default value 2.")
			depth = config.DEFAULT_PCT_BUG_DEPTH
		}

		fmt.Printf("Initialized Scheduler: pct, MaxEvents: %d, BugDepth: %d\n", num, depth)

		scheduler = pct.NewPCTScheduler(depth, num)

	default:
		panic("Invalid scheduler name " + schedulerName)
	}

	return scheduler
}

func GetGoid() runtime_types.Goid {
	return runtime_types.Goid(routine.Goid())
}

// ================================= Mutex =================================
func AfterMutexCreation(m *sync.Mutex, id uint64) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered AfterMutexCreation (id: %d)\n", goid, id)
	}
	syncMonitor.AfterMutexCreation(goid, m, id)
	if debug {
		fmt.Printf("Goroutine [%d] Exited AfterMutexCreation (id: %d)\n", goid, id)
	}
}

func BeforeMutexLock(m *sync.Mutex, id uint64) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered BeforeMutexLock (id: %d)\n", goid, id)
	}

	syncMonitor.BeforeMutexLock(goid, m, id)

	if debug {
		fmt.Printf("Goroutine [%d] Exited BeforeMutexLock (id: %d)\n", goid, id)
	}
}

func AfterMutexLock(m *sync.Mutex, id uint64) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered AfterMutexLock (id: %d)\n", goid, id)
	}
	syncMonitor.AfterMutexLock(goid, m, id)
	if debug {
		fmt.Printf("Goroutine [%d] Exited AfterMutexLock (id: %d)\n", goid, id)
	}
}

func BeforeMutexUnlock(m *sync.Mutex, id uint64) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered BeforeMutexUnlock (id: %d)\n", goid, id)
	}
	syncMonitor.BeforeMutexUnlock(goid, m, id)
	if debug {
		fmt.Printf("Goroutine [%d] Exited BeforeMutexUnlock (id: %d)\n", goid, id)
	}
}

func AfterMutexUnlock(m *sync.Mutex, id uint64) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered AfterMutexUnlock (id: %d)\n", goid, id)
	}
	syncMonitor.AfterMutexUnlock(goid, m, id)
	if debug {
		fmt.Printf("Goroutine [%d] Exited AfterMutexUnlock (id: %d)\n", goid, id)
	}
}

func BeforeMutexTryLock(m *sync.Mutex, id uint64) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered BeforeMutexTryLock (id: %d)\n", goid, id)
	}
	syncMonitor.BeforeMutexTryLock(goid, m, id)
	if debug {
		fmt.Printf("Goroutine [%d] Exited BeforeMutexTryLock (id: %d)\n", goid, id)
	}
}

func AfterMutexTryLock(m *sync.Mutex, id uint64) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered AfterMutexTryLock (id: %d)\n", goid, id)
	}
	syncMonitor.AfterMutexTryLock(goid, m, id)
	if debug {
		fmt.Printf("Goroutine [%d] Exited AfterMutexTryLock (id: %d)\n", goid, id)
	}
}

// ================================= RWMutex =================================
func AfterRWMutexCreation(m *sync.RWMutex, id uint64) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered AfterRWMutexCreation (id: %d)\n", goid, id)
	}
	syncMonitor.AfterRWMutexCreation(goid, m, id)
	if debug {
		fmt.Printf("Goroutine [%d] Exited AfterRWMutexCreation \n", goid)
	}
}

func BeforeRWMutexLock(m *sync.RWMutex, id uint64) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered BeforeRWMutexLock (id: %d)\n", goid, id)
	}
	syncMonitor.BeforeRWMutexLock(goid, m, id)
	if debug {
		fmt.Printf("Goroutine [%d] Exited BeforeRWMutexLock (id: %d)\n", goid, id)
	}
}

func AfterRWMutexLock(m *sync.RWMutex, id uint64) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered AfterRWMutexLock (id: %d)\n", goid, id)
	}
	syncMonitor.AfterRWMutexLock(goid, m, id)
	if debug {
		fmt.Printf("Goroutine [%d] Exited AfterRWMutexLock (id: %d)\n", goid, id)
	}
}

func BeforeRWMutexUnlock(m *sync.RWMutex, id uint64) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered BeforeRWMutexUnlock (id: %d)\n", goid, id)
	}
	syncMonitor.BeforeRWMutexUnlock(goid, m, id)
	if debug {
		fmt.Printf("Goroutine [%d] Exited BeforeRWMutexUnlock (id: %d)\n", goid, id)
	}
}

func AfterRWMutexUnlock(m *sync.RWMutex, id uint64) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered AfterRWMutexUnlock (id: %d)\n", goid, id)
	}
	syncMonitor.AfterRWMutexUnlock(goid, m, id)
	if debug {
		fmt.Printf("Goroutine [%d] Exited AfterRWMutexUnlock (id: %d)\n", goid, id)
	}
}

func BeforeRWMutexRLock(m *sync.RWMutex, id uint64) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered BeforeRWMutexRLock (id: %d)\n", goid, id)
	}
	syncMonitor.BeforeRWMutexRLock(goid, m, id)
	if debug {
		fmt.Printf("Goroutine [%d] Exited BeforeRWMutexRLock (id: %d)\n", goid, id)
	}
}

func AfterRWMutexRLock(m *sync.RWMutex, id uint64) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered AfterRWMutexRLock (id: %d)\n", goid, id)
	}
	syncMonitor.AfterRWMutexRLock(goid, m, id)
	if debug {
		fmt.Printf("Goroutine [%d] Exited AfterRWMutexRLock (id: %d)\n", goid, id)
	}
}

func BeforeRWMutexTryRLock(m *sync.RWMutex, id uint64) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered BeforeRWMutexTryRLock (id: %d)\n", goid, id)
	}
	syncMonitor.BeforeRWMutexTryRLock(goid, m, id)
	if debug {
		fmt.Printf("Goroutine [%d] Exited BeforeRWMutexTryRLock (id: %d)\n", goid, id)
	}
}

func AfterRWMutexTryRLock(m *sync.RWMutex, id uint64) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered AfterRWMutexTryRLock (id: %d)\n", goid, id)
	}
	syncMonitor.AfterRWMutexTryRLock(goid, m, id)
	if debug {
		fmt.Printf("Goroutine [%d] Exited AfterRWMutexTryRLock (id: %d)\n", goid, id)
	}
}

func BeforeRWMutexTryLock(m *sync.RWMutex, id uint64) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered BeforeRWMutexTryLock (id: %d)\n", goid, id)
	}
	syncMonitor.BeforeRWMutexTryLock(goid, m, id)
	if debug {
		fmt.Printf("Goroutine [%d] Exited BeforeRWMutexTryLock (id: %d)\n", goid, id)
	}
}

func AfterRWMutexTryLock(m *sync.RWMutex, id uint64) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered AfterRWMutexTryLock (id: %d)\n", goid, id)
	}
	syncMonitor.AfterRWMutexTryLock(goid, m, id)
	if debug {
		fmt.Printf("Goroutine [%d] Exited AfterRWMutexTryLock (id: %d)\n", goid, id)
	}
}

func BeforeRWMutexRUnlock(m *sync.RWMutex, id uint64) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered BeforeRWMutexRUnlock (id: %d)\n", goid, id)
	}
	syncMonitor.BeforeRWMutexRUnlock(goid, m, id)
	if debug {
		fmt.Printf("Goroutine [%d] Exited BeforeRWMutexRUnlock (id: %d)\n", goid, id)
	}
}

func AfterRWMutexRUnlock(m *sync.RWMutex, id uint64) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered AfterRWMutexRUnlock (id: %d)\n", goid, id)
	}
	syncMonitor.AfterRWMutexRUnlock(goid, m, id)
	if debug {
		fmt.Printf("Goroutine [%d] Exited AfterRWMutexRUnlock (id: %d)\n", goid, id)
	}
}

// ================================= Interface Mutex =================================
func BeforeInterfaceLock(m sync.Locker, id uint64) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered BeforeInterfaceLock (id: %d)\n", goid, id)
	}

	switch typedM := m.(type) {
	case *sync.Mutex:
		syncMonitor.BeforeMutexLock(goid, typedM, id)

	case *sync.RWMutex:
		syncMonitor.BeforeRWMutexLock(goid, typedM, id)

	default:
		if rw, ok := utils.ExtractRWMutexFromLocker(m); ok {
			syncMonitor.BeforeRWMutexRLock(goid, rw, id)
		} else {
			typeName := reflect.TypeOf(m).String()
			panic(fmt.Sprintf("Unknown mutex type %s \n", typeName))
		}
	}

	if debug {
		fmt.Printf("Goroutine [%d] Exited BeforeInterfaceLock (id: %d)\n", goid, id)
	}
}

func AfterInterfaceLock(m sync.Locker, id uint64) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered AfterInterfaceLock (id: %d)\n", goid, id)
	}

	switch typedM := m.(type) {
	case *sync.Mutex:
		syncMonitor.AfterMutexLock(goid, typedM, id)

	case *sync.RWMutex:
		syncMonitor.AfterRWMutexLock(goid, typedM, id)

	default:
		if rw, ok := utils.ExtractRWMutexFromLocker(m); ok {
			syncMonitor.AfterRWMutexRLock(goid, rw, id)
		} else {
			typeName := reflect.TypeOf(m).String()
			panic(fmt.Sprintf("Unknown mutex type %s \n", typeName))
		}
	}

	if debug {
		fmt.Printf("Goroutine [%d] Exited AfterInterfaceLock (id: %d)\n", goid, id)
	}
}

func BeforeInterfaceUnlock(m sync.Locker, id uint64) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered BeforeInterfaceUnlock (id: %d)\n", goid, id)
	}

	switch typedM := m.(type) {
	case *sync.Mutex:
		syncMonitor.BeforeMutexUnlock(goid, typedM, id)

	case *sync.RWMutex:
		syncMonitor.BeforeRWMutexUnlock(goid, typedM, id)

	default:
		if rw, ok := utils.ExtractRWMutexFromLocker(m); ok {
			syncMonitor.BeforeRWMutexRUnlock(goid, rw, id)
		} else {
			typeName := reflect.TypeOf(m).String()
			panic(fmt.Sprintf("Unknown mutex type %s \n", typeName))
		}
	}

	if debug {
		fmt.Printf("Goroutine [%d] Exited BeforeInterfaceUnlock (id: %d)\n", goid, id)
	}
}

func AfterInterfaceUnlock(m sync.Locker, id uint64) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered AfterInterfaceUnlock (id: %d)\n", goid, id)
	}

	switch typedM := m.(type) {
	case *sync.Mutex:
		syncMonitor.AfterMutexUnlock(goid, typedM, id)

	case *sync.RWMutex:
		syncMonitor.AfterRWMutexUnlock(goid, typedM, id)

	default:
		if rw, ok := utils.ExtractRWMutexFromLocker(m); ok {
			syncMonitor.AfterRWMutexRUnlock(goid, rw, id)
		} else {
			typeName := reflect.TypeOf(m).String()
			panic(fmt.Sprintf("Unknown mutex type %s \n", typeName))
		}
	}

	if debug {
		fmt.Printf("Goroutine [%d] Exited AfterInterfaceUnlock (id: %d)\n", goid, id)
	}
}

// ================================= WaitGroup =================================
func AfterWaitGroupCreation(wg *sync.WaitGroup, id uint64) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered AfterWaitGroupCreation (id: %d)\n", goid, id)
	}
	syncMonitor.AfterWaitGroupCreation(goid, wg, id)
	if debug {
		fmt.Printf("Goroutine [%d] Exited AfterWaitGroupCreation (id: %d)\n", goid, id)
	}
}

func BeforeWaitGroupAdd(wg *sync.WaitGroup, delta int, id uint64) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered BeforeWaitGroupAdd (id: %d)\n", goid, id)
	}
	syncMonitor.BeforeWaitGroupAdd(goid, wg, delta, id)
	if debug {
		fmt.Printf("Goroutine [%d] Exited BeforeWaitGroupAdd\n", goid)
	}
}

func AfterWaitGroupAdd(wg *sync.WaitGroup, delta int, id uint64) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered AfterWaitGroupAdd (id: %d)\n", goid, id)
	}
	syncMonitor.AfterWaitGroupAdd(goid, wg, delta, id)
	if debug {
		fmt.Printf("Goroutine [%d] Exited AfterWaitGroupAdd (id: %d)\n", goid, id)
	}
}

func BeforeWaitGroupDone(wg *sync.WaitGroup, id uint64) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered BeforeWaitGroupDone (id: %d)\n", goid, id)
	}
	syncMonitor.BeforeWaitGroupDone(goid, wg, id)
	if debug {
		fmt.Printf("Goroutine [%d] Exited BeforeWaitGroupDone (id: %d)\n", goid, id)
	}
}

func AfterWaitGroupDone(wg *sync.WaitGroup, id uint64) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered AfterWaitGroupDone (id: %d)\n", goid, id)
	}
	syncMonitor.AfterWaitGroupDone(goid, wg, id)
	if debug {
		fmt.Printf("Goroutine [%d] Exited AfterWaitGroupDone (id: %d)\n", goid, id)
	}
}

func BeforeWaitGroupWait(wg *sync.WaitGroup, id uint64) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered BeforeWaitGroupWait (id: %d)\n", goid, id)
	}
	syncMonitor.BeforeWaitGroupWait(goid, wg, id)
	if debug {
		fmt.Printf("Goroutine [%d] Exited BeforeWaitGroupWait (id: %d)\n", goid, id)
	}
}

func AfterWaitGroupWait(wg *sync.WaitGroup, id uint64) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered AfterWaitGroupWait (id: %d)\n", goid, id)
	}
	syncMonitor.AfterWaitGroupWait(goid, wg, id)
	if debug {
		fmt.Printf("Goroutine [%d] Exited AfterWaitGroupWait (id: %d)\n", goid, id)
	}
}

func AfterWaitGroupGoRun(wg *sync.WaitGroup, id uint64) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered AfterWaitGroupGoRun (id: %d)\n", goid, id)
	}
	syncMonitor.AfterWaitGroupGoRun(goid, wg, id)
	if debug {
		fmt.Printf("Goroutine [%d] Exited AfterWaitGroupGoRun (id: %d)\n", goid, id)
	}
}

func BeforeWaitGroupGoEnd(wg *sync.WaitGroup, id uint64) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered BeforeWaitGroupGoEnd (id: %d)\n", goid, id)
	}
	syncMonitor.BeforeWaitGroupGoEnd(goid, wg, id)
	if debug {
		fmt.Printf("Goroutine [%d] Exited BeforeWaitGroupGoEnd (id: %d)\n", goid, id)
	}
}

// ================================= Channel =================================
// here we use any to represent channels
// this is because directional channels are not subtypes of bidirectional channels in Go
// so that if we define ch as chan T, we cannot pass chan<- T or <-chan T to the function

func BeforeChannelSend[T any](ch chan<- T, isSelect, isTimer bool, id uint64) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered BeforeChannelSend (id: %d)\n", goid, id)
	}
	syncMonitor.BeforeChannelSend(goid, ch, isSelect, isTimer, id)
	if debug {
		fmt.Printf("Goroutine [%d] Exited BeforeChannelSend \n", goid)
	}
}

func AfterChannelSend[T any](ch chan<- T, id uint64) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered AfterChannelSend (id: %d)\n", goid, id)
	}
	syncMonitor.AfterChannelSend(goid, ch, id)
	if debug {
		fmt.Printf("Goroutine [%d] Exited AfterChannelSend (id: %d)\n", goid, id)
	}
}

func BeforeChannelReceive[T any](ch <-chan T, isSelect, isTimer bool, id uint64) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered BeforeChannelReceive (id: %d)\n", goid, id)
	}
	syncMonitor.BeforeChannelReceive(goid, ch, isSelect, isTimer, id)
	if debug {
		fmt.Printf("Goroutine [%d] Exited BeforeChannelReceive (id: %d)\n", goid, id)
	}
}

func AfterChannelReceive[T any](ch <-chan T, isSelect, isTimer bool, id uint64) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered AfterChannelReceive (id: %d)\n", goid, id)
	}
	syncMonitor.AfterChannelReceive(goid, ch, isSelect, isTimer, id)
	if debug {
		fmt.Printf("Goroutine [%d] Exited AfterChannelReceive (id: %d)\n", goid, id)
	}
}

func BeforeChannelClose[T any](ch chan<- T, id uint64) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered BeforeChannelClose (id: %d)\n", goid, id)
	}
	syncMonitor.BeforeChannelClose(goid, ch, id)
	if debug {
		fmt.Printf("Goroutine [%d] Exited BeforeChannelClose (id: %d)\n", goid, id)
	}
}

func AfterChannelClose[T any](ch chan<- T, id uint64) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered AfterChannelClose (id: %d)\n", goid, id)
	}
	syncMonitor.AfterChannelClose(goid, ch, id)
	if debug {
		fmt.Printf("Goroutine [%d] Exited AfterChannelClose (id: %d)\n", goid, id)
	}
}

func AfterChannelCreation(ch any, cap int, id uint64) {
	// Global channel creation will invoke this function before main test goroutine starts.
	// Therfore, we save global channels in a global map and copy it to the monitor after 
	// main goroutine starts.
	if !hasInitialized.Load() {
		globalChannelMutex.Lock()
		chPtr := utils.GetPtrOf(ch)
		globalChannelStates[chPtr] = monitor_defs.NewChannelState(cap, id)
		globalChannelMutex.Unlock()
		return
	}

	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered AfterChannelCreation (id: %d)\n", goid, id)
	}
	syncMonitor.AfterChannelCreation(goid, ch, cap, id)
	if debug {
		fmt.Printf("Goroutine [%d] Exited AfterChannelCreation \n", goid)
	}
}

// ================================= Cond Var =================================
func AfterCondVarCreation(c *sync.Cond, id uint64) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered AfterCondVarCreation (id: %d)\n", goid, id)
	}
	syncMonitor.AfterCondVarCreation(goid, c, id)
	if debug {
		fmt.Printf("Goroutine [%d] Exited AfterCondVarCreation (id: %d)\n", goid, id)
	}
}

func BeforeCondVarWait(c *sync.Cond, id uint64) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered BeforeCondVarWait (id: %d)\n", goid, id)
	}
	syncMonitor.BeforeCondVarWait(goid, c, id)
	if debug {
		fmt.Printf("Goroutine [%d] Exited BeforeCondVarWait (id: %d)\n", goid, id)
	}
}

func AfterCondVarWait(c *sync.Cond, id uint64) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered AfterCondVarWait (id: %d)\n", goid, id)
	}
	syncMonitor.AfterCondVarWait(goid, c, id)
	if debug {
		fmt.Printf("Goroutine [%d] Exited AfterCondVarWait (id: %d)\n", goid, id)
	}
}

func BeforeCondVarSignal(c *sync.Cond, id uint64) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered BeforeCondVarSignal (id: %d)\n", goid, id)
	}
	syncMonitor.BeforeCondVarSignal(goid, c, id)
	if debug {
		fmt.Printf("Goroutine [%d] Exited BeforeCondVarSignal (id: %d)\n", goid, id)
	}
}

func AfterCondVarSignal(c *sync.Cond, id uint64) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered AfterCondVarSignal (id: %d)\n", goid, id)
	}
	syncMonitor.AfterCondVarSignal(goid, c, id)
	if debug {
		fmt.Printf("Goroutine [%d] Exited AfterCondVarSignal (id: %d)\n", goid, id)
	}
}

func BeforeCondVarBroadcast(c *sync.Cond, id uint64) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered BeforeCondVarBroadcast (id: %d)\n", goid, id)
	}
	syncMonitor.BeforeCondVarBroadcast(goid, c, id)
	if debug {
		fmt.Printf("Goroutine [%d] Exited BeforeCondVarBroadcast\n", goid)
	}
}

func AfterCondVarBroadcast(c *sync.Cond, id uint64) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered AfterCondVarBroadcast (id: %d)\n", goid, id)
	}
	syncMonitor.AfterCondVarBroadcast(goid, c, id)
	if debug {
		fmt.Printf("Goroutine [%d] Exited AfterCondVarBroadcast (id: %d)\n", goid, id)
	}
}

// ================================= Goroutine =================================

func BeforeGoroutineCreation() runtime_types.Goid {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered BeforeGoroutineCreation \n", goid)
	}
	parentGoid := syncMonitor.BeforeGoroutineCreation(goid)
	return parentGoid
}

func AfterNewGoroutineCreation(parentGoid runtime_types.Goid) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered AfterNewGoroutineCreation \n", goid)
	}
	syncMonitor.AfterNewGoroutineCreation(goid, parentGoid)
	if debug {
		fmt.Printf("Goroutine [%d] Exited AfterNewGoroutineCreation \n", goid)
	}
}

func AfterGoroutineCreationCreator() {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered AfterNewGoroutineCreationCreator \n", goid)
	}
	syncMonitor.AfterNewGoroutineCreationCreator(goid)
	if debug {
		fmt.Printf("Goroutine [%d] Exited AfterNewGoroutineCreationCreator \n", goid)
	}
}

func BeforeGoroutineEnd() {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered BeforeGoroutineEnd \n", goid)
	}
	syncMonitor.BeforeGoroutineEnd(goid)
	if debug {
		fmt.Printf("Goroutine [%d] Exited BeforeGoroutineEnd \n", goid)
	}
}

func AfterMainGoroutineCreation() {
	goid := GetGoid()
	lazyInitSyncMonitor()

	if debug {
		fmt.Printf("Goroutine [%d] Entered AfterMainGoroutineCreation \n", goid)
	}
	syncMonitor.AfterMainGoroutineCreation(goid)
	if debug {
		fmt.Printf("Goroutine [%d] Exited AfterMainGoroutineCreation \n", goid)
	}
}

func BeforeMainGoroutineEnd() {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered BeforeMainGoroutineEnd \n", goid)
	}
	syncMonitor.BeforeMainGoroutineEnd(goid)
	if debug {
		fmt.Printf("Goroutine [%d] Exited BeforeMainGoroutineEnd \n", goid)
	}
}

// ================================= For t.Run =================================

// BeforeTRun is called in the parent goroutine before t.Run.
// It's similar to BeforeGoroutineCreation but doesn't involve the GoSpawnChan.
func BeforeTRun() runtime_types.Goid {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered BeforeTRun \n", goid)
	}
	syncMonitor.BeforeTRun(goid)
	if debug {
		fmt.Printf("Goroutine [%d] Exited AfterTRun \n", goid)
	}
	return goid
}

// AfterTRun is called in the child goroutine created by t.Run.
// It's similar to AfterNewGoroutineCreation but doesn't send on GoSpawnChan.
func AfterTRun(parentGoid runtime_types.Goid) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered AfterTRun \n", goid)
	}
	syncMonitor.AfterTRun(goid, parentGoid)
	if debug {
		fmt.Printf("Goroutine [%d] Exited AfterTRun \n", goid)
	}
}

// ================================= Select =================================

func BeforeSelect(sends []any, recvs []any, isSendTimer, isRecvTimer []bool, sendIdxs []int, recvIdxs []int,
	hasDefault bool, id uint64) int {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered BeforeSelect (id: %d)\n", goid, id)
	}
	selected := syncMonitor.BeforeSelect(goid, sends, recvs, isSendTimer, isRecvTimer, sendIdxs, recvIdxs, hasDefault, id)
	if debug {
		fmt.Printf("Goroutine [%d] Exited BeforeSelect (id: %d)\n", goid, id)
	}
	return selected
}

func AfterSelect(sends []any, recvs []any, isSendTimer, isRecvTimer []bool, sendIdxs []int, recvIdxs []int,
	hasDefault bool, selected int, id uint64) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered AfterSelect (id: %d)\n", goid, id)
	}
	syncMonitor.AfterSelect(goid, sends, recvs, isSendTimer, isRecvTimer, sendIdxs, recvIdxs, hasDefault, selected, id)
	if debug {
		fmt.Printf("Goroutine [%d] Exited AfterSelect (id: %d)\n", goid, id)
	}
}

// ================================= Context =================================

func AfterContextCreation(ctx context.Context, ctxType string) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered AfterContextCreation \n", goid)
	}
	syncMonitor.AfterContextCreation(goid, ctx)
	if debug {
		fmt.Printf("Goroutine [%d] Exited AfterContextCreation \n", goid)
	}
}

func BeforeContextCancel(ctx context.Context) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered BeforeContextCancel \n", goid)
	}
	syncMonitor.BeforeContextCancel(goid, ctx)
	if debug {
		fmt.Printf("Goroutine [%d] Exited BeforeContextCancel \n", goid)
	}
}

func AfterContextCancel(ctx context.Context) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered AfterContextCancel \n", goid)
	}
	syncMonitor.AfterContextCancel(goid, ctx)
	if debug {
		fmt.Printf("Goroutine [%d] Exited AfterContextCancel \n", goid)
	}
}

func BeforeAssertion() {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered BeforeAssertion \n", goid)
	}
	syncMonitor.BeforeAssertion(goid)
	if debug {
		fmt.Printf("Goroutine [%d] Exited BeforeAssertion \n", goid)
	}
}

// ================================= Time =================================
func AfterTimerCreation(t any) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered AfterTimerCreation \n", goid)
	}
	syncMonitor.AfterTimerCreation(goid, t)
	if debug {
		fmt.Printf("Goroutine [%d] Exited AfterTimerCreation \n", goid)
	}
}

func BeforeTimerStop(t any) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered BeforeTimerStop \n", goid)
	}
	syncMonitor.BeforeTimerStop(goid, t)
	if debug {
		fmt.Printf("Goroutine [%d] Exited BeforeTimerStop \n", goid)
	}
}

func AfterTimerStop(t any) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered AfterTimerStop \n", goid)
	}
	syncMonitor.AfterTimerStop(goid, t)
	if debug {
		fmt.Printf("Goroutine [%d] Exited AfterTimerStop \n", goid)
	}
}

func BeforeTimerReset(t any) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered BeforeTimerReset \n", goid)
	}
	syncMonitor.BeforeTimerReset(goid, t)
	if debug {
		fmt.Printf("Goroutine [%d] Exited BeforeTimerReset \n", goid)
	}
}

func AfterTimerReset(t any) {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered AfterTimerReset \n", goid)
	}
	syncMonitor.AfterTimerReset(goid, t)
	if debug {
		fmt.Printf("Goroutine [%d] Exited AfterTimerReset \n", goid)
	}
}

// ================================= Loop =================================
func OnEachLoopIteration() {
	goid := GetGoid()
	if debug {
		fmt.Printf("Goroutine [%d] Entered OnEachLoopIteration \n", goid)
	}
	syncMonitor.OnEachLoopIteration(goid)
	if debug {
		fmt.Printf("Goroutine [%d] Exited OnEachLoopIteration \n", goid)
	}
}
