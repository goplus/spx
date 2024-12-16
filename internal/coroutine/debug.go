package coroutine

import (
	"fmt"

	"github.com/goplus/spx/internal/debug"
	"github.com/goplus/spx/internal/time"
)

// -------------------------------------------------------------------------------------
func (p *Coroutines) debugLog(args ...any) {
	if !p.debug {
		return
	}
	debug.Log(args...)
}
func (p *Coroutines) dump() {
	p.mutex.Lock()
	debug.PrintAllStackTrace()
	p.mutex.Unlock()

	p.mutex.Lock()
	for a := range p.suspended {
		p.debugLog(fmt.Sprintf("==id %d, delta%f start%f a.stack%s\n", a.id, a.lastDelta, a.startTime, a.stack_))
	}
	p.mutex.Unlock()
}
func (p *Coroutines) dumpTasks(isTimeout bool, msg string, calledTaskes []*WaitJob) {
	if time.Frame() < 10 {
		return
	}
	if isTimeout {
		print("==")
	}
	fmt.Println(msg)
	if isTimeout {
		for _, job := range calledTaskes {
			if job == nil || job.Thread == nil {
				continue
			}
			p.debugLog("called jobs: ", job.Id, job.Thread.stackSimple_)
		}
		p.dump()
	}
}
