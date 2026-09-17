package coroutine

import "sync"

// waiterSet owns one-shot waiter registration. Its zero value is open to waiters.
type waiterSet struct {
	mu      sync.Mutex
	closed  bool
	threads map[Thread]struct{}
}

// Publish blocking and registration under schedulerMu, then waiterSet.mu.
// Cancellation unregisters even when Yield panics.
func (p *Coroutines) waitOn(me Thread, waiters *waiterSet) {
	p.schedulerMu.Lock()
	p.setThreadStateLocked(me, threadBlocked)
	registered := waiters.add(me)
	if !registered {
		p.setThreadStateLocked(me, threadRunnable)
	}
	p.schedulerCond.Signal()
	p.schedulerMu.Unlock()

	if registered {
		defer waiters.remove(me)
		p.Yield(me)
	}
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
