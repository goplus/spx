package engine

import (
	"time"

	"github.com/goplus/spx/internal/coroutine"
)

var (
	gco *coroutine.Coroutines
)

func SetCoroutines(co *coroutine.Coroutines) {
	gco = co
}

func Wait(secs float64) {
	gco.Sleep(time.Duration(secs * 1e9))
}

// ========== Engine Coroutines ==========
const (
	maxExecTime = 16 * time.Millisecond
)

var (
	updateJobQueue = make(chan Job, 1)
)

type Job func()

func handleEngineCoroutines() {
	startTime := time.Now()
	timer := time.NewTimer(maxExecTime)
	defer timer.Stop()

	for {
		isTimeout := false
		select {
		case job, ok := <-updateJobQueue:
			if !ok {
				return
			}
			job()
		case <-timer.C:
			isTimeout = true
			break
		}

		if isTimeout {
			break
		}
		if time.Since(startTime) > maxExecTime {
			break
		}
	}
}
