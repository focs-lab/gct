package monitor

import (
	"time"
)

// time.AfterFunc(d, f)
func TimeAfterFunc(d time.Duration, f func()) *time.Timer {
	parentId := BeforeGoroutineCreation()

	call := func() {
		AfterNewGoroutineCreation(parentId)
		defer BeforeGoroutineEnd()
		f()
	}

	timer := time.AfterFunc(1*time.Millisecond, call)
	AfterGoroutineCreationCreator()
	return timer
}

// time.After(d)
// make this immediately avaiable
func TimeAfter(d time.Duration) <-chan time.Time {
	ch := make(chan time.Time, 1)
	ch <- time.Now()
	return ch
}