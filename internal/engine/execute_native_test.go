//go:build !js && !pure_engine

package engine

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	gdx "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
	"github.com/visualfc/gid"
)

type executeMainThreadPlatform struct {
	gdx.IPlatformMgr
	mainGID atomic.Uint64
}

func (p *executeMainThreadPlatform) IsMainThread() bool {
	return gid.Get() == p.mainGID.Load()
}

func TestExecuteOnEngineThreadServicesMainThreadCalls(t *testing.T) {
	setupExecuteTest(t)
	platform := &executeMainThreadPlatform{}
	gdx.PlatformMgr = platform
	returned := make(chan struct{})
	var calls atomic.Int32
	var wrongThread atomic.Bool
	go func() {
		platform.mainGID.Store(gid.Get())
		Execute("engine-caller", func(context.Context, any) {
			for range 2 {
				WaitMainThread(func() {
					if !platform.IsMainThread() {
						wrongThread.Store(true)
					}
					calls.Add(1)
				})
			}
		})
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("Execute on the engine thread did not service main-thread calls")
	}
	if calls.Load() != 2 || wrongThread.Load() {
		t.Fatalf("main-thread calls = %d, wrong thread = %v", calls.Load(), wrongThread.Load())
	}
}
