package coroutine

import (
	"fmt"
	"sort"
	"strings"

	"github.com/goplus/spx/internal/time"
)

var debugSb strings.Builder

func (p *Coroutines) DebugLog(info ...any) {
	p.debugLog("p.debugLog", info...)
}
func (p *Coroutines) debugLog(msg string, args ...any) {
	p.debugLogWithId(gCurThreadId, msg, args...)
}
func (p *Coroutines) debugLogWithId(id int64, msg string, args ...any) {
	if !p.debug {
		return
	}
	argsStr := fmt.Sprint(args...)
	dt := int64(time.RealTimeSinceStart() * 1000000)
	val := fmt.Sprint(id, " "+msg+"  ", dt, argsStr)
	debugSb.WriteString(val)
	debugSb.WriteString("\n")
}

func (p *Coroutines) printCorotines() {
	var sb strings.Builder
	idx := 0
	p.waitMutex.Lock()
	ths := make([]*threadImpl, len(p.waiting))
	for thread, val := range p.waiting {
		thread.isActive_ = !val
		ths[idx] = thread
		idx++
	}
	p.waitMutex.Unlock()
	sort.Slice(ths, func(i, j int) bool {
		return ths[i].duration_ > ths[j].duration_
	})

	for i, thread := range ths {
		str := thread.stackSimple_
		dt := thread.duration_ * 1000
		if dt > 3 {
			str = thread.stack_
		}
		msg := fmt.Sprintf(" %d, isActive %t, duration %fms isWaitFrame %t  stack:%s \n", i, thread.isActive_, dt, thread.isWaitFrame_, str)
		sb.WriteString(msg)
	}

	msg := fmt.Sprintf("printCorotines coroCount= %d frame:%d deltaTime %d \n", len(p.waiting), time.Frame(), int(time.DeltaTime()*1000))
	println(msg, sb.String())
}
