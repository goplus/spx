package coroutine

import (
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
)

var (
	// ErrCannotYieldANonrunningThread represents an "can not yield a non-running thread" error.
	ErrCannotYieldANonrunningThread = errors.New("can not yield a non-running thread")
	ErrAbortThread                  = errors.New("abort thread")
)

// -------------------------------------------------------------------------------------

type ThreadObj any

type threadImpl struct {
	Obj      ThreadObj
	stopped_ bool
	frame    int
	mutex    sync.Mutex // Mutex for this thread's condition variable
	cond     *sync.Cond // Per-thread condition variable for targeted wake-up
	id       int64
	name     string
	stack    string

	schedFrame     int64
	schedTimestamp stime.Time
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
	return p.stopped_
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
	onPanic   func(name, stack string)
	hasInited bool
	suspended map[Thread]bool
	current   Thread
	mutex     sync.Mutex
	cond      sync.Cond
	sema      sync.Mutex
	frame     int
	curQueue  *Queue[*WaitJob]
	nextQueue *Queue[*WaitJob]
	curId     int64
	curThId   int64

	waiting   map[Thread]bool
	waitMutex sync.Mutex
	waitCond  sync.Cond
	debug     bool
}

const (
	waitStatusAdd = iota
	waitStatusDelete
	waitStatusBlock
	waitStatusIdle
	waitNotify
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

// New creates a coroutine manager.
func New(onPanic func(name, stack string)) *Coroutines {
	p := &Coroutines{
		onPanic:   onPanic,
		suspended: make(map[Thread]bool),
		waiting:   make(map[Thread]bool),
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
		p.setWaitStatus(me, waitStatusIdle)
		p.Resume(me)
	}()
	// Mark the thread as blocked and yield control
	p.setWaitStatus(me, waitStatusBlock)
	p.Yield(me)
}
func (p *Coroutines) OnRestart() {
	p.hasInited = false
}
func (p *Coroutines) OnInited() {
	p.hasInited = true
}

// Create creates a new coroutine.
func (p *Coroutines) Create(tobj ThreadObj, fn func(me Thread) int) Thread {
	return p.CreateAndStart(false, tobj, fn)
}

func (p *Coroutines) setCurrent(id Thread) {
	atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&p.current)), unsafe.Pointer(id))
}

func (p *Coroutines) Current() Thread {
	return Thread(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&p.current))))
}

func (p *Coroutines) Abort() {
	panic(ErrAbortThread)
}

func (p *Coroutines) StopIf(filter func(th Thread) bool) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	for th := range p.suspended {
		if filter(th) {
			th.stopped_ = true
		}
	}
}

// CreateAndStart creates and executes the new coroutine.
func (p *Coroutines) CreateAndStart(start bool, tobj ThreadObj, fn func(me Thread) int) Thread {
	id := &threadImpl{Obj: tobj, frame: p.frame, id: atomic.AddInt64(&p.curThId, 1), schedFrame: -1}

	name := ""
	if tobj != nil {
		t := reflect.TypeOf(tobj)
		if t.Kind() == reflect.Ptr && t.Elem().Name() != "" {
			name = "*" + t.Elem().Name()
			v := reflect.ValueOf(tobj)
			nameMethod := v.MethodByName("Name")
			if nameMethod.IsValid() {
				results := nameMethod.Call(nil)
				name = results[0].String()
			}
		}
	}
	id.name = name

	if p.debug {
		id.stack = debug.GetStackTrace()
	}

	id.cond = sync.NewCond(&id.mutex) // Initialize the thread's condition variable
	go func() {
		p.sema.Lock()
		p.setCurrent(id)
		defer func() {
			p.mutex.Lock()
			delete(p.suspended, id)
			p.mutex.Unlock()
			p.setWaitStatus(id, waitStatusDelete)
			p.sema.Unlock()
			if e := recover(); e != nil {
				if e != ErrAbortThread {
					if p.onPanic != nil {
						p.onPanic(id.name, id.stack)
					}
					panic(e)
				}
			}
		}()
		p.setWaitStatus(id, waitStatusAdd)
		fn(id)
	}()
	if start {
		runtime.Gosched()
	}
	return id
}

func (p *Coroutines) WaitYield(me Thread) {
	job := &WaitJob{
		Id:   atomic.AddInt64(&p.curId, 1),
		Type: waitTypeYield,
		Call: func() {
			p.setWaitStatus(me, waitStatusIdle)
			p.Resume(me)
		},
		Th: me,
	}

	p.addWaitJob(job, false)
	p.setWaitStatus(me, waitStatusBlock)
	p.Yield(me)
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

	p.waitNotify()

	p.sema.Lock()

	p.setCurrent(me)
	if me.stopped_ {
		panic(ErrAbortThread)
	}
}

