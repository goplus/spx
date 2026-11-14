package coroutine

import "sync"

// completion owns a one-shot signal and its managed waiters.
// It must be initialized with newCompletion and not copied after first use.
type completion struct {
	done    chan struct{}
	mu      sync.Mutex
	closed  bool
	threads map[Thread]struct{}
}

func newCompletion() completion {
	return completion{done: make(chan struct{})}
}

func (p *completion) add(thread Thread) bool {
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

func (p *completion) remove(thread Thread) {
	p.mu.Lock()
	delete(p.threads, thread)
	if len(p.threads) == 0 {
		p.threads = nil
	}
	p.mu.Unlock()
}

// complete publishes completion once and transfers the waiters to the caller,
// which must wake them outside the lock.
func (p *completion) complete() map[Thread]struct{} {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil
	}
	p.closed = true
	close(p.done)
	waiters := p.threads
	p.threads = nil
	return waiters
}

// Completed waits return immediately. Other callers block; managed callers
// publish blocking and registration under schedulerMu, then completion.mu.
func (p *Scheduler) waitOn(me Thread, signal *completion) {
	select {
	case <-signal.done:
		return
	default:
	}
	p.requireCanWait()
	if me == nil {
		<-signal.done
		return
	}
	p.schedulerMu.Lock()
	registered := signal.add(me)
	if registered {
		p.blockThreadLocked(me)
	}
	p.schedulerCond.Signal()
	p.schedulerMu.Unlock()

	if registered {
		// Cancellation unregisters even when yield panics.
		defer signal.remove(me)
		p.yield(me)
	}
}

func (p *Scheduler) wakeWaiters(waiters map[Thread]struct{}) {
	p.schedulerMu.Lock()
	p.markWaitersPendingLocked(waiters)
	p.scheduleLocked()
	p.schedulerCond.Signal()
	p.schedulerMu.Unlock()
}
