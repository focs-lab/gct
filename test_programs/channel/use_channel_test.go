package channel

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
)

type test struct {
	c chan int
}

func Test_three_channels(t *testing.T) {
	ch := make(chan int, 3)

	c1 := &test{
		c: make(chan int),
	}

	c2 := &test{
		c: make(chan int, 3),
	}

	c3 := test{
		c: make(chan int),
	}

	c4 := &test{
		c: make(chan int, 3),
	}

	_ = c1
	_ = c2
	_ = c3
	_ = c4

	go func() {
		ch <- 1
	}()

	go func() {
		ch <- 2
	}()

	go func() {
		ch <- 3
	}()
	x := <-ch
	x = <-ch
	x = <-ch

	assert.Equal(t, 1, x)
}

func Test_directional_channels(t *testing.T) {
	ch := make(chan struct{}, 1)

	// Store using bidirectional channel
	channels := make(map[any]string)
	channels[ch] = "tracked"

	// Convert to receive-only channel
	var recvOnly <-chan struct{} = ch

	// Attempt lookup
	v, ok := channels[recvOnly]

	fmt.Println("Lookup success:", ok)
	fmt.Println("Value:", v)

	// Show why
	var a interface{} = ch
	var b interface{} = recvOnly

	fmt.Println("a == b:", a == b)
	fmt.Println("Type of a:", reflect.TypeOf(a))
	fmt.Println("Type of b:", reflect.TypeOf(b))

	fmt.Println("Underlying pointer a:",
		reflect.ValueOf(a).Pointer())
	fmt.Println("Underlying pointer b:",
		reflect.ValueOf(b).Pointer())
}

type Breaker struct {
	pendingRequests chan int
	activeRequests  chan int
}

func NewBreaker(c1, c2 int32) *Breaker {
	return &Breaker{
		pendingRequests: make(chan int, c1),
		activeRequests:  make(chan int, c2),
	}
}

func retChannel(ch chan int) int {
	return <-ch
}

func Test_Channel_2(t *testing.T) {
	ch := make(chan int, 2)

	_ = retChannel(ch)
}

func Test_three_channels_async(t *testing.T) {
	ch := make(chan int, 3)

	go func() {
		ch <- 1
	}()

	go func() {
		ch <- 2
	}()

	go func() {
		ch <- 3
	}()

	go func() {
		<-ch
	}()

	go func() {
		<-ch
	}()

	go func() {
		<-ch
	}()
}
