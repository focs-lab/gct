package utils

import (
	"math/rand"
	"reflect"
)

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

func GetRandomElemInSlice[T any](rng *rand.Rand, slice []T) T {
	l := len(slice)
	pos := rng.Intn(l)
	return slice[pos]
}

func GetRandomKeyValInMap[K comparable, V any](rng *rand.Rand, m map[K]V) (K, V) {
	keys := make([]K, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}

	l := len(keys)
	if l == 0 {
		panic("GetRandomKeyInMap: map is empty, so len(map) = 0")
	}
	pos := rng.Intn(l)
	return keys[pos], m[keys[pos]]
}

func GetRandomKeyInMap[K comparable, V any](rng *rand.Rand, m map[K]V) K {
	keys := make([]K, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}

	l := len(keys)
	if l == 0 {
		panic("GetRandomKeyInMap: map is empty, so len(map) = 0")
	}
	pos := rng.Intn(l)
	return keys[pos]
}

func IsInSlice[T comparable](slice []T, element T) (int, bool) {
	for idx, item := range slice {
		if item == element {
			return idx, true
		}
	}
	return -1, false
}

type Tuple[T, U any] struct {
	First  T
	Second U
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
