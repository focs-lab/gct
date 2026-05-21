package tracing

import (
	"os"
	"strings"
	"sync"

	"github.com/focs-lab/gct/config"
	"github.com/focs-lab/gct/runtime_types/monitor_defs"
)

type Tracer struct {
	sync.Mutex
	events []*monitor_defs.SyncEvent
}

func NewTracer() *Tracer {
	return &Tracer{
		events: make([]*monitor_defs.SyncEvent, 0),
	}
}

func (t *Tracer) TraceEvent(event *monitor_defs.SyncEvent) {
	t.Lock()
	t.events = append(t.events, event)
	t.Unlock()
}

func (t *Tracer) ToFile() {
	filename := config.TRACER_TRACE_LOC

	t.Lock()
	defer t.Unlock()

	eventStrings := make([]string, len(t.events))
	for i, event := range t.events {
		eventStrings[i] = event.String()
	}

	content := strings.Join(eventStrings, "\n")
	os.WriteFile(filename, []byte(content), 0644)
}
