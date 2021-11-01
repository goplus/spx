package coroutine

import (
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"
)

var (
	// ErrCannotYieldANonrunningThread represents an "can not yield a non-running thread" error.
	ErrCannotYieldANonrunningThread = errors.New("can not yield a non-running thread")
	ErrAbortThread                  = errors.New("abort thread")
)

// -------------------------------------------------------------------------------------

type ThreadObj interface {
	Stopped() bool
}

type threadImpl struct {
	Obj ThreadObj
}

// Thread represents a coroutine id.
//
type Thread = *threadImpl

// Coroutines represents a coroutine manager.
//
type Coroutines struct {
	suspended map[Thread]bool
	current   Thread
	mutex     sync.Mutex
	cond      sync.Cond
	sema      sync.Mutex
	init      bool
}

// New creates a coroutine manager.
//
func New() *Coroutines {
	p := &Coroutines{
		suspended: make(map[Thread]bool),
		init:      false,
	}
	p.cond.L = &p.mutex
	return p
}

// Create creates a new coroutine.
//
func (p *Coroutines) Create(tobj ThreadObj, fn func(me Thread) int) Thread {
	return p.CreateAndStart(tobj, fn, nil)
}

// CreateAndStart creates and executes the new coroutine.
//
func (p *Coroutines) CreateAndStart(tobj ThreadObj, fn func(me Thread) int, main Thread) Thread {
	id := &threadImpl{Obj: tobj}
	go func() {
		p.sema.Lock()
		p.setCurrent(id)
		defer func() {
			p.mutex.Lock()
			delete(p.suspended, id)
			p.mutex.Unlock()
			p.sema.Unlock()
			if e := recover(); e != nil {
				if e != ErrAbortThread {
					panic(e)
				}
			}
		}()
		p.init = true
		fn(id)
	}()
	if main != nil {
		runtime.Gosched()
	}
	return id
}

func (p *Coroutines) setCurrent(id Thread) {
	atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&p.current)), unsafe.Pointer(id))
}

func (p *Coroutines) Current() Thread {
	return Thread(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&p.current))))
}

// Yield suspends a running coroutine.
//
func (p *Coroutines) Yield(me Thread) {
	if p.Current() != me {
		panic(ErrCannotYieldANonrunningThread)
	}
	if p.init == false {
		//log.Printf("Coroutines is not init success")
		return
	}
	p.sema.Unlock()
	p.mutex.Lock()
	p.suspended[me] = true
	for p.suspended[me] {
		p.cond.Wait()
	}
	p.mutex.Unlock()
	p.sema.Lock()

	p.setCurrent(me)
	if me.Obj != nil && me.Obj.Stopped() { // check stopped
		panic(ErrAbortThread)
	}
}

// Resume resumes a suspended coroutine.
//
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

// Sched func.
//
func (p *Coroutines) Sched(me Thread) {
	go func() {
		p.Resume(me)
	}()
	p.Yield(me)
}

// -------------------------------------------------------------------------------------
