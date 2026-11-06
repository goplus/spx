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

// Completed waits return immediately. Other callers block; managed callers
// publish blocking and registration under schedulerMu, then waiterSet.mu.
func (p *Coroutines) waitOn(me Thread, done <-chan struct{}, waiters *waiterSet) {
	select {
	case <-done:
		return
	default:
	}
	p.requireCanWait()
	if me == nil {
		<-done
		return
	}
	p.schedulerMu.Lock()
	registered := waiters.add(me)
	if registered {
		p.blockThreadLocked(me)
	}
	p.schedulerCond.Signal()
	p.schedulerMu.Unlock()

	if registered {
		// Cancellation unregisters even when yield panics.
		defer waiters.remove(me)
		p.yield(me)
	}
}

func (p *Coroutines) wakeWaiters(waiters map[Thread]struct{}) {
	p.schedulerMu.Lock()
	p.wakeWaitersLocked(waiters)
	p.schedulerCond.Signal()
	p.schedulerMu.Unlock()
}
