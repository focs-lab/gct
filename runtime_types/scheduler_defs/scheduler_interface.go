package scheduler_defs

import (
	"github.com/focs-lab/gct/runtime_types/monitor_defs"
	"github.com/focs-lab/gct/runtime_types"
)

type Scheduler interface {
	MakeNextSchedulingDecision() *ScheduleResult
	OnSyncEvent(e *monitor_defs.SyncEvent, result *ScheduleResult) 
	OnNewGoroutineBegin(goid runtime_types.Goid, isMain bool) // To register new goroutines
	SetMonitor(monitor monitor_defs.SyncMonitorSched) 
	OnTermination()
}
