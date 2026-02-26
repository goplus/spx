package coroutine

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	sdebug "runtime/debug"
	"sync"
	"sync/atomic"
	stime "time"
	"unsafe"

	"github.com/goplus/spx/v2/internal/debug"
	"github.com/goplus/spx/v2/internal/engine/platform"
	"github.com/goplus/spx/v2/internal/time"
	"github.com/petermattis/goid"
)

var (
	// ErrCannotYieldANonrunningThread represents an "can not yield a non-running thread" error.
	ErrCannotYieldANonrunningThread = errors.New("can not yield a non-running thread")
	ErrAbortThread                  = errors.New("abort thread")
)

// -------------------------------------------------------------------------------------

type ThreadObj any

type threadImpl struct {
	Obj     ThreadObj
	stopped bool
	frame   int
	mutex   sync.Mutex // Mutex for this thread's condition variable
	cond    *sync.Cond // Per-thread condition variable for targeted wake-up
	id      int64
	name    string
	stack   string

	schedFrame     int64
	schedTimestamp stime.Time

	ctx        context.Context
	cancelFunc context.CancelFunc
}

// Context returns the context associated with this thread.
// If no context was set, returns context.Background().
func (p *threadImpl) Context() context.Context {
	if p.ctx == nil {
		return context.Background()
	}
	return p.ctx
}

// Cancel cancels the thread's context, signaling graceful termination.
// Safe to call multiple times or on threads without contexts.
func (p *threadImpl) Cancel() {
	if p.cancelFunc != nil {
		p.cancelFunc()
	}
}

func (p *threadImpl) String() string {
	return fmt.Sprintf("id=%d name=%s ", p.id, p.name)
}

func (p *threadImpl) Name() string {
	return p.name
}

func (p *threadImpl) Stack() string {
	return p.stack
}

func (p *threadImpl) Stopped() bool {
	return p.stopped
}

// Thread represents a coroutine id.
type Thread = *threadImpl

func (p Thread) IsSchedTimeout(ms float64) bool {
	if p.schedFrame < time.Frame() {
		p.schedFrame = time.Frame()
		p.schedTimestamp = stime.Now()
	}
	timeout := stime.Since(p.schedTimestamp) > stime.Duration(ms)*stime.Millisecond
	return timeout
}

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

	// goroutineIDs tracks all goroutine IDs created by CreateAndStart
	goroutineIDs sync.Map // map[int64]bool

	allThreads map[Thread]struct{}
}

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

// New creates a coroutine manager.
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

func (p *Coroutines) Sched(me Thread) {
	go func() {
		p.setWaitState(me, waitStatusIdle)
		p.Resume(me)
	}()
	// Mark the thread as blocked and yield control
	p.setWaitState(me, waitStatusBlock)
	p.Yield(me)
}

func (p *Coroutines) OnRestart() {
	p.hasInited = false
}

func (p *Coroutines) OnInited() {
	p.hasInited = true
}

// Create creates a new coroutine.
func (p *Coroutines) Create(obj ThreadObj, fn func(me Thread) int) Thread {
	return p.CreateAndStart(false, obj, fn)
}

func (p *Coroutines) Current() Thread {
	return Thread(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&p.current))))
}

func (p *Coroutines) Abort() {
	panic(ErrAbortThread)
}

func (p *Coroutines) AbortAll() {
	p.mutex.Lock()
	threads := make([]Thread, 0, len(p.allThreads))
	for th := range p.allThreads {
		threads = append(threads, th)
	}
	p.mutex.Unlock()

	for _, th := range threads {
		th.mutex.Lock()
		if !th.stopped {
			th.stopped = true
			th.Cancel()
			th.cond.Signal()
		}
		th.mutex.Unlock()
	}
}

func (p *Coroutines) StopIf(filter func(th Thread) bool) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	for th := range p.suspended {
		if filter(th) {
			th.stopped = true
			th.Cancel()
		}
	}
}

// CreateAndStart creates and executes the new coroutine.
func (p *Coroutines) CreateAndStart(start bool, obj ThreadObj, fn func(me Thread) int) Thread {
	th := p.newThread(obj)
	p.registerThread(th)

	go func() {
		p.runThread(th, fn)
	}()
	if start {
		runtime.Gosched()
	}
	return th
}

func (p *Coroutines) WaitYield(me Thread) {
	p.enqueueAndYield(me, p.newResumeWaitJob(me, waitTypeYield), false)
}

