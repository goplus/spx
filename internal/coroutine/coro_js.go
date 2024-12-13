package coroutine

import (
	"sync"
	stime "time"

	"github.com/goplus/spx/internal/time"
)

var mainMutex sync.Mutex

func (p *Coroutines) Wait(t float64) {
	me := p.Current()
	go func() {
		time.Sleep(t * 1000)
		p.Resume(me)
	}()
	p.Yield(me)
}

func (p *Coroutines) WaitNextFrame() {
	p.Wait(0.016)
}

func (p *Coroutines) WaitMainThread(call func()) {
	done := make(chan bool, 1)
	job := func() {
		call()
		done <- true
	}
	updateJobQueue <- job
	<-done
}

var (
	updateJobQueue = make(chan Job, 1)
)

type Job func()

func (p *Coroutines) UpdateJobs() {
	maxExecTime := stime.Duration(16 * stime.Millisecond)
	startTime := stime.Now()
	timer := stime.NewTimer(maxExecTime)
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
		if stime.Since(startTime) > maxExecTime {
			break
		}
	}
}
