package sync_shadow

import (
	"time"

	"github.com/focs-lab/gct/monitor"
)

type Timer struct {
	timer     *time.Timer
	C         <-chan time.Time
}

func NewTimer(d time.Duration) *Timer {
	realTimer := time.NewTimer(0)
	timer := &Timer{
		timer:     realTimer,
		C:         realTimer.C,
	}
	monitor.AfterTimerCreation(timer)
	return timer
}

func (t *Timer) Stop() bool {
	monitor.BeforeTimerStop(t)
	ret := t.timer.Stop()
	monitor.AfterTimerStop(t)
	return ret
}

func (t *Timer) Reset(d time.Duration) bool {
	monitor.BeforeTimerReset(t)
	ret := t.timer.Reset(0)
	monitor.AfterTimerReset(t)
	return ret
}

func (t *Timer) GetC() <-chan time.Time {
	return t.C
}