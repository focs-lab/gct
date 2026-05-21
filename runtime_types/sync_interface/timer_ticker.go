package sync_interface

import (
	"time"
)

type Timer interface {
	GetC() <-chan time.Time
	Stop() bool
	Reset(d time.Duration) bool
}

type Ticker interface {
	GetC() <-chan time.Time
	Stop() 
	Reset(d time.Duration) 
}