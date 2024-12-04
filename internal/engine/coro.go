package engine

import (
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
	startTime := time.TimeSinceLevelLoad()
	gco.Wait(secs)
	return time.TimeSinceLevelLoad() - startTime
}

func WaitNextFrame() float64 {
	gco.WaitNextFrame()
	return time.DeltaTime()
}

func WaitMainThread(call func()) {
	gco.WaitMainThread(call)
}

func WaitToDo(fn func()) {
	gco.WaitToDo(fn)
}

func WaitForChan[T any](done chan T, data *T) {
	coroutine.WaitForChan(gco, done, data)
}
