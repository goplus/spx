package coroutine

import (
	"errors"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/visualfc/gid"
)

func TestPanicReportPreservesValueAndFaultStack(t *testing.T) {
	errorValue := errors.New("original error")
	for _, value := range []any{"sentinel", errorValue, []string{"uncomparable", "value"}} {
		t.Run(reflect.TypeOf(value).String(), func(t *testing.T) {
			reports := make(chan PanicReport, 1)
			scheduler := New(func(report PanicReport) { reports <- report })
			worker := scheduler.Create("panic-worker", func(Thread) {
				panicWithValue(value)
			})
			waitForThreadSignal(t, worker.finished.done, "panicking coroutine did not finish")
			scheduler.RunAfterStop(nil)
			select {
			case report := <-reports:
				if !reflect.DeepEqual(report.Value, value) {
					t.Errorf("reported panic = %#v, want %#v", report.Value, value)
				}
				if value == errorValue && report.Value != errorValue {
					t.Error("panic report replaced the original error")
				}
				if report.Name != "panic-worker" {
					t.Errorf("reported name = %q, want panic-worker", report.Name)
				}
				if !strings.Contains(report.Stack, "panicWithValue") {
					t.Errorf("report does not include the fault site: %q", report.Stack)
				}
			default:
				t.Fatal("panic was not reported")
			}
		})
	}
}

func TestPanicReportPreservesRuntimePanicAndFaultStack(t *testing.T) {
	reports := make(chan PanicReport, 1)
	scheduler := New(func(report PanicReport) { reports <- report })
	worker := scheduler.Create("runtime-panic-worker", func(Thread) {
		panicWithNilDereference(nil)
	})
	waitForThreadSignal(t, worker.finished.done, "panicking coroutine did not finish")
	scheduler.RunAfterStop(nil)
	select {
	case report := <-reports:
		if _, ok := report.Value.(runtime.Error); !ok {
			t.Errorf("runtime panic type = %T, want runtime.Error", report.Value)
		}
		if !strings.Contains(report.Stack, "panicWithNilDereference") {
			t.Errorf("report does not include the fault site: %q", report.Stack)
		}
	default:
		t.Fatal("runtime panic was not reported")
	}
}

func TestPanicReportIgnoresNormalCompletionAndStopSentinels(t *testing.T) {
	for _, value := range []any{nil, ErrAbortThread, ErrStopThisScript} {
		reports := make(chan PanicReport, 1)
		scheduler := New(func(report PanicReport) { reports <- report })
		worker := scheduler.Create("stopped-worker", func(Thread) {
			if value != nil {
				panic(value)
			}
		})
		waitForThreadSignal(t, worker.finished.done, "coroutine did not finish")
		scheduler.RunAfterStop(nil)
		select {
		case report := <-reports:
			t.Errorf("normal termination %v reported as panic: %+v", value, report)
		default:
		}
	}
}

func TestFinishThreadCleansLifecycleStateWhenPanicHandlerPanics(t *testing.T) {
	scheduler := New(func(PanicReport) {
		panic("panic handler failure")
	})
	worker := scheduler.newThread("panicking-worker")
	scheduler.registerThread(worker)

	recovered := make(chan any, 1)
	go func() {
		goroutineID := gid.Get()
		scheduler.goroutineThreads.Store(goroutineID, worker)
		scheduler.runMu.Lock()
		scheduler.current.Store(worker)
		defer func() { recovered <- recover() }()
		scheduler.finishThread(worker, goroutineID, "worker failure")
	}()

	select {
	case got := <-recovered:
		if got != "panic handler failure" {
			t.Fatalf("panic handler panic = %v, want %q", got, "panic handler failure")
		}
	case <-time.After(time.Second):
		t.Fatal("panicking handler did not return control")
	}
	if scheduler.hasPendingWork() {
		t.Fatal("thread remained registered after panic handler panicked")
	}
	finalizing := 0
	scheduler.callbacks.Range(func(any, any) bool {
		finalizing++
		return true
	})
	if finalizing != 0 {
		t.Fatalf("finalizing goroutine markers = %d, want 0", finalizing)
	}
	scheduler.RunAfterStop(nil)
}

func panicWithValue(value any) { panic(value) }

func panicWithNilDereference(pointer *int) {
	_ = *pointer
}