// Yield suspends a running coroutine.
func (p *Coroutines) Yield(me Thread) {
	if p.Current() != me {
		panic(ErrCannotYieldANonrunningThread)
	}
	p.sema.Unlock()
	p.mutex.Lock()
	p.suspended[me] = true
	p.mutex.Unlock()

	me.mutex.Lock()
	for p.isSuspended(me) {
		me.cond.Wait()
	}
	me.mutex.Unlock()

	p.notifyWaiters()

	p.sema.Lock()

	p.setCurrent(me)
	if me.stopped {
		panic(ErrAbortThread)
	}
}

// Resume resumes a suspended coroutine.
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
			me.mutex.Lock()
			me.cond.Signal()
			me.mutex.Unlock()
			return
		}
		runtime.Gosched()
	}
}

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
	if platform.IsWeb() {
		call()
		return
	}
	jobID := p.nextWaitJobID()
	done := make(chan int)
	job := &WaitJob{
		Id:   jobID,
		Type: waitTypeMainThread,
		Call: func() {
			call()
			done <- 1
		},
	}
	// main thread call's priority is higher than other wait jobs
	p.addWaitJob(job, true)
	<-done
}

func (p *Coroutines) WaitToDo(fn func()) {
	me := p.Current()
	// This goroutine is necessary since fn() could be a long-running task
	go func() {
		fn()
		p.markIdleAndResume(me)
	}()

	// Mark the thread as blocked and yield control
	p.blockAndYield(me)
}

func WaitForChan[T any](p *Coroutines, ch chan T, data *T) {
	me := p.Current()
	// This goroutine is necessary since channel receive could be long-running.
	go func() {
		*data = <-ch
		p.markIdleAndResume(me)
	}()
	// Mark the thread as blocked and yield control
	p.blockAndYield(me)
}

func (p *Coroutines) Update() {
	// Total timing starts
	start := stime.Now()

	// Record GC information
	var gcStatsBefore sdebug.GCStats
	sdebug.ReadGCStats(&gcStatsBefore)

	// Initialize statistics
	stats := UpdateJobsStats{}

	// Initialization phase starts
	initStart := stime.Now()
	state := updateLoopState{
		curQueue:       p.curQueue,
		nextQueue:      p.nextQueue,
		curFrame:       time.Frame(),
		curTime:        time.TimeSinceLevelLoad(),
		debugStartTime: time.RealTimeSinceStart(),
	}
	// Initialization phase ends
	stats.InitTime = stime.Since(initStart).Seconds() * 1000

	// Main loop starts
	loopStart := stime.Now()
	// Loop iteration counter
	loopIterCount := 0
	for {
		loopIterCount++

		done, shouldContinue := p.waitForWork(&state, &stats)
		if done {
			break
		}
		if shouldContinue {
			continue
		}

		// Task processing starts
		taskStart := stime.Now()
		task := state.curQueue.PopFront()
		stats.TaskCounts++
		p.processWaitTask(&state, task)
		stats.TaskProcessing += stime.Since(taskStart).Seconds() * 1000

		if time.RealTimeSinceStart()-state.debugStartTime > 1 {
			println("Warning: engine update > 1 seconds, please check your code ! waitMainCount=", state.waitMainCount)
			break
		}
	}
	// Main loop ends
	stats.LoopTime = stime.Since(loopStart).Seconds() * 1000

	// Queue move starts
	moveStart := stime.Now()
	stats.NextCount = state.nextQueue.Count()
	state.curQueue.Move(state.nextQueue)
	stats.MoveTime = stime.Since(moveStart).Seconds() * 1000

	// Update statistics
	stats.WaitFrameCount = state.waitFrameCount
	stats.WaitMainCount = state.waitMainCount

	// Get GC statistics
	var gcStatsAfter sdebug.GCStats
	sdebug.ReadGCStats(&gcStatsAfter)
	stats.GCCount = int(gcStatsAfter.NumGC - gcStatsBefore.NumGC)
	stats.GCPauses = float64(gcStatsAfter.PauseTotal-gcStatsBefore.PauseTotal) / float64(stime.Millisecond)

	// Calculate total time
	delta := stime.Since(start).Seconds() * 1000

	// Calculate the difference between the measured total time and the sum of individual times
	measuredTotal := delta
	sumParts := stats.InitTime + stats.LoopTime + stats.MoveTime
	timeDiff := measuredTotal - sumParts

	// Calculate the external time (may include Go runtime scheduling overhead)
	externalTime := delta - sumParts

	// Update statistics
	stats.ExternalTime = externalTime
	stats.LoopIterations = loopIterCount
	stats.TotalTime = delta
	stats.TimeDifference = timeDiff

	// Save statistics for external access
	lastDebugUpdateStats = stats

}

