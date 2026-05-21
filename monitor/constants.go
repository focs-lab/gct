package monitor

import (
	"github.com/focs-lab/gct/runtime_types"
	"github.com/focs-lab/gct/runtime_types/monitor_defs"
)

// used in instrumentation passes, for controlling the name of hooks
const (
	BEFORE_INTERFACE_LOCK = "BeforeInterfaceLock"
	AFTER_INTERFACE_LOCK  = "AfterInterfaceLock"

	BEFORE_INTERFACE_UNLOCK = "BeforeInterfaceUnlock"
	AFTER_INTERFACE_UNLOCK  = "AfterInterfaceUnlock"

	BEFORE_MUTEX_LOCK = "BeforeMutexLock"
	AFTER_MUTEX_LOCK  = "AfterMutexLock"

	BEFORE_MUTEX_UNLOCK = "BeforeMutexUnlock"
	AFTER_MUTEX_UNLOCK  = "AfterMutexUnlock"

	BEFORE_MUTEX_TRYLOCK = "BeforeMutexTryLock"
	AFTER_MUTEX_TRYLOCK  = "AfterMutexTryLock"

	AFTER_MUTEX_CREATION = "CreateMutex"
)

const (
	BEFORE_RWMUTEX_LOCK = "BeforeRWMutexLock"
	AFTER_RWMUTEX_LOCK  = "AfterRWMutexLock"

	BEFORE_RWMUTEX_UNLOCK = "BeforeRWMutexUnlock"
	AFTER_RWMUTEX_UNLOCK  = "AfterRWMutexUnlock"

	BEFORE_RWMUTEX_TRYLOCK = "BeforeRWMutexTryLock"
	AFTER_RWMUTEX_TRYLOCK  = "AfterRWMutexTryLock"

	BEFORE_RWMUTEX_TRYRLOCK = "BeforeRWMutexTryRLock"
	AFTER_RWMUTEX_TRYRLOCK  = "AfterRWMutexTryRLock"

	BEFORE_RWMUTEX_RLOCK = "BeforeRWMutexRLock"
	AFTER_RWMUTEX_RLOCK  = "AfterRWMutexRLock"

	BEFORE_RWMUTEX_RUNLOCK = "BeforeRWMutexRUnlock"
	AFTER_RWMUTEX_RUNLOCK  = "AfterRWMutexRUnlock"

	AFTER_RWMUTEX_CREATION = "CreateRWMutex"
)

const (
	BEFORE_GOROUTINE_CREATION        = "BeforeGoroutineCreation"
	AFTER_GOROUTINE_CREATION         = "AfterNewGoroutineCreation"
	AFTER_GOROUTINE_CREATION_CREATOR = "AfterGoroutineCreationCreator"

	BEFORE_GOROUTINE_END          = "BeforeGoroutineEnd"
	AFTER_MAIN_GOROUTINE_CREATION = "AfterMainGoroutineCreation"

	BEFORE_MAIN_GOROUTINE_END = "BeforeMainGoroutineEnd"

	BEFORE_T_RUN = "BeforeTRun"
	AFTER_T_RUN  = "AfterTRun"
)

const (
	BEFORE_WAITGROUP_ADD = "BeforeWaitGroupAdd"
	AFTER_WAITGROUP_ADD  = "AfterWaitGroupAdd"

	BEFORE_WAITGROUP_DONE = "BeforeWaitGroupDone"
	AFTER_WAITGROUP_DONE  = "AfterWaitGroupDone"

	BEFORE_WAITGROUP_WAIT = "BeforeWaitGroupWait"
	AFTER_WAITGROUP_WAIT  = "AfterWaitGroupWait"

	AFTER_WAITGROUP_GO_RUN = "AfterWaitGroupGoRun"
	BEFORE_WAITGROUP_GO_END  = "BeforeWaitGroupGoEnd"

	AFTER_WAITGROUP_CREATION = "CreateWaitGroup"
)

const (
	BEFORE_COND_VAR_WAIT = "BeforeCondVarWait"
	AFTER_COND_VAR_WAIT  = "AfterCondVarWait"
	BEFORE_COND_VAR_SIGNAL = "BeforeCondVarSignal"
	AFTER_COND_VAR_SIGNAL  = "AfterCondVarSignal"
	BEFORE_COND_VAR_BROADCAST = "BeforeCondVarBroadcast"
	AFTER_COND_VAR_BROADCAST  = "AfterCondVarBroadcast"
)

const (
	AFTER_CONTEXT_CREATION = "AfterContextCreation"
	BEFORE_CONTEXT_CANCEL = "BeforeContextCancel"
	AFTER_CONTEXT_CANCEL  = "AfterContextCancel"
)

const (
	AFTER_CHANNEL_CREATION = "AfterChannelCreation"

	BEFORE_CHANNEL_SEND = "BeforeChannelSend"
	AFTER_CHANNEL_SEND  = "AfterChannelSend"

	BEFORE_CHANNEL_RECV = "BeforeChannelReceive"
	AFTER_CHANNEL_RECV  = "AfterChannelReceive"

	BEFORE_CHANNEL_CLOSE = "BeforeChannelClose"
	AFTER_CHANNEL_CLOSE  = "AfterChannelClose"

	CHANNEL_SEND   = "ChannelSend"
	CHANNEL_RECV   = "ChannelRecv"
	CHANNEL_CLOSE  = "ChannelClose"
	CHANNEL_CREATE = "ChannelCreate"
)

const (
	BEFORE_SELECT = "BeforeSelect"
	AFTER_SELECT  = "AfterSelect"
)

const (
	BEFORE_ASSERTION           = "BeforeAssertion"
	ASSERTION_ASSERT_PKG_PATH  = "github.com/stretchr/testify/assert"
	ASSERTION_REQUIRE_PKG_PATH = "github.com/stretchr/testify/require"
)

const (
	ON_EACH_LOOP_ITERATION = "OnEachLoopIteration"
)

const (
	TIME_AFTER_FUNC = "TimeAfterFunc"
	TIME_AFTER      = "TimeAfter"
	NEW_TIMER       = "NewTimer"
	NEW_TICKER      = "NewTicker"
)

type Goid = runtime_types.Goid
type WakeupMessage = monitor_defs.WakeupMessage
type GoroutineRendezvousPairOption = monitor_defs.GoroutineRendezvousPairOption
type SingleGoroutineOption = monitor_defs.SingleGoroutineOption
