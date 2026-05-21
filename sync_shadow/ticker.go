package sync_shadow

import (
	"time"

	"github.com/focs-lab/gct/monitor"
)

type Ticker struct {
	ticker     *time.Ticker
	C         <-chan time.Time
}

func NewTicker(d time.Duration) *Ticker {
	realTicker := time.NewTicker(1 * time.Microsecond)
	ticker := &Ticker{
		ticker:     realTicker,
		C:         realTicker.C,
	}
	monitor.AfterTimerCreation(ticker)
	return ticker
}

func (t *Ticker) Stop() {
	monitor.BeforeTimerStop(t)
	t.ticker.Stop()
	monitor.AfterTimerStop(t)
}

func (t *Ticker) Reset(d time.Duration) {
	monitor.BeforeTimerReset(t)
	t.ticker.Reset(1 * time.Microsecond)
	monitor.AfterTimerReset(t)
}

func (t *Ticker) GetC() <-chan time.Time {
	return t.C
}