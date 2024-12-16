package coroutine

import (
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/goplus/spx/internal/debug"
	"github.com/goplus/spx/internal/time"
)

var (
	// ErrCannotYieldANonrunningThread represents an "can not yield a non-running thread" error.
	ErrCannotYieldANonrunningThread = errors.New("can not yield a non-running thread")
	ErrAbortThread                  = errors.New("abort thread")
)

// -------------------------------------------------------------------------------------

type ThreadObj interface {
}

type threadImpl struct {
	id           int64
	Obj          ThreadObj
	stopped_     bool
	stack_       string
	stackSimple_ string
	// debug
	startTime float64
	lastDelta float64
}

func (p *threadImpl) Stopped() bool {
	return p.stopped_
}

// Thread represents a coroutine id.
type Thread = *threadImpl

// Coroutines represents a coroutine manager.
type Coroutines struct {
	suspended map[Thread]bool
	current   Thread
	mutex     sync.Mutex
	cond      sync.Cond
	sema      sync.Mutex
	curQueue  *Queue[*WaitJob]
	nextQueue *Queue[*WaitJob]

	curJobId int64
	curThId  int64
	thCount  int64

	// debug infos
	debug          bool
	debugWithTrace bool
}

const (
	waitTypeFrame = iota
	waitTypeTime
	waitTypeMainThread
	waitTypeNoActiveWorker
)

type WaitJob struct {
	Id     int64
	Type   int
	Call   func()
	Time   float64
	Frame  int64
	Thread Thread
}

// New creates a coroutine manager.
func New() *Coroutines {
	p := &Coroutines{
		suspended:      make(map[Thread]bool),
		debug:          false,
		debugWithTrace: false,
	}
	p.cond.L = &p.mutex
	p.curQueue = NewQueue[*WaitJob]()
	p.nextQueue = NewQueue[*WaitJob]()
	return p
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

// CreateAndStart creates and executes the new coroutine.
func (p *Coroutines) CreateAndStart(start bool, tobj ThreadObj, fn func(me Thread) int) Thread {
	cid := atomic.AddInt64(&p.curThId, 1)
	id := &threadImpl{Obj: tobj, id: cid}
	if p.debugWithTrace {
		id.stack_, id.stackSimple_ = debug.GetStackInfo(4)
	}

	atomic.AddInt64(&p.thCount, 1)
	go func() {
		id.startTime = time.RealTimeSinceStart()
		//p.debugLog("taskstart  ", id.id, "  time =", time.RealTimeSinceStartStr(), id.stack_)
		p.sema.Lock()
		p.setCurrent(id)
		defer func() {
			p.mutex.Lock()
			atomic.AddInt64(&p.thCount, -1)
			delete(p.suspended, id)
			p.mutex.Unlock()
			p.sema.Unlock()
			if e := recover(); e != nil {
				if e != ErrAbortThread {
					panic(e)
				}
			}
			id.stopped_ = true
			id.lastDelta = time.RealTimeSinceStart() - id.startTime
			//p.debugLog(fmt.Sprintf("taskdone %d coroDone delta %f, time %f, stack %s \n", id.id, id.lastDelta, time.RealTimeSinceStart(), id.stackSimple_))
		}()
		fn(id)
	}()
	if start {
		runtime.Gosched()
	}
	return id
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

func (p *Coroutines) Yield(me Thread) {
	me.lastDelta = time.RealTimeSinceStart() - me.startTime
	//p.debugLog(fmt.Sprintf("yield %d %f, %s", me.id, me.lastDelta, me.stackSimple_))
	if p.Current() != me {
		panic(ErrCannotYieldANonrunningThread)
	}
	p.sema.Unlock()
	p.mutex.Lock()
	p.suspended[me] = true
	for p.suspended[me] {
		p.cond.Wait()
	}
	p.mutex.Unlock()
	p.sema.Lock()

	//p.debugLog(fmt.Sprintf("id=%d resume %f, %s ", me.id, me.lastDelta, me.stackSimple_))
	p.setCurrent(me)
	if me.stopped_ { // check stopped
		panic(ErrAbortThread)
	}
	me.startTime = time.RealTimeSinceStart()
}

// Resume resumes a suspended coroutine.
func (p *Coroutines) Resume(th Thread) {
	for {
		done := false
		p.mutex.Lock()
		if p.suspended[th] {
			p.suspended[th] = false
			p.cond.Broadcast()
			done = true
		}
		p.mutex.Unlock()
		if done {
			return
		}
		runtime.Gosched()
	}
}

func (p *Coroutines) WaitGroup(wg *sync.WaitGroup) {
	me := p.Current()
	go func() {
		wg.Wait()
		p.Resume(me)
	}()
	p.Yield(me)
}
