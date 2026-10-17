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
			co := New(func(report PanicReport) { reports <- report })
			worker := co.Create("panic-worker", func(Thread) {
				panicWithValue(value)
			})
			waitForThreadSignal(t, worker.done, "panicking coroutine did not finish")
			if co.RunAfterStop(drainTestContext(t), nil) != nil {
				t.Fatal("panic handler did not finish")
			}
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
				if report.CreationStack != "" {
					t.Error("creation stack captured when debug is disabled")
				}
			default:
				t.Fatal("panic was not reported")
			}
		})
	}
}

func panicWithValue(value any) { panic(value) }

func TestPanicReportPreservesRuntimePanicAndCreationStack(t *testing.T) {
	reports := make(chan PanicReport, 1)
	co := New(func(report PanicReport) { reports <- report })
	co.debug = true
	worker := co.Create("runtime-panic-worker", panicWithNilDereference)
	waitForThreadSignal(t, worker.done, "panicking coroutine did not finish")
	if co.RunAfterStop(drainTestContext(t), nil) != nil {
		t.Fatal("panic handler did not finish")
	}
	select {
	case report := <-reports:
		if _, ok := report.Value.(runtime.Error); !ok {
			t.Errorf("runtime panic type = %T, want runtime.Error", report.Value)
		}
		if !strings.Contains(report.Stack, "panicWithNilDereference") {
			t.Errorf("report does not include the fault site: %q", report.Stack)
		}
		if !strings.Contains(report.CreationStack, "TestPanicReportPreservesRuntimePanicAndCreationStack") {
			t.Errorf("report does not include the creation site: %q", report.CreationStack)
		}
		if strings.Contains(report.CreationStack, "panicWithNilDereference") {
			t.Error("creation stack was replaced with the fault stack")
		}
	default:
		t.Fatal("runtime panic was not reported")
	}
}

func panicWithNilDereference(Thread) {
	var pointer *int
	_ = *pointer
}

func TestPanicReportIgnoresNormalCompletionAndStopSentinels(t *testing.T) {
	for _, value := range []any{nil, ErrAbortThread, ErrStopThisScript} {
		reports := make(chan PanicReport, 1)
		co := New(func(report PanicReport) { reports <- report })
		worker := co.Create("stopped-worker", func(Thread) {
			if value != nil {
				panic(value)
			}
		})
		waitForThreadSignal(t, worker.done, "coroutine did not finish")
		if co.RunAfterStop(drainTestContext(t), nil) != nil {
			t.Fatal("coroutine did not drain")
		}
		select {
		case report := <-reports:
			t.Errorf("normal termination %v reported as panic: %+v", value, report)
		default:
		}
	}
}

func TestFinishThreadCleansLifecycleStateWhenPanicHandlerPanics(t *testing.T) {
	co := New(func(PanicReport) {
		panic("panic handler failure")
	})
	worker := co.newThread("panicking-worker")
	co.registerThread(worker)

	recovered := make(chan any, 1)
	go func() {
		goroutineID := gid.Get()
		co.goroutineThreads.Store(goroutineID, worker)
		co.runMu.Lock()
		co.current.Store(worker)
		defer func() { recovered <- recover() }()
		co.finishThread(worker, goroutineID, "worker failure")
	}()

	select {
	case got := <-recovered:
		if got != "panic handler failure" {
			t.Fatalf("panic handler panic = %v, want %q", got, "panic handler failure")
		}
	case <-time.After(time.Second):
		t.Fatal("panicking handler did not return control")
	}
	if co.hasPendingWork() {
		t.Fatal("thread remained registered after panic handler panicked")
	}
	finalizing := 0
	co.callbacks.Range(func(any, any) bool {
		finalizing++
		return true
	})
	if finalizing != 0 {
		t.Fatalf("finalizing goroutine markers = %d, want 0", finalizing)
	}
	if co.RunAfterStop(drainTestContext(t), nil) != nil {
		t.Fatal("manager remained blocked after panic handler panicked")
	}
}
