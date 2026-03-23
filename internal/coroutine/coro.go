package coroutine

import (
	"errors"
	"runtime"
	sdebug "runtime/debug"
	"sync"
	"sync/atomic"
	stime "time"

	"github.com/goplus/spx/v2/internal/engine/platform"
	"github.com/goplus/spx/v2/internal/log"
	"github.com/goplus/spx/v2/internal/time"
)

// -------------------------------------------------------------------------------------
// Errors
// -------------------------------------------------------------------------------------

var (
	// ErrCannotYieldANonrunningThread represents an "can not yield a non-running thread" error.
	ErrCannotYieldANonrunningThread = errors.New("can not yield a non-running thread")
	ErrAbortThread                  = errors.New("abort thread")
)

// Coroutines represents a coroutine manager.
type Coroutines struct {
	onPanic      func(name, stack string)
	hasInited    bool
	suspended    map[Thread]bool
	current      Thread
	mutex        sync.Mutex
	cond         sync.Cond
	sema         sync.Mutex
	frame        int
	curQueue     *Queue[*WaitJob]
	nextQueue    *Queue[*WaitJob]
	nextJobID    int64
	nextThreadID int64

	waiting   map[Thread]bool
	waitMutex sync.Mutex
	waitCond  sync.Cond
	debug     bool
	perfDebug atomic.Bool

	// goroutineIDs tracks all goroutine IDs created by CreateAndStart
	goroutineIDs sync.Map // map[int64]bool
	allThreads   map[Thread]struct{}
	wg           sync.WaitGroup // tracks active coroutines
}

// readGCStats is a variable so tests can replace runtime/debug collection.
var readGCStats = sdebug.ReadGCStats

const (
	waitStatusAdd = iota
	waitStatusDelete
	waitStatusBlock
	waitStatusIdle
)

const (
	waitTypeFrame = iota
	waitTypeTime
	waitTypeMainThread
	waitTypeYield
)

type WaitJob struct {
	Th    Thread
	Id    int64
	Type  int
	Call  func()
	Time  float64
	Frame int64
}

type updateLoopState struct {
	curQueue       *Queue[*WaitJob]
	nextQueue      *Queue[*WaitJob]
	curFrame       int64
	curTime        float64
	debugStartTime float64
	waitFrameCount int
	waitMainCount  int
}

// -------------------------------------------------------------------------------------
// Constructor
// -------------------------------------------------------------------------------------

func New(onPanic func(name, stack string)) *Coroutines {
	p := &Coroutines{
		onPanic:    onPanic,
		suspended:  make(map[Thread]bool),
		waiting:    make(map[Thread]bool),
		allThreads: make(map[Thread]struct{}),
	}
	p.cond.L = &p.mutex
	p.curQueue = NewQueue[*WaitJob]()
	p.nextQueue = NewQueue[*WaitJob]()
	p.hasInited = false
	p.waitCond.L = &p.waitMutex
	p.debug = false
	return p
}

// -------------------------------------------------------------------------------------
// Lifecycle Management
// -------------------------------------------------------------------------------------

func (p *Coroutines) OnRestart() {
	p.hasInited = false
}

func (p *Coroutines) OnInited() {
	p.hasInited = true
}

// SetPerfDebug enables or disables GC statistics collection during Update.
func (p *Coroutines) SetPerfDebug(enabled bool) {
	p.perfDebug.Store(enabled)
}

// -------------------------------------------------------------------------------------
// Scheduling Control
// -------------------------------------------------------------------------------------

func (p *Coroutines) Sched(me Thread) {
	go func() {
		p.markIdleAndResume(me)
	}()
	p.blockAndYield(me)
}

func (p *Coroutines) Yield(me Thread) {
	if p.Current() != me {
		panic(ErrCannotYieldANonrunningThread)
	}
	p.sema.Unlock()
	// Set atomic suspended flag first
	me.suspended.Store(true)
	// Then update the map for backward compatibility
	p.mutex.Lock()
	p.suspended[me] = true
	p.mutex.Unlock()

	me.mutex.Lock()
	// Abort/cancel can happen before or during wait; in that case
	// break out and let the caller observe ErrAbortThread.
	// Use atomic suspended field to avoid lock-order inversion with AbortAll
	for me.suspended.Load() && !p.isThreadCanceled(me) {
		me.cond.Wait()
	}
	me.mutex.Unlock()

	p.notifyWaiters()
	p.sema.Lock()
	p.setCurrent(me)
	if me.stopped.Load() {
		panic(ErrAbortThread)
	}
}

func (p *Coroutines) Resume(me Thread) {
	for {
		done := false
		p.mutex.Lock()
		if p.suspended[me] {
			p.suspended[me] = false
			done = true
		}
		p.mutex.Unlock()

		if done {
			// Clear atomic suspended flag before signaling
			me.suspended.Store(false)
			me.mutex.Lock()
			me.cond.Signal()
			me.mutex.Unlock()
			return
		}

		if p.isThreadCanceled(me) {
			return
		}
		runtime.Gosched()
	}
}

