package condvar

// import (
// 	"testing"
// 	"sync"
// )

// type TestCondVar struct {
// 	c *sync.Cond
// }

// func NewTestCondVar() *TestCondVar {
// 	var mu sync.Mutex
// 	return &TestCondVar{
// 		c: &sync.Cond{
// 			L: &mu,
// 		},
// 	}
// }

// func TestCondVar_1(t *testing.T) {
// 	var mu sync.Mutex
// 	c := sync.NewCond(&mu)

// 	go func() {
// 		c.L.Lock()
// 		c.Wait()
// 		c.L.Unlock()
// 	}()

// 	c.L.Lock()
// 	c.Signal()
// 	c.L.Unlock()
// }

// func TestCondVar_2(t *testing.T) {
// 	var mu sync.Mutex
// 	c := sync.NewCond(&mu)

// 	go func() {
// 		c.L.Lock()
// 		c.Wait()
// 		c.L.Unlock()
// 	}()

// 	go func() {
// 		c.L.Lock()
// 		c.Wait()
// 		c.L.Unlock()
// 	}()

// 	c.L.Lock()
// 	c.Signal()
// 	c.L.Unlock()
// }

// func TestCondVar_3(t *testing.T) {
// 	var mu sync.Mutex
// 	c := sync.NewCond(&mu)

// 	go func() {
// 		c.L.Lock()
// 		c.Wait()
// 		c.L.Unlock()
// 	}()

// 	c.L.Lock()
// 	c.Signal()
// 	c.L.Unlock()
// }

// func TestCondVar_4(t *testing.T) {
// 	var mu sync.Mutex
// 	c := sync.NewCond(&mu)

// 	go func() {
// 		c.L.Lock()
// 		c.Wait()
// 		c.L.Unlock()
// 	}()

// 	c.L.Lock()
// 	c.Broadcast()
// 	c.L.Unlock()
// }

// func TestCondVar_5(t *testing.T) {
// 	var mu sync.Mutex
// 	c := sync.NewCond(&mu)

// 	go func() {
// 		c.L.Lock()
// 		c.Wait()
// 		c.L.Unlock()
// 	}()

// 	go func() {
// 		c.L.Lock()
// 		c.Wait()
// 		c.L.Unlock()
// 	}()

// 	go func() {
// 		c.L.Lock()
// 		c.Wait()
// 		c.L.Unlock()
// 	}()

// 	c.L.Lock()
// 	c.Broadcast()
// 	c.L.Unlock()
// }
