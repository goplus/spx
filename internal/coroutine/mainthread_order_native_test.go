//go:build !js && !pure_engine

package coroutine

import (
	"runtime"
	"slices"
	"sync"
	"testing"
	"time"

	itime "github.com/goplus/spx/v3/internal/time"
)

func TestWaitMainThreadPreservesScriptSliceAcrossFrames(t *testing.T) {
	setMainThreadForTest(t, false)
	co := New(nil)
	co.OnInited()
	itime.Start(nil)
	t.Cleanup(func() {
		if !co.StopAllAndWait(time.Second) {
			t.Error("script order test coroutines did not stop")
		}
	})

	var mu sync.Mutex
	var trace []string
	record := func(event string) {
		mu.Lock()
		trace = append(trace, event)
		mu.Unlock()
	}
	first := co.Create("A", func(Thread) int {
		for range 3 {
			co.WaitNextFrame()
			record("A-before")
			co.WaitMainThread(func() {
				record("main")
				if co.runMu.TryLock() {
					co.runMu.Unlock()
					t.Error("main-thread call released the current script slice")
				}
				runtime.Gosched()
			})
			record("A-after")
		}
		return 0
	})
	second := co.Create("B", func(Thread) int {
		for range 3 {
			co.WaitNextFrame()
			record("B")
		}
		return 0
	})
	co.JoinYieldedOrDoneAll([]Thread{first, second})
	co.Update()

	want := []string{"A-before", "main", "A-after", "B"}
	for frame := range 3 {
		itime.Update(1.0/30, 30)
		co.Update()
		mu.Lock()
		got := append([]string(nil), trace...)
		trace = nil
		mu.Unlock()
		if !slices.Equal(got, want) {
			t.Fatalf("frame %d script order = %v, want %v", frame+1, got, want)
		}
	}
	co.JoinAll([]Thread{first, second})
}

func TestRunBetweenScriptsServicesPendingMainThreadCall(t *testing.T) {
	co := New(nil)
	co.OnInited()
	queued := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	t.Cleanup(unblock)
	value := 0
	go func() {
		co.runMu.Lock()
		defer co.runMu.Unlock()
		co.enqueuePriorityJob(&WaitJob{Type: waitTypeMainThread, Call: unblock})
		close(queued)
		<-release
		value = 42
	}()
	<-queued
	result := make(chan int, 1)
	go co.RunBetweenScripts(func() { result <- value })
	select {
	case got := <-result:
		if got != 42 {
			t.Fatalf("script state = %d, want 42", got)
		}
	case <-time.After(time.Second):
		t.Fatal("frame-boundary callback deadlocked behind an engine call")
	}
}
