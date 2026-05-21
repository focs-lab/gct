package monitor_defs

import (
	"context"
	"sync"

	runtime_types "github.com/focs-lab/gct/runtime_types"
	replay "github.com/focs-lab/gct/runtime_types/replay"
)

type SyncMonitorEntry interface {
	// Goroutines
	AfterNewGoroutineCreation(goid runtime_types.Goid, parentGoid runtime_types.Goid)
	AfterMainGoroutineCreation(goid runtime_types.Goid)
	AfterNewGoroutineCreationCreator(goid runtime_types.Goid)

	BeforeGoroutineCreation(goid runtime_types.Goid) runtime_types.Goid
	BeforeGoroutineEnd(goid runtime_types.Goid)
	BeforeMainGoroutineEnd(goid runtime_types.Goid)
	BeforeTRun(goid runtime_types.Goid)
	AfterTRun(goid, parentGoId runtime_types.Goid)

	// Mutexes
	AfterMutexCreation(goid runtime_types.Goid, m *sync.Mutex, id uint64)

	BeforeMutexLock(goid runtime_types.Goid, m *sync.Mutex, id uint64)
	AfterMutexLock(goid runtime_types.Goid, m *sync.Mutex, id uint64)

	BeforeMutexUnlock(goid runtime_types.Goid, m *sync.Mutex, id uint64)
	AfterMutexUnlock(goid runtime_types.Goid, m *sync.Mutex, id uint64)

	BeforeMutexTryLock(goid runtime_types.Goid, m *sync.Mutex, id uint64)
	AfterMutexTryLock(goid runtime_types.Goid, m *sync.Mutex, id uint64)

	// RWMutexes
	AfterRWMutexCreation(goid runtime_types.Goid, m *sync.RWMutex, id uint64)

	BeforeRWMutexLock(goid runtime_types.Goid, m *sync.RWMutex, id uint64)
	AfterRWMutexLock(goid runtime_types.Goid, m *sync.RWMutex, id uint64)

	BeforeRWMutexUnlock(goid runtime_types.Goid, m *sync.RWMutex, id uint64)
	AfterRWMutexUnlock(goid runtime_types.Goid, m *sync.RWMutex, id uint64)

	BeforeRWMutexTryLock(goid runtime_types.Goid, m *sync.RWMutex, id uint64)
	AfterRWMutexTryLock(goid runtime_types.Goid, m *sync.RWMutex, id uint64)

	BeforeRWMutexTryRLock(goid runtime_types.Goid, m *sync.RWMutex, id uint64)
	AfterRWMutexTryRLock(goid runtime_types.Goid, m *sync.RWMutex, id uint64)

	BeforeRWMutexRUnlock(goid runtime_types.Goid, m *sync.RWMutex, id uint64)
	AfterRWMutexRUnlock(goid runtime_types.Goid, m *sync.RWMutex, id uint64)

	BeforeRWMutexRLock(goid runtime_types.Goid, m *sync.RWMutex, id uint64)
	AfterRWMutexRLock(goid runtime_types.Goid, m *sync.RWMutex, id uint64)

	// Channels
	BeforeChannelSend(goid runtime_types.Goid, ch any, isSelect, isTimer bool, id uint64)
	AfterChannelSend(goid runtime_types.Goid, ch any, id uint64)

	BeforeChannelReceive(goid runtime_types.Goid, ch any, isSelect, isTimer bool, id uint64)
	AfterChannelReceive(goid runtime_types.Goid, ch any, isSelect, isTimer bool, id uint64)

	BeforeChannelClose(goid runtime_types.Goid, ch any, id uint64)
	AfterChannelClose(goid runtime_types.Goid, ch any, id uint64)

	AfterChannelCreation(goid runtime_types.Goid, ch any, cap int, d uint64)

	// WaitGroup
	AfterWaitGroupCreation(goid runtime_types.Goid, wg *sync.WaitGroup, id uint64)
	BeforeWaitGroupAdd(goid runtime_types.Goid, wg *sync.WaitGroup, delta int, id uint64)
	AfterWaitGroupAdd(goid runtime_types.Goid, wg *sync.WaitGroup, delta int, id uint64)

	BeforeWaitGroupDone(goid runtime_types.Goid, wg *sync.WaitGroup, id uint64)
	AfterWaitGroupDone(goid runtime_types.Goid, wg *sync.WaitGroup, id uint64)

	BeforeWaitGroupWait(goid runtime_types.Goid, wg *sync.WaitGroup, id uint64)
	AfterWaitGroupWait(goid runtime_types.Goid, wg *sync.WaitGroup, id uint64)

	AfterWaitGroupGoRun(goid runtime_types.Goid, wg *sync.WaitGroup, id uint64)
	BeforeWaitGroupGoEnd(goid runtime_types.Goid, wg *sync.WaitGroup, id uint64)

	// Cond
	AfterCondVarCreation(goid runtime_types.Goid, c *sync.Cond, id uint64)
	
	BeforeCondVarWait(goid runtime_types.Goid, c *sync.Cond, id uint64)
	AfterCondVarWait(goid runtime_types.Goid, c *sync.Cond, id uint64)

	BeforeCondVarSignal(goid runtime_types.Goid, c *sync.Cond, id uint64)
	AfterCondVarSignal(goid runtime_types.Goid, c *sync.Cond, id uint64)

	BeforeCondVarBroadcast(goid runtime_types.Goid, c *sync.Cond, id uint64)
	AfterCondVarBroadcast(goid runtime_types.Goid, c *sync.Cond, id uint64)

	// Select
	BeforeSelect(goid runtime_types.Goid, sends []any, recvs []any, isSendTimer, isRecvTimer []bool, sendIdxs []int, recvIdxs []int, hasDefault bool, opId uint64) int
	AfterSelect(goid runtime_types.Goid, sends []any, recvs []any, isSendTimer, isRecvTimer []bool, sendIdxs []int, recvIdxs []int, hasDefault bool, selected int, opId uint64)

	// Context
	AfterContextCreation(goid runtime_types.Goid, ctx context.Context)
	BeforeContextCancel(goid runtime_types.Goid, ctx context.Context)
	AfterContextCancel(goid runtime_types.Goid, ctx context.Context)

	// Time
	AfterTimerCreation(goid runtime_types.Goid, t any)
	BeforeTimerReset(goid runtime_types.Goid, t any)
	AfterTimerReset(goid runtime_types.Goid, t any)
	BeforeTimerStop(goid runtime_types.Goid, t any)
	AfterTimerStop(goid runtime_types.Goid, t any)

	// Assertion
	BeforeAssertion(goid runtime_types.Goid)

	// Loop
	OnEachLoopIteration(goid runtime_types.Goid)
}

type SyncMonitorSched interface {
	GetGoroutinesWithStatus(expected GoStatus) []runtime_types.Goid
	GetExecutableChoices() ([]*SingleGoroutineOption, []*SingleGoroutineOption, []*GoroutineRendezvousPairOption)
	GetGoroutineState(goid runtime_types.Goid) *GoroutineState
	GetNumRunningGoroutines() int
	GetNumCreatingGoroutines() int
	GetNumAliveGoroutines() int
	CheckConflict(goid1, goid2 runtime_types.Goid) bool
	IsNextOperationReadLike(goid runtime_types.Goid) bool
	IsNextOperationWriteLike(goid runtime_types.Goid) bool
	GetChannelState(chAddr uintptr) (*ChannelState, bool)

	// methods for recording
	GetLid(goid runtime_types.Goid) *replay.LogicalId
	RecordSeed(seed int64)

	// methods for tracing
	LogSyncEvent(goid runtime_types.Goid)
}