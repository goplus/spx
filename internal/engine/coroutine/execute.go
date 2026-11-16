package coroutine

import "github.com/goplus/spx/v4/internal/engine/platform"

// RunSync executes call in a managed coroutine and waits for completion.
// Managed callers run inline; engine callers service main-thread jobs while
// waiting. External calls are skipped while shutdown has closed admission.
func (p *Scheduler) RunSync(owner any, call func(Thread)) {
	if caller := p.Caller(); caller != nil {
		p.requireCanWait()
		call(caller)
		return
	}
	if p.admissionClosed() {
		return
	}
	p.requireCanWait()
	if platform.TryCallEngineDirectly(func() {
		// Keep engine-thread ownership through creation and the pumping wait.
		// Shutdown may have closed admission after the initial check.
		if p.admissionClosed() {
			return
		}
		dispatcher := p.Create(owner, call)
		p.waitForSignal(dispatcher.finished.done)
	}) {
		return
	}
	p.Join(p.Create(owner, call))
}
