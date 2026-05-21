package passes

import (
	"math/rand"
	set "github.com/deckarep/golang-set/v2"
)

var SharedInstrCtx = NewInstrContext()

type InstrContext struct {
	Metadata        map[string]interface{}
	usedOpids       set.Set[uint64]
	rand  			rand.Rand
}

func NewInstrContext() *InstrContext {
	return &InstrContext{
		Metadata: make(map[string]interface{}),
		usedOpids: set.NewThreadUnsafeSet[uint64](),
		rand: *rand.New(rand.NewSource(0)),
	}
}

func (ctx *InstrContext) GetNewOpid() uint64 {
	for {
		opid := ctx.rand.Uint64()
		if !ctx.usedOpids.Contains(opid) {
			ctx.usedOpids.Add(opid)
			return opid
		}
	}
}

