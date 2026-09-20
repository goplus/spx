package coroutine

import (
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestNativeTasksDrainAfterLastCanceledCallerWorker(t *testing.T) {
	for _, test := range []struct {
		name string
		exit func()
	}{
		{name: "return", exit: func() {}},
		{name: "goexit", exit: runtime.Goexit},
	} {
		t.Run(test.name, func(t *testing.T) {
			co := New(nil)
			started := [2]chan struct{}{make(chan struct{}), make(chan struct{})}
			unblock := [2]chan struct{}{make(chan struct{}), make(chan struct{})}
			finished := [2]chan struct{}{make(chan struct{}), make(chan struct{})}
			release := [2]func(){
				sync.OnceFunc(func() { close(unblock[0]) }),
				sync.OnceFunc(func() { close(unblock[1]) }),
			}
			t.Cleanup(func() {
				for _, releaseWorker := range release {
					releaseWorker()
				}
				if !co.StopAllAndWait(time.Second) {
					t.Error("native task test did not drain during cleanup")
				}
			})

			var callers [2]Thread
			for i := range callers {
				callers[i] = co.Create("caller", func(Thread) {
					co.WaitToDo(func() {
						defer close(finished[i])
						close(started[i])
						<-unblock[i]
						if i == 1 {
							test.exit()
						}
					})
				})
			}
			for _, signal := range started {
				waitForThreadSignal(t, signal, "native worker did not start")
			}
			for _, caller := range callers {
				co.Stop(caller)
			}
			for _, caller := range callers {
				waitForThreadSignal(t, caller.done, "canceled caller did not finish")
			}

			reset := make(chan struct{})
			drained := make(chan bool, 1)
			go func() {
				drained <- co.RunAfterStopAll(2*time.Second, func() { close(reset) })
			}()
			release[0]()
			waitForThreadSignal(t, finished[0], "first native worker did not finish")
			select {
			case <-reset:
				t.Fatal("reset ran while the last native worker was still running")
			case completed := <-drained:
				t.Fatalf("drain returned %v before the last native worker finished", completed)
			case <-time.After(25 * time.Millisecond):
			}

			release[1]()
			waitForThreadSignal(t, finished[1], "last native worker did not finish")
			select {
			case completed := <-drained:
				if !completed {
					t.Fatal("runtime did not drain after the last native worker finished")
				}
			case <-time.After(time.Second):
				t.Fatal("drain did not wake after the last native worker finished")
			}
			waitForThreadSignal(t, reset, "drain completed without running reset")
		})
	}
}
