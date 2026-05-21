package monitor

import "golang.org/x/exp/constraints"

func ChannelSend[T any](ch chan<- T, val T, isSelect, isTimer bool, id uint64) {
	BeforeChannelSend(ch, isSelect, isTimer, id)
	ch <- val
	AfterChannelSend(ch, id)
}

func ChannelRecv[T any](ch <-chan T, isSelect, isTimer bool, id uint64) (T, bool) {
	BeforeChannelReceive(ch, isSelect, isTimer, id)
	v, ok := <-ch
	AfterChannelReceive(ch, isSelect, isTimer, id)
	return v, ok
}

func ChannelClose[T any](ch chan<- T, id uint64) {
	BeforeChannelClose(ch, id)
	close(ch)
	AfterChannelClose(ch, id)
}

// Some programs uses int32 for channel capacity,
// it is fine to call make(chan int, int32), but not for ChannelCreate
func ChannelCreate[T any, C constraints.Integer](cap C, id uint64) chan T {
	c := int(cap)

	if int64(c) != int64(cap) {
		panic("ChannelCreate: capacity overflow")
	}

	var ch chan T
	if c == 0 {
		ch = make(chan T)
	} else {
		ch = make(chan T, c)
	}

	AfterChannelCreation(ch, c, id)
	return ch
}
