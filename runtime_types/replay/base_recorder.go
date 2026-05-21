package replay

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/focs-lab/gct/config"
)

type BaseEvent struct {
	isPair bool

	// For single event
	lid     *LogicalId
	caseIdx int
	hasCase bool

	// For pair event
	sender          *LogicalId
	receiver        *LogicalId
	senderCaseIdx   int
	senderHasCase   bool
	receiverCaseIdx int
	receiverHasCase bool
}

type BaseRecorder struct {
	mu    sync.Mutex
	trace []string
}

func NewBaseRecorder() *BaseRecorder {
	return &BaseRecorder{
		trace: make([]string, 0),
		mu:    sync.Mutex{},
	}
}

func (r *BaseRecorder) Record(lid *LogicalId, caseIdx int, hasCase bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	event := lid.String()
	if hasCase {
		event = fmt.Sprintf("%s:%d", event, caseIdx)
	}
	r.trace = append(r.trace, event)
}

func (r *BaseRecorder) RecordPair(sender *LogicalId, receiver *LogicalId, senderCaseIdx int, senderHasCase bool, receiverCaseIdx int, receiverHasCase bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	senderEvent := sender.String()
	if senderHasCase {
		senderEvent = fmt.Sprintf("%s:%d", senderEvent, senderCaseIdx)
	}

	receiverEvent := receiver.String()
	if receiverHasCase {
		receiverEvent = fmt.Sprintf("%s:%d", receiverEvent, receiverCaseIdx)
	}
	r.trace = append(r.trace, fmt.Sprintf("%s,%s", senderEvent, receiverEvent))
}

func (r *BaseRecorder) RecordSeed(seed int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.trace = append(r.trace, fmt.Sprintf("Seed: %d", seed))
}

func (r *BaseRecorder) ToFile() error {
	filename := config.RECORDER_TRACE_LOC

	r.mu.Lock()
	defer r.mu.Unlock()

	content := strings.Join(r.trace, "\n")
	return os.WriteFile(filename, []byte(content), 0644)
}

func (r *BaseRecorder) BuildFromTrace(traceLoc string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	file, err := os.Open(traceLoc)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	r.trace = make([]string, 0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		r.trace = append(r.trace, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		panic(err)
	}
}

func (r *BaseRecorder) GetTotalNumEvents() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.trace)
}

func (r *BaseRecorder) GetEvent(idx int) string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.trace[idx]
}
