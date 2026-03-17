package coroutine

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"sync"
	"sync/atomic"
	stime "time"
	"unsafe"

	"github.com/goplus/spx/v2/internal/debug"
	"github.com/goplus/spx/v2/internal/time"
	"github.com/petermattis/goid"
)

type ThreadObj any

type threadImpl struct {
	Obj       ThreadObj
	stopped   atomic.Bool
	suspended atomic.Bool // Per-thread suspension state to avoid lock-order inversion
	frame     int
	mutex     sync.Mutex // Mutex for this thread's condition variable
	cond      *sync.Cond // Per-thread condition variable for targeted wake-up
	id        int64
	name      string
	stack     string

	schedFrame     int64
	schedTimestamp stime.Time

	ctx        context.Context
	cancelFunc context.CancelFunc
}

// Thread represents a coroutine id.
type Thread = *threadImpl

type threadNamer interface {
	Name() string
}

// -------------------------------------------------------------------------------------
// Thread Methods
// -------------------------------------------------------------------------------------

func (p *threadImpl) Context() context.Context {
	if p.ctx == nil {
		return context.Background()
	}
	return p.ctx
}

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
	return p.stopped.Load()
}

func (p Thread) IsSchedTimeout(ms float64) bool {
	frame := time.Frame()
	if p.schedFrame < frame {
		p.schedFrame = frame
		p.schedTimestamp = stime.Now()
	}
	timeout := stime.Since(p.schedTimestamp) > stime.Duration(ms)*stime.Millisecond
	return timeout
}

// -------------------------------------------------------------------------------------
// Thread Creation & Management
// -------------------------------------------------------------------------------------

func (p *Coroutines) Create(obj ThreadObj, fn func(me Thread) int) Thread {
	return p.CreateAndStart(false, obj, fn)
}

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
		if !th.stopped.Load() {
			th.stopped.Store(true)
			th.Cancel()
			th.cond.Signal()
		}
		th.mutex.Unlock()
	}
}

func (p *Coroutines) AbortAllAndWait(timeout stime.Duration) bool {
	p.AbortAll()

	if timeout <= 0 {
		p.wg.Wait()
		return true
	}

	deadline := stime.Now().Add(timeout)
	for {
		p.mutex.Lock()
		done := len(p.allThreads) == 0
		p.mutex.Unlock()
		if done {
			return true
		}

		remaining := stime.Until(deadline)
		if remaining <= 0 {
			return false
		}
		if remaining > 10*stime.Millisecond {
			remaining = 10 * stime.Millisecond
		}
		stime.Sleep(remaining)
	}
}

func (p *Coroutines) StopIf(filter func(th Thread) bool) {
	p.mutex.Lock()
	allThreads := make([]Thread, 0, len(p.allThreads))
	for th := range p.allThreads {
		allThreads = append(allThreads, th)
	}
	p.mutex.Unlock()

	threads := allThreads[:0]
	for _, th := range allThreads {
		if filter(th) {
			threads = append(threads, th)
		}
	}

	// Stop each thread with proper signaling
	for _, th := range threads {
		th.mutex.Lock()
		th.stopped.Store(true)
		th.Cancel()
		th.cond.Signal()
		th.mutex.Unlock()
	}
}

func (p *Coroutines) IsInCoroutine() bool {
	currentGID := goid.Get()
	_, exists := p.goroutineIDs.Load(currentGID)
	return exists
}

// -------------------------------------------------------------------------------------
// Internal Helpers (unexported)
// -------------------------------------------------------------------------------------

func (p *Coroutines) setCurrent(id Thread) {
	atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&p.current)), unsafe.Pointer(id))
}

func resolveThreadName(obj ThreadObj) string {
	if obj == nil {
		return ""
	}

	if str, ok := obj.(string); ok {
		return str
	}

	if named, ok := obj.(threadNamer); ok {
		return named.Name()
	}

	t := reflect.TypeOf(obj)
	if t.Kind() != reflect.Pointer || t.Elem().Name() == "" {
		return ""
	}

	return "*" + t.Elem().Name()
}

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
	p.wg.Add(1)
	p.mutex.Lock()
	p.allThreads[th] = struct{}{}
	p.mutex.Unlock()
}

func (p *Coroutines) unregisterThread(th Thread) {
	p.mutex.Lock()
	delete(p.suspended, th)
	delete(p.allThreads, th)
	p.mutex.Unlock()
	p.wg.Done()
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
