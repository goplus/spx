package engine

import (
	"github.com/goplus/spx/v2/internal/coroutine"
	"github.com/goplus/spx/v2/internal/engine/profiler"
	"github.com/goplus/spx/v2/internal/time"
)

var (
	gco   *coroutine.Coroutines
	pgame any // pointer to the current game object
)

func SetGame(game any) {
	pgame = game
}

func GetGame() any {
	return pgame
}

func IsSpxEnv() bool {
	return GetGame() != nil
}

func SetCoroutines(co *coroutine.Coroutines) {
	gco = co
	profiler.SetGco(co)
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

func WaitYield() {
	gco.WaitYield(gco.Current())
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
