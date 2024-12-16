package coroutine

import (
	"fmt"
	"runtime"
	"sync/atomic"

	"github.com/goplus/spx/internal/time"
)

func (p *Coroutines) Wait(t float64) {
	id := atomic.AddInt64(&p.curJobId, 1)
	me := p.Current()
	job := &WaitJob{
		Id:     id,
		Thread: me,
		Type:   waitTypeTime,
		Call: func() {
			go p.Resume(me)
		},
		Time: time.TimeSinceLevelLoad() + t,
	}
	p.addWaitJob(job, false)
	p.Yield(me)
}

func (p *Coroutines) WaitNextFrame() {
	id := atomic.AddInt64(&p.curJobId, 1)
	me := p.Current()
	job := &WaitJob{
		Id:     id,
		Type:   waitTypeFrame,
		Thread: me,
		Call: func() {
			go p.Resume(me)
		},
		Frame: time.Frame(),
	}
	p.addWaitJob(job, false)
	p.Yield(me)
}

func (p *Coroutines) WaitMainThread(call func()) {
	me := p.Current()
	id := atomic.AddInt64(&p.curJobId, 1)
	job := &WaitJob{
		Id:     id,
		Thread: me,
		Type:   waitTypeMainThread,
		Call: func() {
			call()
			go p.Resume(me)
		},
	}
	// main thread call's priority is higher than other wait jobs
	p.addWaitJob(job, true)
	p.Yield(me)
}

func (p *Coroutines) CreateAndWait(tobj ThreadObj, call func()) {
	th := p.CreateAndStart(true, tobj, func(th Thread) int {
		call()
		return 0
	})
	calledTaskes := make([]*WaitJob, 1)
	for !th.Stopped() {
		calledTaskes = p.doJobs(calledTaskes)
		runtime.Gosched()
		time.Sleep(0.1)
	}
}

func (p *Coroutines) UpdateJobs() {
	timestamp := time.RealTimeSinceStart()
	isTimeout := false
	calledTaskes := make([]*WaitJob, 1)
	for {
		// handle tasks
		calledTaskes = p.doJobs(calledTaskes)
		// check break condition
		isTimeout = time.RealTimeSinceStart()-timestamp > 0.1 // max wait 100 ms
		allDone := (p.nextQueue.count >= p.thCount)
		if isTimeout || allDone {
			break
		}
		runtime.Gosched()
		time.Sleep(0.1)
	}
	moveCount := p.nextQueue.Count()
	p.curQueue.Move(p.nextQueue)
	delta := (time.RealTimeSinceStart() - timestamp) * 1000
	if isTimeout || p.debug {
		msg := fmt.Sprintf("TimeOut realTime %f curFrame %d, timestamp %f, fps %d, delta %fms,  moveCount %d thCount %d coroNum %d",
			time.RealTimeSinceStart(), time.Frame(), timestamp, int(time.FPS()), delta, moveCount, p.thCount, runtime.NumGoroutine())
		p.dumpTasks(isTimeout, msg, calledTaskes)
	}
}

func (p *Coroutines) doJobs(calledTaskes []*WaitJob) []*WaitJob {
	curFrame := time.Frame()
	curTime := time.TimeSinceLevelLoad()
	curQueue := p.curQueue
	nextQueue := p.nextQueue

	for curQueue.Count() > 0 {
		task := curQueue.PopFront()
		if task.Thread != nil {
			p.debugLog("handle task", task.Id, nextQueue.Count(), task.Thread.stackSimple_)
		}
		switch task.Type {
		case waitTypeFrame:
			calledTaskes = append(calledTaskes, task)
			if task.Frame >= curFrame {
				nextQueue.PushBack(task)
			} else {
				task.Call()
			}
		case waitTypeTime:
			calledTaskes = append(calledTaskes, task)
			if task.Time >= curTime {
				nextQueue.PushBack(task)
			} else {
				task.Call()
			}
		case waitTypeMainThread:
			task.Call()
		}
	}
	return calledTaskes
}

func (p *Coroutines) addWaitJob(job *WaitJob, isFront bool) {
	if isFront {
		p.curQueue.PushFront(job)
	} else {
		p.curQueue.PushBack(job)
	}
}