// -------------------------------------------------------------------------------------
// Wait Operations
// -------------------------------------------------------------------------------------

func (p *Coroutines) Wait(t float64) {
	me := p.Current()
	dstTime := time.TimeSinceLevelLoad() + t

	job := p.newResumeWaitJob(me, waitTypeTime)
	job.Time = dstTime
	p.enqueueAndYield(me, job, false)
}

func (p *Coroutines) WaitNextFrame() {
	me := p.Current()
	frame := time.Frame()

	job := p.newResumeWaitJob(me, waitTypeFrame)
	job.Frame = frame
	p.enqueueAndYield(me, job, false)
}

func (p *Coroutines) WaitMainThread(call func()) {
	if platform.IsWeb() || platform.IsMainThread() {
		call()
		return
	}

	jobID := p.nextWaitJobID()
	done := make(chan struct{}, 1)
	me := p.Current()
	job := &WaitJob{
		Th:   me,
		Id:   jobID,
		Type: waitTypeMainThread,
		Call: func() {
			if p.isThreadCanceled(me) {
				// The waiting select will handle cancellation.
				return
			}
			call()
			done <- struct{}{}
		},
	}
	// Main thread calls are prioritized and queued at the front for immediate execution.
	p.addWaitJob(job, true)

	if me == nil {
		<-done
		return
	}
	select {
	case <-done:
	case <-me.Context().Done():
		panic(ErrAbortThread)
	}
}

func (p *Coroutines) WaitToDo(fn func()) {
	me := p.Current()
	// Delegate fn execution to separate goroutine to prevent blocking the scheduler.
	go func() {
		fn()
		p.markIdleAndResume(me)
	}()
	p.blockAndYield(me)
}

func (p *Coroutines) WaitYield(me Thread) {
	p.enqueueAndYield(me, p.newResumeWaitJob(me, waitTypeYield), false)
}

func WaitForChan[T any](p *Coroutines, ch <-chan T, data *T) {
	me := p.Current()
	if me == nil {
		*data = <-ch
		return
	}
	// Delegate channel receive to separate goroutine to prevent blocking the scheduler.
	go func() {
		select {
		case value := <-ch:
			if p.isThreadCanceled(me) {
				return
			}
			*data = value
			p.markIdleAndResume(me)
		case <-me.Context().Done():
		}
	}()
	p.blockAndYield(me)
}

// -------------------------------------------------------------------------------------
// Update Loop
// -------------------------------------------------------------------------------------

func (p *Coroutines) Update() {
	start := stime.Now()
	var gcStatsBefore *sdebug.GCStats
	if p.perfDebug.Load() {
		stats := &sdebug.GCStats{}
		readGCStats(stats)
		gcStatsBefore = stats
	}

	jobsStats, loopState := p.initializeUpdate()
	p.runMainLoop(&jobsStats, &loopState)
	p.finalizeUpdate(&jobsStats, start, gcStatsBefore)
	lastDebugUpdateStats = jobsStats
}

func (p *Coroutines) initializeUpdate() (UpdateJobsStats, updateLoopState) {
	initStart := stime.Now()
	jobsStats := UpdateJobsStats{}
	loopState := updateLoopState{
		curQueue:       p.curQueue,
		nextQueue:      p.nextQueue,
		curFrame:       time.Frame(),
		curTime:        time.TimeSinceLevelLoad(),
		debugStartTime: time.RealTimeSinceStart(),
	}
	jobsStats.InitTime = elapsedMillis(initStart)
	return jobsStats, loopState
}

func (p *Coroutines) runMainLoop(jobsStats *UpdateJobsStats, loopState *updateLoopState) {
	loopStart := stime.Now()
	loopIterCount := 0
	for {
		loopIterCount++
		if done, shouldContinue := p.waitForWork(loopState, jobsStats); done {
			break
		} else if shouldContinue {
			continue
		}
		p.processSingleTask(loopState, jobsStats)
		if time.RealTimeSinceStart()-loopState.debugStartTime > 1 {
			log.Warn("engine update exceeded 1 second - please review your code (waitMainCount=%d)", loopState.waitMainCount)
			break
		}
	}
	jobsStats.LoopTime = elapsedMillis(loopStart)
	jobsStats.LoopIterations = loopIterCount
	jobsStats.WaitFrameCount = loopState.waitFrameCount
	jobsStats.WaitMainCount = loopState.waitMainCount
	p.moveQueues(loopState, jobsStats)
}

func (p *Coroutines) processSingleTask(state *updateLoopState, stats *UpdateJobsStats) {
	taskStart := stime.Now()
	task := state.curQueue.PopFront()
	stats.TaskCounts++
	p.processWaitTask(state, task)
	stats.TaskProcessing += elapsedMillis(taskStart)
}

func (p *Coroutines) moveQueues(state *updateLoopState, stats *UpdateJobsStats) {
	moveStart := stime.Now()
	stats.NextCount = state.nextQueue.Count()
	state.curQueue.Move(state.nextQueue)
	stats.MoveTime = elapsedMillis(moveStart)
}

