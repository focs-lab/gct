package rwmutex

import (
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
)

func TestRWMutex_1(t *testing.T) {
	m := sync.RWMutex{}

	go func() {
		m.RLock()
		m.RUnlock()
	}()

	go func() {
		if success := m.TryLock(); success {
			m.Unlock()
		}
	}()

	m.Lock()
	m.Unlock()
}

func TestRWMutex_2(t *testing.T) {
	m := sync.RWMutex{}
	m.Lock()
	defer m.Unlock()
	x := 1
	_ = x
}

func TestRWMutex_3(t *testing.T) {
	m := sync.RWMutex{}
	m.RLock()
	defer m.RUnlock()
	x := 1
	_ = x
}

func TestRWMutex_4(t *testing.T) {
	m := sync.RWMutex{}
	success := m.TryLock()

	if success {
		x := 1
		println(x)
		m.Unlock()
	}
}

func TestRWMutex_5(t *testing.T) {
	m := sync.RWMutex{}
	success := m.TryRLock()

	if success {
		x := 1
		println(x)
		m.RUnlock()
	}
}

func TestRWMutex_6(t *testing.T) {
	m := sync.RWMutex{}

	if success := m.TryLock(); success {
		x := 1
		_ = x
		m.Unlock()
	}
}

func TestRWMutex_7(t *testing.T) {
	m1 := sync.RWMutex{}
	m2 := &sync.RWMutex{}

	m1.Lock()
	m2.Lock()
	m2.Unlock()
	m1.Unlock()
}

func TestRWMutex_8(t *testing.T) {
	type MyStruct struct {
		*sync.RWMutex
	}

	s := MyStruct{}
	s.Lock()
	sr := s.RLocker()
	sr.Lock()
	sr.Unlock()
	s.Unlock()
}

func TestRWMutex_9(t *testing.T) {
	m := &sync.RWMutex{}

	go func() {
		m.RLock()
		println("G1 after 1st rlock")
		m.RLock()
		println("G1 after 2nd rlock")
		m.RUnlock()
		m.RUnlock()
		println("G1 finished")
	}()

	go func() {
		m.Lock()
		println("G2 after lock")
		m.Unlock()
		println("G2 finished")
	}()
}

func TestRWMutex_10(t *testing.T) {
	type MyStruct struct {
		*sync.RWMutex
	}

	s := MyStruct{}

	type MyStruct2 struct {
		sync.RWMutex
	}

	s2 := MyStruct2{}
	_ = s2
	s.Lock()
	sr := s.RLocker()
	sr.Lock()
	sr.Unlock()
	s.Unlock()
}

func TestRWMutex_11(t *testing.T) {
	type MyStruct struct {
		Mu *sync.RWMutex
	}

	s := MyStruct{
		Mu: &sync.RWMutex{},
	}

	type MyStruct2 struct {
		mu *sync.RWMutex
	}

	s1 := MyStruct2{
		mu: &sync.RWMutex{},
	}

	_ = s
	_ = s1
}

func TestRWMutex_12(t *testing.T) {
	m := sync.RWMutex{}
	wrapper(m.Lock)
	wrapper(m.Unlock)
	wrapper2(m.TryLock)
	wrapper2(m.TryRLock)
	wrapper(m.RLock)
	wrapper(m.RUnlock)
	wrapper(m.RLocker().Lock)
	wrapper(m.RLocker().Unlock)
}

func wrapper(f func()) {
	f()
}

func wrapper2(f func() bool) {

}

func TestRWMutex_13(t *testing.T) {
	m := sync.RWMutex{}
	var x int

	go func() {
		for i := 0; i < 2; i++ {
			m.Lock()
			x = i
			m.Unlock()
		}
	}()

	m.Lock()
	x = 3
	m.Unlock()

	assert.Equal(t, x, 1)
}

type wrapper3 struct {
	batchTx
}

type batchTx struct {
	sync.RWMutex
	pending int
}

func TestMutex_10(t *testing.T) {
	w := &wrapper3{}

	w.Lock()
	w.Unlock()

}
