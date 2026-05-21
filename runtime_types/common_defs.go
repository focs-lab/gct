package runtime_types

type Goid uint64

type Tuple[T, U any] struct {
    First  T
    Second U
}