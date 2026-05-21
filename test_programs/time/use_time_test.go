package use_time

import (
	"testing"
	"time"
)

func TestTimerSelectOri(t *testing.T) {
	timer := time.NewTimer(time.Nanosecond)
	ticker := time.NewTicker(time.Nanosecond)

	select {
		case <-timer.C:
			println("Timer expired")
		case <-ticker.C:
			println("Ticker expired")
	}
}

func TestTimerSelectReceived(t *testing.T) {
	timer := time.NewTimer(time.Nanosecond)
	ticker := time.NewTicker(time.Nanosecond)

	<-timer.C
 <-ticker.C
	select {
		case <-timer.C:
			println("Timer expired")
		case <-ticker.C:
			println("Ticker expired")
	}
}

func TestTimerSelectStoped(t *testing.T) {
	timer := time.NewTimer(time.Nanosecond)
	ticker := time.NewTicker(time.Nanosecond)

	timer.Stop()
	ticker.Stop()

	select {
		case <-timer.C:
			println("Timer expired")
		case <-ticker.C:
			println("Ticker expired")
	}
}

func TestTimerSelectReset(t *testing.T) {
	timer := time.NewTimer(time.Nanosecond)
	ticker := time.NewTicker(time.Nanosecond)

	timer.Stop()
	timer.Reset(time.Nanosecond)
	ticker.Stop()
	ticker.Reset(time.Nanosecond)

	select {
		case <-timer.C:
			println("Timer expired")
		case <-ticker.C:
			println("Ticker expired")
	}
}

func TestTimerReset(t *testing.T) {
	timer := time.NewTimer(time.Nanosecond)
	resetChan := make(chan struct{}, 1)

	go func() {
		for {
			select {
			case <-timer.C:
				println("Timer expired")
			case <-resetChan:
				if !timer.Stop() {
					println("before draining the channel")
					<-timer.C
					println("after draining the channel")
				}
				timer.Reset(time.Nanosecond)
			}
		}
	}()

	go func() {
		for i := 0; i < 1024; i++ {
			resetChan <- struct{}{}
		}
	}()

	time.Sleep(time.Second)
}

func TestTimerSemantics(t *testing.T) {
	timer := time.NewTimer(0)

	time.Sleep(30 * time.Millisecond)

	timer.Stop()
	<-timer.C
}

type MyStruct struct {
	t time.Time
}

func TestTimeTimeAfter(t *testing.T) {
	m := &MyStruct{}
	if m.t.After(time.Now()) {
		println("123")
	}
}