func (p *Coroutines) isSuspended(me Thread) bool {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.suspended[me]
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

func (p *Coroutines) waitNotify() {
	p.waitMutex.Lock()
	p.waitCond.Signal()
	p.waitMutex.Unlock()
}

func (p *Coroutines) setWaitStatus(me *threadImpl, typeId int) {
	p.waitMutex.Lock()
	switch typeId {
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

func (p *Coroutines) Wait(t float64) {
	me := p.Current()
	dstTime := time.TimeSinceLevelLoad() + t

	job := &WaitJob{
		Id:   atomic.AddInt64(&p.curId, 1),
		Type: waitTypeTime,
		Call: func() {
			p.setWaitStatus(me, waitStatusIdle)
			p.Resume(me)
		},
		Time: dstTime,
		Th:   me,
	}

	p.addWaitJob(job, false)

	p.setWaitStatus(me, waitStatusBlock)
	p.Yield(me)
}

func (p *Coroutines) WaitNextFrame() {
	me := p.Current()
	frame := time.Frame()

	job := &WaitJob{
		Id:   atomic.AddInt64(&p.curId, 1),
		Type: waitTypeFrame,
		Call: func() {
			p.setWaitStatus(me, waitStatusIdle)
			p.Resume(me)
		},
		Th:    me,
		Frame: frame,
	}

	p.addWaitJob(job, false)

	p.setWaitStatus(me, waitStatusBlock)
	p.Yield(me)
}

func (p *Coroutines) WaitMainThread(call func()) {
	if platform.IsWeb() {
		call()
		return
	}
	id := atomic.AddInt64(&p.curId, 1)
	done := make(chan int)
	job := &WaitJob{
		Id:   id,
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
		p.setWaitStatus(me, waitStatusIdle)
		p.Resume(me)
	}()

	// Mark the thread as blocked and yield control
	p.setWaitStatus(me, waitStatusBlock)
	p.Yield(me)
}

func WaitForChan[T any](p *Coroutines, done chan T, data *T) {
	me := p.Current()
	// This goroutine is necessary since <-done could be a long-running task
	go func() {
		*data = <-done
		p.setWaitStatus(me, waitStatusIdle)
		p.Resume(me)
	}()
	// Mark the thread as blocked and yield control
	p.setWaitStatus(me, waitStatusBlock)
	p.Yield(me)
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
	curQueue := p.curQueue
	nextQueue := p.nextQueue
	curFrame := time.Frame()
	curTime := time.TimeSinceLevelLoad()
	debugStartTime := time.RealTimeSinceStart()
	waitFrameCount := 0
	waitMainCount := 0
	// Initialization phase ends
	stats.InitTime = stime.Since(initStart).Seconds() * 1000
	// Main loop starts
	loopStart := stime.Now()
	// Loop iteration counter
	loopIterCount := 0
	for {
		// Record the start time of each loop iteration
		_ = stime.Now()
		loopIterCount++

		if !p.hasInited {
			if curQueue.Count() == 0 {
				waitStart := stime.Now()
				time.Sleep(0.05)
				stats.WaitTime += stime.Since(waitStart).Seconds() * 1000
				continue
			}
		} else {
			done := false
			isContinue := false

			waitStart := stime.Now()
			p.waitMutex.Lock()
			if curQueue.Count() == 0 {
				activeCount := 0
				for _, val := range p.waiting {
					if !val {
						activeCount++
					}
				}
				if activeCount == 0 {
					done = true
				} else {
					p.waitCond.Wait()
					isContinue = true
				}
			}
			p.waitMutex.Unlock()
			stats.WaitTime += stime.Since(waitStart).Seconds() * 1000

			if done {
				break
			}
			if isContinue {
				continue
			}
		}

		// Task processing starts
		taskStart := stime.Now()
		task := curQueue.PopFront()
		stats.TaskCounts++

		switch task.Type {
		case waitTypeFrame:
			if task.Frame >= curFrame {
				nextQueue.PushBack(task)
			} else {
				task.Call()
				waitFrameCount++
			}
		case waitTypeTime:
			if task.Time >= curTime {
				nextQueue.PushBack(task)
			} else {
				task.Call()
			}
		case waitTypeYield:
			task.Call()
		case waitTypeMainThread:
			task.Call()
			waitMainCount++
		}
		stats.TaskProcessing += stime.Since(taskStart).Seconds() * 1000

		if time.RealTimeSinceStart()-debugStartTime > 1 {
			println("Warning: engine update > 1 seconds, please check your code ! waitMainCount=", waitMainCount)
			break
		}
	}
	// Main loop ends
	_ = stime.Now()
	stats.LoopTime = stime.Since(loopStart).Seconds() * 1000

	// Queue move starts
	moveStart := stime.Now()
	stats.NextCount = p.nextQueue.Count()
	p.curQueue.Move(p.nextQueue)
	stats.MoveTime = stime.Since(moveStart).Seconds() * 1000

	// Update statistics
	stats.WaitFrameCount = waitFrameCount
	stats.WaitMainCount = waitMainCount

	// Get GC statistics
	var gcStatsAfter sdebug.GCStats
	sdebug.ReadGCStats(&gcStatsAfter)
	stats.GCCount = int(gcStatsAfter.NumGC - gcStatsBefore.NumGC)
	stats.GCPauses = float64(gcStatsAfter.PauseTotal-gcStatsBefore.PauseTotal) / float64(stime.Millisecond)

	// Calculate total time
	_ = stime.Now()
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
