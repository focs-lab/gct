package utils

import (
	"reflect"
	"unsafe"
)

func CheckSyncRecvChannelEnabled(ch any) bool {
	v := reflect.ValueOf(ch)

	selCase := reflect.SelectCase{
		Dir:  reflect.SelectRecv,
		Chan: v,
	}
	defaultCase := reflect.SelectCase{
		Dir: reflect.SelectDefault,
	}

	chosen, recvValue, ok := reflect.Select([]reflect.SelectCase{selCase, defaultCase})

	if chosen == 0 {// Successfully received from the channel
		if !ok {
			return true 
		}

		go func() {
			v.Send(recvValue)
		}()
		
		return true
	}

	return false
}


func GetChanLen(ch any) int {
	v := reflect.ValueOf(ch)

	if v.Kind() != reflect.Chan {
		panic("GetChanLen: value is not a channel")
	}

	return v.Len()
}

func GetChanCap(ch any) int {
	v := reflect.ValueOf(ch)

	if v.Kind() != reflect.Chan {
		panic("GetChanLen: value is not a channel")
	}

	return v.Cap()
}

// hchan is a mirror of the runtime's hchan struct.
// It's used with unsafe to peek at the closed field of a channel.
// The layout must be kept in sync with the Go runtime's internal representation.
// This layout is valid for Go 1.18+
type hchan struct {
    qcount   uint
    dataqsiz uint
    buf      unsafe.Pointer
    elemsize uint16
    closed   uint32
    elemtype unsafe.Pointer
    sendx    uint
    recvx    uint
    recvq    waitq 
    sendq    waitq 
}

type waitq struct {
	first unsafe.Pointer 
	last  unsafe.Pointer 
}

// Unsafe way of checking if a channel is closed
func IsChanClosed(ch any) bool {
	v := reflect.ValueOf(ch)

	if v.Kind() != reflect.Chan {
		panic("IsChanClosed: value is not a channel")
	}

	// Use unsafe to get a pointer to the channel's data and cast it to our hchan struct.
	// v.Pointer() gives the address of the underlying hchan struct.
	h := (*hchan)(unsafe.Pointer(v.Pointer()))
	return h.closed != 0
}

func CheckSyncChanWaiters(ch any) (bool, bool) {
	v := reflect.ValueOf(ch)
	if v.Kind() != reflect.Chan {
		panic("expecting a channel")
	}

	hchanPtr := (*hchan)(v.UnsafePointer())

	hasRecv := hchanPtr.recvq.first != nil

	hasSend := hchanPtr.sendq.first != nil

	return hasSend, hasRecv
}