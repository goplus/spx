//go:build !js

package ispx

import (
	"sync"
	"testing"
	"time"

	"github.com/goplus/ixgo"
	"github.com/goplus/ixgo/xgobuild"
	"github.com/goplus/spx/v4/internal/engine"
	"github.com/goplus/spx/v4/internal/enginewrap"
	gdx "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

type interpreterExitBackend struct {
	gdx.IExtMgr
	exit func(int64)
}

type interpreterPlatform struct{ gdx.IPlatformMgr }

func TestInterpreterShutdownWaitsForRun(t *testing.T) {
	isolateInterpreter(t)
	previousExt, previousPlatform := gdx.ExtMgr, gdx.PlatformMgr
	enginewrap.Init(func(call func()) { call() })
	gdx.PlatformMgr = interpreterPlatform{}
	t.Cleanup(func() {
		gdx.ExtMgr, gdx.PlatformMgr = previousExt, previousPlatform
		enginewrap.Init(engine.WaitMainThread)
	})
	entered, release := make(chan struct{}), make(chan struct{})
	releaseRun := sync.OnceFunc(func() { close(release) })
	defer releaseRun()
	requested := make(chan int64, 1)
	gdx.ExtMgr = interpreterExitBackend{exit: func(code int64) { requested <- code }}
	ctx := ixgo.NewContext(ixgo.SupportMultipleInterp | xgobuild.StaticLoad)
	ctx.RegisterExternal("main.wait", func() { close(entered); <-release })
	if err := Init(ctx); err != nil {
		t.Fatal(err)
	}
	if err := Build(map[string][]byte{"main.xgo": []byte("func wait() {}\nfunc main() { wait() }\n")}); err != nil {
		t.Fatal(err)
	}
	type result struct {
		code int
		err  error
	}
	finished := make(chan result, 1)
	go func() {
		defer close(finished)
		code, err := Run()
		finished <- result{code, err}
	}()
	t.Cleanup(func() {
		releaseRun()
		select {
		case <-finished:
		case <-time.After(3 * time.Second):
			t.Error("interpreter did not finish during cleanup")
		}
	})
	select {
	case <-entered:
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not enter the program")
	}
	requireInterpreterPanic(t, "ispx: already running", func() { _, _ = Run() })
	stopped := make(chan error, 1)
	go func() {
		defer close(stopped)
		stopped <- Shutdown()
	}()
	t.Cleanup(func() {
		releaseRun()
		select {
		case <-stopped:
		case <-time.After(3 * time.Second):
			t.Error("Shutdown did not finish during cleanup")
		}
	})
	select {
	case code := <-requested:
		if code != 0 {
			t.Errorf("exit request = %d, want 0", code)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Shutdown did not request engine exit")
	}
	select {
	case err := <-stopped:
		t.Fatalf("Shutdown returned before Run completed: %v", err)
	default:
	}
	releaseRun()
	select {
	case result := <-finished:
		if result.code != 0 || result.err != nil {
			t.Fatalf("Run() = %d, %v", result.code, result.err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not finish after release")
	}
	select {
	case err := <-stopped:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Shutdown did not observe completion")
	}
	if runDone != nil {
		t.Fatal("completed interpreter still reports a running instance")
	}
}

func (b interpreterExitBackend) RequestExit(code int64) { b.exit(code) }

func (interpreterPlatform) IsMainThread() bool { return true }
