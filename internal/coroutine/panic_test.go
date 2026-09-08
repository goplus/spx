package coroutine

import (
	"errors"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestPanicReportPreservesValueAndFaultStack(t *testing.T) {
	errorValue := errors.New("original error")
	for _, value := range []any{"sentinel", errorValue, []string{"uncomparable", "value"}} {
		t.Run(reflect.TypeOf(value).String(), func(t *testing.T) {
			reports := make(chan PanicReport, 1)
			co := New(func(report PanicReport) { reports <- report })
			worker := co.Create("panic-worker", func(Thread) int {
				panicWithValue(value)
				return 0
			})
			waitForThreadSignal(t, worker.done, "panicking coroutine did not finish")
			if !co.AbortAllAndWait(time.Second) {
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
	if !co.AbortAllAndWait(time.Second) {
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

func panicWithNilDereference(Thread) int {
	var pointer *int
	return *pointer
}

func TestPanicReportIgnoresNormalCompletionAndStopSentinels(t *testing.T) {
	for _, value := range []any{nil, ErrAbortThread, ErrStopThisScript} {
		reports := make(chan PanicReport, 1)
		co := New(func(report PanicReport) { reports <- report })
		worker := co.Create("stopped-worker", func(Thread) int {
			if value != nil {
				panic(value)
			}
			return 0
		})
		waitForThreadSignal(t, worker.done, "coroutine did not finish")
		if !co.AbortAllAndWait(time.Second) {
			t.Fatal("coroutine did not drain")
		}
		select {
		case report := <-reports:
			t.Errorf("normal termination %v reported as panic: %+v", value, report)
		default:
		}
	}
}
