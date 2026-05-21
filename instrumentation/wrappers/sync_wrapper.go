package wrappers

// import (
// 	"github.com/focs-lab/gct/monitor"
// 	"github.com/focs-lab/gct/sync_shadow"
// 	"context"
// )

// func CreateWaitGroup(wg *sync_shadow.WaitGroup, id uint64) {
// 	wg.Id = id
// 	monitor.AfterWaitGroupCreation(wg, id)
// }

// func CreateMutex(m *sync_shadow.Mutex, id uint64) {
// 	m.Id = id
// 	monitor.AfterMutexCreation(m, id)
// }

// func CreateRWMutex(m *sync_shadow.RWMutex, id uint64) {
// 	m.Id = id
// 	monitor.AfterRWMutexCreation(m, id)
// }

// func CreateContext(parent context.Context, ctxType string, key any, value any) context.Context {
// 	switch ctxType {
// 	case "Background":
// 		ctx := context.Background()
// 		monitor.AfterContextCreation(ctx, ctxType)
// 		return ctx
	
// 	case "TODO":
// 		ctx := context.TODO()
// 		monitor.AfterContextCreation(ctx, ctxType)
// 		return ctx

// 	case "WithValue":
// 		ctx := context.WithValue(parent, key, value)
// 		monitor.AfterContextCreation(ctx, ctxType)
// 		return ctx

// 	case "WithCancel":
// 		ctx, cancel := context.WithCancel(parent)
// 		monitor.AfterContextCreation(ctx, ctxType)
// 		return ctx

// 	case "WithDeadline":
// 		ctx, cancel := context.WithDeadline(parent, value.(context.TimePoint))


// 	default:
// 		panic("CreateContext: unknown ctx type: " + ctxType)
// 	}

// }