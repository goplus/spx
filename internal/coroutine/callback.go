package coroutine

import "github.com/visualfc/gid"

type callbackScope uint8

const (
	callbackNoDrain callbackScope = 1 << iota
	callbackExclusive
)

type callbackResult struct {
	panicValue any
	panicked   bool
}

// invokeCallback invokes fn with drain protection and captures panics.
func (p *Coroutines) invokeCallback(fn func()) (result callbackResult) {
	id, previous := p.enterCallback(callbackNoDrain)
	defer p.leaveCallback(id, previous)
	// Distinguish panic(nil) from a normal return.
	result.panicked = true
	defer func() {
		result.panicValue = recover()
	}()
	fn()
	result.panicked = false
	return result
}

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

func (p *Coroutines) callbackScope() callbackScope {
	if value, ok := p.callbacks.Load(gid.Get()); ok {
		return value.(callbackScope)
	}
	return 0
}

func (p *Coroutines) requireDrainCaller() {
	if p.callbackScope() != 0 {
		panic(ErrReentrantWait)
	}
}
