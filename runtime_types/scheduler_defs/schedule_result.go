package scheduler_defs

import (
	"github.com/focs-lab/gct/runtime_types/monitor_defs"
)

type ScheduleResult struct {
	IsSingleGoroutine bool
	SingleGoroutine *monitor_defs.SingleGoroutineOption
	GoroutinePair   *monitor_defs.GoroutineRendezvousPairOption
}