package coroutine

import "sync"

// waiterSet owns one-shot waiter registration. Its zero value is open to waiters.
type waiterSet struct {
	mu      sync.Mutex
	closed  bool
	threads map[Thread]struct{}
}

func (p *waiterSet) add(thread Thread) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return false
	}
	if p.threads == nil {
		p.threads = make(map[Thread]struct{})
	}
	p.threads[thread] = struct{}{}
	return true
}

func (p *waiterSet) remove(thread Thread) {
	p.mu.Lock()
	delete(p.threads, thread)
	if len(p.threads) == 0 {
		p.threads = nil
	}
	p.mu.Unlock()
}

// close transfers the waiters to the caller, which must wake them outside the
// lock. A nil done lets thread completion publish its channel after cancellation.
func (p *waiterSet) close(done chan struct{}) map[Thread]struct{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil
	}
	p.closed = true
	if done != nil {
		close(done)
	}
	waiters := p.threads
	p.threads = nil
	return waiters
}

// Publish blocking and registration under schedulerMu, then waiterSet.mu.
// Cancellation unregisters even when Yield panics.
func (p *Coroutines) waitOn(me Thread, waiters *waiterSet) {
	p.schedulerMu.Lock()
	delete(p.runnableThreads, me)
	registered := waiters.add(me)
	if !registered {
		p.runnableThreads[me] = struct{}{}
	}
	p.schedulerCond.Signal()
	p.schedulerMu.Unlock()

	if registered {
		defer waiters.remove(me)
		p.yield(me)
	}
}

func (p *Coroutines) wakeWaiters(waiters map[Thread]struct{}) {
	for waiter := range waiters {
		p.wakeThread(waiter)
	}
}

func (p *Coroutines) waitOutsideScript(done <-chan struct{}) {
	select {
	case <-done:
		return
	default:
	}
	if p.callbackScope()&callbackExclusive != 0 {
		panic(ErrReentrantWait)
	}
	<-done
}
