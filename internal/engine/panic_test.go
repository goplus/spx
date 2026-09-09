//go:build !js && !pure_engine

package engine

import (
	"strings"
	"testing"
	"time"

	"github.com/goplus/spx/v3/internal/coroutine"
	"github.com/goplus/spx/v3/internal/enginewrap"
	gdx "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

type panicRecordingExt struct {
	gdx.IExtMgr
	messages chan string
	exits    chan int64
}

func (r *panicRecordingExt) OnRuntimePanic(message string) { r.messages <- message }
func (r *panicRecordingExt) RequestExit(code int64)        { r.exits <- code }

func TestCoroutinePanicReachesRuntimeWithCauseAndFaultStack(t *testing.T) {
	co := coroutine.New(OnPanic)
	originalCo, originalPlatform, originalExt := gco, gdx.PlatformMgr, gdx.ExtMgr
	recorder := &panicRecordingExt{messages: make(chan string, 1), exits: make(chan int64, 1)}
	SetCoroutines(co)
	gdx.PlatformMgr = resetDirectPlatform{}
	gdx.ExtMgr = recorder
	enginewrap.Init(WaitMainThread)
	t.Cleanup(func() {
		if !co.AbortAllAndWait(time.Second) {
			t.Error("panicking coroutine did not finish")
		}
		SetCoroutines(originalCo)
		gdx.PlatformMgr, gdx.ExtMgr = originalPlatform, originalExt
	})

	co.Create("panic-worker", panicWithSentinelCause)
	select {
	case message := <-recorder.messages:
		for _, want := range []string{"panic-worker: panic: sentinel cause", "stack:", "panicWithSentinelCause"} {
			if !strings.Contains(message, want) {
				t.Errorf("runtime panic message %q does not contain %q", message, want)
			}
		}
	case <-time.After(time.Second):
		t.Fatal("coroutine panic did not reach runtime")
	}
	select {
	case code := <-recorder.exits:
		if code != 1 {
			t.Errorf("panic exit code = %d, want 1", code)
		}
	case <-time.After(time.Second):
		t.Fatal("coroutine panic did not request exit")
	}
}

func panicWithSentinelCause(coroutine.Thread) int {
	panic("sentinel cause")
}
