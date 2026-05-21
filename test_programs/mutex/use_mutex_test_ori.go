package test_programs

// import (
// 	"sync"
// 	"testing"

// 	"github.com/stretchr/testify/assert"
// )

// func TestMutex_1(t *testing.T) {
// 	m := sync.Mutex{}
// 	m.Lock()

// 	m.Unlock()
// }

// func TestMutex_2(t *testing.T) {
// 	m := sync.Mutex{}
// 	x := 1
// 	go func() {
// 		m.Lock()
// 		x = 2
// 		m.Unlock()
// 	}()
// 	m.Lock()
// 	x = 3
// 	m.Unlock()

// 	assert.Equal(t, x, 2)
// }

// func TestMutex_3(t *testing.T) {
// 	m1 := &sync.Mutex{}
// 	m2 := &sync.Mutex{}
// 	x := 1
// 	go func() {
// 		m1.Lock()
// 		m2.Lock()
// 		x = 2
// 		m2.Unlock()
// 		m1.Unlock()
// 	}()
// 	m1.Lock()
// 	m2.Lock()
// 	x = 3
// 	m2.Unlock()
// 	m1.Unlock()

// 	assert.Equal(t, x, 2)
// }

// func TestMutex_4(t *testing.T) {
// 	m1 := &sync.Mutex{}
// 	m2 := &sync.Mutex{}
// 	x := 1
// 	go func() {
// 		m2.Lock()
// 		m1.Lock()
// 		x = 2
// 		m1.Unlock()
// 		m2.Unlock()
// 	}()
// 	m1.Lock()
// 	m2.Lock()
// 	x = 3
// 	m2.Unlock()
// 	m1.Unlock()

// 	assert.Equal(t, x, 2)
// }

// func wrapper(f func()) {
// 	f()
// }

// func wrapper2(f func() bool) {

// }

// func TestMutex_5(t *testing.T) {
// 	m := sync.Mutex{}
// 	wrapper(m.Lock)
// 	wrapper(m.Unlock)
// 	wrapper2(m.TryLock)
// }

// type myStruct struct {
// 	mu sync.Mutex
// 	sync.Mutex
// 	pmu *sync.Mutex

// 	rwmu sync.RWMutex
// 	sync.RWMutex
// 	prwmu *sync.RWMutex

// 	cond sync.Cond
// 	sync.Cond

// 	sync.WaitGroup
// 	wg sync.WaitGroup

// 	x  int
// }

// func returnMyStruct() *myStruct {
// 	return &myStruct{
// 		mu: sync.Mutex{},
// 		pmu: &sync.Mutex{},
// 		rwmu: sync.RWMutex{},
// 		prwmu: &sync.RWMutex{},
// 		cond: sync.Cond{},
// 		wg: sync.WaitGroup{},

// 		x:  1,
// 	}
// }

// func TestMutex_6(t *testing.T) {
// 	m := sync.Mutex{}
// 	var x int

// 	go func() {
// 		for i := 0; i < 2; i++ {
// 			m.Lock()
// 			x = i
// 			m.Unlock()
// 		}
// 	}()

// 	s := returnMyStruct()
// 	s.mu.Lock()
// 	defer s.mu.Unlock()
// 	x = 3

// 	assert.Equal(t, x, 1)
// }

// type embedMutex struct {
// 	sync.Mutex
// 	mu sync.Mutex
// }

// type embedMutex2 struct {
// 	*sync.Mutex
// 	mu *sync.Mutex
// }

// func TestMutex_7(t *testing.T) {
// 	e := embedMutex{}

// 	e.Lock()
// 	defer e.Unlock()
// 	e.mu.Lock()
// 	defer e.mu.Unlock()

// 	e2 := &embedMutex{}
// 	e2.Lock()
// 	defer e2.Unlock()
// 	e2.mu.Lock()
// 	defer e2.mu.Unlock()
// }

// func TestMutex_8(t *testing.T) {
// 	e := embedMutex2{}

// 	e.Lock()
// 	defer e.Unlock()
// 	e.mu.Lock()
// 	defer e.mu.Unlock()

// 	e2 := &embedMutex2{}
// 	e2.Lock()
// 	defer e2.Unlock()
// 	e2.mu.Lock()
// 	defer e2.mu.Unlock()
// }

// func TestMutex_9(t *testing.T) {
// 	m := sync.Mutex{}
// 	var x int

// 	go func() {
// 		for i := 0; i < 2; i++ {
// 			if ok := m.TryLock(); ok {
// 				x = i
// 				m.Unlock()
// 			}
// 		}
// 	}()

// 	s := returnMyStruct()
// 	s.mu.Lock()
// 	defer s.mu.Unlock()
// 	x = 3

// 	assert.Equal(t, x, 1)
// }

// type wrapper3 struct {
// 	batchTx
// }

// type batchTx struct {
// 	sync.Mutex
// 	pending int
// }

// func TestMutex_10(t *testing.T) {
// 	w := &wrapper3{}

// 	w.Lock()
// 	w.Unlock()

// }
