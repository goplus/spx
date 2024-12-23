package engine

import (
	"sync"

	"github.com/goplus/spx/internal/coroutine"
	"github.com/goplus/spx/internal/time"
)

var (
	gco *coroutine.Coroutines
)

func SetCoroutines(co *coroutine.Coroutines) {
	gco = co
}
func Go(tobj coroutine.ThreadObj, fn func()) {
	gco.CreateAndStart(false, tobj, func(me coroutine.Thread) int {
		fn()
		return 0
	})
}
func Wait(secs float64) float64 {
	startTime := time.RealTimeSinceStart()
	gco.Wait(secs)
	return time.RealTimeSinceStart() - startTime
}

func WaitNextFrame() float64 {
	startTime := time.RealTimeSinceStart()
	gco.WaitNextFrame()
	return time.RealTimeSinceStart() - startTime
}

func CreateAndStart(tobj coroutine.ThreadObj, fn func()) {
	gco.CreateAndStart(true, tobj, func(me coroutine.Thread) int {
		fn()
		return 0
	})
}

func CreateCoroAndWait(p coroutine.ThreadObj, call func()) {
	gco.CreateAndWait(p, call)
}

func WaitMainThread(call func()) {
	gco.WaitMainThread(call)
}

func WaitGroup(wg *sync.WaitGroup) {
	gco.WaitGroup(wg)
}
