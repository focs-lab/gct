package test_context

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestContext_1(t *testing.T) {
	ctx := context.Background()

	<-ctx.Done()

	assert.NotNil(t, ctx)
}

func TestContext_2(t *testing.T) {
	ctx, cancelFunc := context.WithCancel(context.Background())

	go func() {
		time.Sleep(1 * time.Second)
		cancelFunc()
	}()

	<-ctx.Done()

	assert.NotNil(t, ctx)
}

func TestContext_3(t *testing.T) {
	ctx, cancelFunc := context.WithCancelCause(context.Background())

	go func() {
		time.Sleep(1 * time.Second)
		cancelFunc(errors.New("123"))
	}()

	<-ctx.Done()

	println(context.Cause(ctx).Error())
	assert.NotNil(t, ctx)
}

func TestContext_4(t *testing.T) {
	ctx, cancelFunc := context.WithDeadline(context.Background(), time.Now().Add(200 * time.Millisecond))
	defer cancelFunc()

	select {
	case <-time.After(100 * time.Millisecond):
		println("timeout")

	case <-ctx.Done():
		println("It is canceled")
	}

	assert.NotNil(t, ctx)
}

func TestContext_5(t *testing.T) {
	ctx, cancelFunc := context.WithCancel(context.Background())

	ch := make(chan struct{})

	go func() {
		ch <- struct{}{}
	}()

	go func() {
		cancelFunc()
	}()

	select {
	case <-ch:
		println("received from ch")

	case <-ctx.Done():
		println("It is canceled")
	}

	assert.NotNil(t, ctx)
}

func TestContext_6(t *testing.T) {
	ctx_parent, cancelFunc_parent := context.WithCancel(context.Background())

	ctx_child, cancelFunc_child := context.WithCancel(ctx_parent)

	fmt.Printf("Parent = %p, Child = %p \n", ctx_parent, ctx_child)

	cancelFunc_parent()

	cancelFunc_child()
}

func TestContext_7(t *testing.T) {
	ctx, cancelFunc := context.WithCancel(context.Background())
	cancelFunc()

	select {
	case <-time.After(100 * time.Millisecond):
		println("timeout")

	case <-ctx.Done():
		println("It is canceled")
	}

	assert.NotNil(t, ctx)
}

func useContext(ctx context.Context) {
	TakesFuncAsInput(func(ctx context.Context){})
}

type MyStruct struct {
	ctx context.Context
}

func TakesFuncAsInput(hfunc func(ctx context.Context)) {

}