func (p *Coroutines) finalizeUpdate(stats *UpdateJobsStats, start stime.Time, gcStatsBefore *sdebug.GCStats) {
	if gcStatsBefore != nil {
		// Get GC statistics after update only when perf debugging is enabled.
		var gcStatsAfter sdebug.GCStats
		readGCStats(&gcStatsAfter)
		stats.GCCount = int(gcStatsAfter.NumGC - gcStatsBefore.NumGC)
		stats.GCPauses = float64(gcStatsAfter.PauseTotal-gcStatsBefore.PauseTotal) / float64(stime.Millisecond)
	}

	// Calculate timing statistics
	totalTime := elapsedMillis(start)
	sumParts := stats.InitTime + stats.LoopTime + stats.MoveTime

	stats.TotalTime = totalTime
	stats.ExternalTime = totalTime - sumParts // External overhead (e.g. Go runtime scheduling)
	stats.TimeDifference = totalTime - sumParts
}

func IsAbortThreadError(err any) bool {
	return err == ErrAbortThread
}

// -------------------------------------------------------------------------------------
// Internal Helpers (unexported)
// -------------------------------------------------------------------------------------

func elapsedMillis(start stime.Time) float64 {
	return stime.Since(start).Seconds() * 1000
}

func (p *Coroutines) nextWaitJobID() int64 {
	return atomic.AddInt64(&p.nextJobID, 1)
}

func (p *Coroutines) markIdleAndResume(me Thread) {
	p.setWaitState(me, waitStatusIdle)
	p.Resume(me)
}

func (p *Coroutines) blockAndYield(me Thread) {
	p.setWaitState(me, waitStatusBlock)
	p.Yield(me)
}

func (p *Coroutines) newResumeWaitJob(me Thread, waitType int) *WaitJob {
	return &WaitJob{
		Th:   me,
		Id:   p.nextWaitJobID(),
		Type: waitType,
		Call: func() {
			p.markIdleAndResume(me)
		},
	}
}

func (p *Coroutines) enqueueAndYield(me Thread, job *WaitJob, isFront bool) {
	p.addWaitJob(job, isFront)
	p.blockAndYield(me)
}

func (p *Coroutines) isThreadCanceled(th Thread) bool {
	if th == nil {
		return false
	}
	if th.stopped.Load() {
		return true
	}
	select {
	case <-th.Context().Done():
		return true
	default:
		return false
	}
}

func (p *Coroutines) addWaitJob(job *WaitJob, isFront bool) {
	p.waitMutex.Lock()
	if isFront {
		p.curQueue.PushFront(job)
	} else {
		p.curQueue.PushBack(job)
	}
	p.waitCond.Signal()
	p.waitMutex.Unlock()
}

func (p *Coroutines) notifyWaiters() {
	p.waitMutex.Lock()
	p.waitCond.Signal()
	p.waitMutex.Unlock()
}

func (p *Coroutines) setWaitState(me *threadImpl, status int) {
	p.waitMutex.Lock()
	switch status {
	case waitStatusDelete:
		delete(p.waiting, me)
	case waitStatusAdd:
		p.waiting[me] = false
	case waitStatusBlock:
		p.waiting[me] = true
	case waitStatusIdle:
		p.waiting[me] = false
	}
	p.waitCond.Signal()
	p.waitMutex.Unlock()
}

func (p *Coroutines) activeThreadCount() int {
	activeCount := 0
	for th, val := range p.waiting {
		if p.isThreadCanceled(th) {
			continue
		}
		if !val {
			activeCount++
		}
	}
	return activeCount
}

func (p *Coroutines) waitForWork(state *updateLoopState, stats *UpdateJobsStats) (done, shouldContinue bool) {
	if !p.hasInited {
		if state.curQueue.Count() == 0 {
			waitStart := stime.Now()
			time.Sleep(0.05)
			stats.WaitTime += elapsedMillis(waitStart)
			return false, true
		}
		return false, false
	}

	waitStart := stime.Now()
	p.waitMutex.Lock()
	if state.curQueue.Count() == 0 {
		if p.activeThreadCount() == 0 {
			done = true
		} else {
			p.waitCond.Wait()
			shouldContinue = true
		}
	}
	p.waitMutex.Unlock()
	stats.WaitTime += elapsedMillis(waitStart)
	return done, shouldContinue
}

func (p *Coroutines) processWaitTask(state *updateLoopState, task *WaitJob) {
	switch task.Type {
	case waitTypeFrame:
		if task.Frame >= state.curFrame {
			state.nextQueue.PushBack(task)
		} else {
			task.Call()
			state.waitFrameCount++
		}
	case waitTypeTime:
		if task.Time >= state.curTime {
			state.nextQueue.PushBack(task)
		} else {
			task.Call()
		}
	case waitTypeYield:
		task.Call()
	case waitTypeMainThread:
		task.Call()
		state.waitMainCount++
	}
}
