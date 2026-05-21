package utils

import (
	"math/rand"
	"reflect"
	"sync"
	"unsafe"
)

func ExtractRWMutexFromLocker(m sync.Locker) (*sync.RWMutex, bool) {
	if m == nil {
		return nil, false
	}

	if reflect.TypeOf(m).String() == "*sync.rlocker" {
		return (*sync.RWMutex)(unsafe.Pointer(reflect.ValueOf(m).Pointer())), true
	}

	return nil, false
}

func GetPtrOf(obj any) uintptr {
	if obj == nil {
		return 0
	}
	v := reflect.ValueOf(obj)

	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Pointer, reflect.Slice, reflect.String, reflect.UnsafePointer:
		if v.IsNil() {
			return 0
		} else {
			return v.Pointer()
		}
	default:
		return 0
	}
}

func GetPositionInSlice[T comparable](slice []T, element T) int {
	for index, item := range slice {
		if item == element {
			return index
		}
	}
	return -1
}

func GetRandomPositionInSlice[T any](rng *rand.Rand, slice []T) int {
	l := len(slice)
	pos := rng.Intn(l)
	return pos
}

func IsInSlice[T comparable](slice []T, element T) (int, bool) {
	for idx, item := range slice {
		if item == element {
			return idx, true
		}
	}
	return -1, false
}
