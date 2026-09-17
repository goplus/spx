package coroutine

import "github.com/visualfc/gid"

type callbackScope uint8

const (
	callbackExternal callbackScope = 1 << iota
	callbackSetup
	callbackCleanup
	callbackFinalizing
	callbackShutdown
	callbackExclusive
)

// Nested callbacks retain their caller's waiting restrictions.
func (p *Coroutines) enterCallback(scope callbackScope) (id uint64, previous callbackScope) {
	id = gid.Get()
	if value, ok := p.callbacks.Load(id); ok {
		previous = value.(callbackScope)
	}
	p.callbacks.Store(id, previous|scope)
	return
}

func (p *Coroutines) leaveCallback(id uint64, previous callbackScope) {
	if previous == 0 {
		p.callbacks.Delete(id)
	} else {
		p.callbacks.Store(id, previous)
	}
}

func (p *Coroutines) currentCallback() callbackScope {
	if value, ok := p.callbacks.Load(gid.Get()); ok {
		return value.(callbackScope)
	}
	return 0
}

func (p *Coroutines) requireDrainCaller() {
	if p.currentCallback() != 0 {
		panic(ErrReentrantWait)
	}
}

func (p *Coroutines) waitOutsideScript(done <-chan struct{}) {
	select {
	case <-done:
		return
	default:
	}
	if p.currentCallback()&callbackExclusive != 0 {
		panic(ErrReentrantWait)
	}
	<-done
}
