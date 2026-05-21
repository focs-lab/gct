package monitor

import (
	mrand "math/rand"
	"sync"
	"time"
	"fmt"
	"runtime"
	"strconv"
)

const debugMutex = false

type DebugMutex struct {
	mu             sync.RWMutex
	lock_id        string
	rlock_id       string
	last_unlock    string
	last_runlock   string
	rlock_id_mutex sync.Mutex
}

func (m *DebugMutex) Lock() {

	// lock the mutex
	(*m).mu.Lock()

	if m.lock_id == "" {
		// (*m).lock_id has not been initialized, set it to a random value
		(*m).lock_id = strconv.Itoa(mrand.Intn(1000000000000000000))
	}

	// get the file and line number that called this function
	pc, _, _, ok := runtime.Caller(1)
	if ok {
		details := runtime.FuncForPC(pc)
		file, line := details.FileLine(pc)
		locked_mutexes_mutex.Lock()
		// store when and where the mutex was locked
		locked_mutexes[(*m).lock_id] = fmt.Sprintf("mutex locked %s:%d @ %s", file, line, time.Now().String())
		locked_mutexes_mutex.Unlock()
	}

}

func (m *DebugMutex) Unlock() {

	// try to lock the mutex
	var locked = (*m).TryLock()

	if locked == true {

		// the expected
		// fatal error: sync: Unlock of unlocked RWMutex
		// because the mutex is already unlocked

		locked_mutexes_mutex.Lock()
		// print when and where the mutex was unlocked
		fmt.Println((*m).last_unlock)
		locked_mutexes_mutex.Unlock()

		// unlock the mutex to allow the fatal error to happen
		// the user can read the last unlock in the lines before the fatal error
		(*m).Unlock()

	}

	// get the file and line number that called this function
	pc, _, _, ok := runtime.Caller(1)
	if ok {
		details := runtime.FuncForPC(pc)
		file, line := details.FileLine(pc)
		locked_mutexes_mutex.Lock()
		// store when and where the mutex was unlocked
		(*m).last_unlock = fmt.Sprintf("last unlock %s:%d @ %s", file, line, time.Now().String())
		// delete the mutex lock log entry
		delete(locked_mutexes, (*m).lock_id)
		locked_mutexes_mutex.Unlock()
	}

	// unlock the mutex
	(*m).mu.Unlock()

}

func (m *DebugMutex) TryLock() bool {
	return (*m).mu.TryLock()
}

func (m *DebugMutex) TryRLock() bool {
	return (*m).mu.TryRLock()
}

func (m *DebugMutex) RLock() {

	// rlock the mutex
	(*m).mu.RLock()

	(*m).rlock_id_mutex.Lock()
	if (*m).rlock_id == "" {
		// (*m).rlock_id has not been initialized, set it to a random value
		(*m).rlock_id = strconv.Itoa(mrand.Intn(1000000000000000000))
	}
	(*m).rlock_id_mutex.Unlock()

	// get the file and line number that called this function
	pc, _, _, ok := runtime.Caller(1)
	if ok {
		details := runtime.FuncForPC(pc)
		file, line := details.FileLine(pc)
		rlocked_mutexes_mutex.Lock()
		// store when and where the mutex was rlocked
		rlocked_mutexes[(*m).rlock_id] = fmt.Sprintf("%s:%d @ %s", file, line, time.Now().String())
		rlocked_mutexes_mutex.Unlock()
	}

}

func (m *DebugMutex) RUnlock() {

	// try to lock the mutex
	var locked = (*m).TryLock()

	if locked == true {

		// the expected
		// fatal error: sync: RUnlock of unlocked RWMutex
		// because the mutex is already runlocked

		rlocked_mutexes_mutex.Lock()
		// print when and where the mutex was runlocked
		fmt.Println((*m).last_runlock)
		rlocked_mutexes_mutex.Unlock()

		// unlock the mutex to allow the fatal error to happen
		// the user can read the last runlock in the lines before the fatal error
		(*m).Unlock()

	}

	// get the file and line number that called this function
	pc, _, _, ok := runtime.Caller(1)
	if ok {
		details := runtime.FuncForPC(pc)
		file, line := details.FileLine(pc)
		rlocked_mutexes_mutex.Lock()
		// store when and where the mutex was runlocked
		(*m).last_runlock = fmt.Sprintf("last runlock %s:%d @ %s", file, line, time.Now().String())
		// delete the mutex rlock log entry
		delete(rlocked_mutexes, (*m).rlock_id)
		rlocked_mutexes_mutex.Unlock()
	}

	// runlock the mutex
	(*m).mu.RUnlock()

}

var locked_mutexes_mutex = sync.Mutex{}
var rlocked_mutexes_mutex = sync.Mutex{}
var locked_mutexes map[string]string
var rlocked_mutexes map[string]string

func init() {
	if !debugMutex {
		return
	}

	locked_mutexes = make(map[string]string)
	rlocked_mutexes = make(map[string]string)

	// show locked and rlocked mutexes
	go func() {

		for {

			locked_mutexes_mutex.Lock()
			if len(locked_mutexes) > 0 {
				fmt.Println("locked mutexes")
				for lock_id := range locked_mutexes {
					fmt.Println("\t", locked_mutexes[lock_id])
				}
			}
			locked_mutexes_mutex.Unlock()

			rlocked_mutexes_mutex.Lock()
			if len(rlocked_mutexes) > 0 {
				fmt.Println("rlocked mutexes")
				for rlock_id := range rlocked_mutexes {
					fmt.Println("\t", rlocked_mutexes[rlock_id])
				}
			}
			rlocked_mutexes_mutex.Unlock()

			time.Sleep(time.Second * 2)

		}

	}()
}