// IsInCoroutine checks if the current execution environment is within
// a coroutine created by (*Coroutines) CreateAndStart.
func (p *Coroutines) IsInCoroutine() bool {
	currentGID := goid.Get()
	_, exists := p.goroutineIDs.Load(currentGID)
	return exists
}

func IsAbortThreadError(err any) bool {
	return err == ErrAbortThread
}

// -----------------------------------------------------------------------------
// Internal Helpers (unexported)
// -----------------------------------------------------------------------------

// setCurrent updates the current running coroutine pointer atomically.
func (p *Coroutines) setCurrent(id Thread) {
	atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&p.current)), unsafe.Pointer(id))
}

func resolveThreadName(obj ThreadObj) string {
	if obj == nil {
		return ""
	}

	t := reflect.TypeOf(obj)
	if t.Kind() != reflect.Pointer || t.Elem().Name() == "" {
		return ""
	}

	name := "*" + t.Elem().Name()
	v := reflect.ValueOf(obj)
	nameMethod := v.MethodByName("Name")
	if !nameMethod.IsValid() {
		return name
	}

	results := nameMethod.Call(nil)
	if len(results) > 0 {
		name = results[0].String()
	}
	return name
}

// newThread allocates thread state and initializes its context/metadata.
func (p *Coroutines) newThread(obj ThreadObj) Thread {
	th := &threadImpl{
		Obj:        obj,
		frame:      p.frame,
		id:         atomic.AddInt64(&p.nextThreadID, 1),
		schedFrame: -1,
		name:       resolveThreadName(obj),
	}
	th.ctx, th.cancelFunc = context.WithCancel(context.Background())
	if p.debug {
		th.stack = debug.GetStackTrace()
	}
	th.cond = sync.NewCond(&th.mutex)
	return th
}

func (p *Coroutines) registerThread(th Thread) {
	p.mutex.Lock()
	p.allThreads[th] = struct{}{}
	p.mutex.Unlock()
}

func (p *Coroutines) unregisterThread(th Thread) {
	p.mutex.Lock()
	delete(p.suspended, th)
	delete(p.allThreads, th)
	p.mutex.Unlock()
}

func (p *Coroutines) handleThreadPanic(th Thread, recovered any) {
	if recovered == nil || recovered == ErrAbortThread {
		return
	}
	if p.onPanic != nil {
		p.onPanic(th.name, th.stack)
		return
	}
	panic(recovered)
}

// runThread executes a coroutine function with lifecycle/panic handling.
func (p *Coroutines) runThread(th Thread, fn func(me Thread) int) {
	gid := goid.Get()
	p.goroutineIDs.Store(gid, true)

	p.sema.Lock()
	p.setCurrent(th)
	defer func() {
		recovered := recover()
		p.unregisterThread(th)
		p.setWaitState(th, waitStatusDelete)
		th.Cancel()
		p.sema.Unlock()
		p.goroutineIDs.Delete(gid)
		p.handleThreadPanic(th, recovered)
	}()

	p.setWaitState(th, waitStatusAdd)
	fn(th)
}

// nextWaitJobID returns a monotonically increasing wait-job identifier.
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

// newResumeWaitJob creates a wait job that resumes the target thread when fired.
func (p *Coroutines) newResumeWaitJob(me Thread, waitType int) *WaitJob {
	return &WaitJob{
		Id:   p.nextWaitJobID(),
		Type: waitType,
		Call: func() {
			p.markIdleAndResume(me)
		},
		Th: me,
	}
}

func (p *Coroutines) enqueueAndYield(me Thread, job *WaitJob, isFront bool) {
	p.addWaitJob(job, isFront)
	p.blockAndYield(me)
}

func (p *Coroutines) isSuspended(me Thread) bool {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.suspended[me]
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

// setWaitState tracks whether a thread is running, blocked, or removed.
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
	for _, val := range p.waiting {
		if !val {
			activeCount++
		}
	}
	return activeCount
}

// waitForWork blocks until tasks are available or all active coroutines finish.
func (p *Coroutines) waitForWork(state *updateLoopState, stats *UpdateJobsStats) (done, shouldContinue bool) {
	if !p.hasInited {
		if state.curQueue.Count() == 0 {
			waitStart := stime.Now()
			time.Sleep(0.05)
			stats.WaitTime += stime.Since(waitStart).Seconds() * 1000
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
	stats.WaitTime += stime.Since(waitStart).Seconds() * 1000
	return done, shouldContinue
}

// processWaitTask executes or defers a wait job based on its type and readiness.
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
