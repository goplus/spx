/*
 * Copyright (c) 2021 The XGo Authors (xgo.dev). All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package spx

import (
	"testing"
	"time"

	"github.com/goplus/spx/v3/internal/coroutine"
	"github.com/goplus/spx/v3/internal/engine"
)

func TestProcedureConsumesStopThisScript(t *testing.T) {
	Procedure(func() {
		panic(coroutine.ErrStopThisScript)
	})
}

func TestProcedurePropagatesOtherPanics(t *testing.T) {
	tests := []struct {
		name  string
		panic any
	}{
		{name: "abort thread", panic: coroutine.ErrAbortThread},
		{name: "user panic", panic: "user panic"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			defer func() {
				if got := recover(); got != test.panic {
					t.Fatalf("panic = %v, want %v", got, test.panic)
				}
			}()
			Procedure(func() {
				panic(test.panic)
			})
		})
	}
}

func TestProcedureScopesStopThisScriptAcrossRepeat(t *testing.T) {
	co := coroutine.New(nil)
	original := gco
	gco = co
	engine.SetCoroutines(co)
	t.Cleanup(func() {
		if !co.AbortAllAndWait(time.Second) {
			t.Error("coroutines did not stop")
		}
		gco = original
		engine.SetCoroutines(original)
	})

	done := make(chan string, 1)
	co.CreateAndStart(true, "stop-this-script", func(coroutine.Thread) int {
		trace := "outer-before;"
		Procedure(func() {
			trace += "inner-before;"
			Repeat(1, func() {
				// Match a stop nested in a converted repeat callback.
				co.AbortThisScript()
				trace += "inner-bad;"
			})
			trace += "inner-bad2;"
		})
		trace += "outer-after;"
		done <- trace
		return 0
	})

	select {
	case got := <-done:
		if want := "outer-before;inner-before;outer-after;"; got != want {
			t.Fatalf("trace = %q, want %q", got, want)
		}
	case <-time.After(time.Second):
		t.Fatal("stop-this-script boundary did not return from procedure")
	}
}

func TestStopThisScriptAtEventBoundaryEndsThread(t *testing.T) {
	panicReported := make(chan struct{}, 1)
	co := coroutine.New(func(name, stack string) {
		panicReported <- struct{}{}
	})
	original := gco
	gco = co
	engine.SetCoroutines(co)
	t.Cleanup(func() {
		if !co.AbortAllAndWait(time.Second) {
			t.Error("coroutines did not stop")
		}
		gco = original
		engine.SetCoroutines(original)
	})

	var script scriptEventBindings
	script.init(&scriptEventRegistry{}, "owner")
	thread := co.CreateAndStart(true, "event-stop-this-script", func(coroutine.Thread) int {
		script.Stop(ThisScript)
		panic("stop-this-script should not continue")
	})

	select {
	case <-thread.Context().Done():
	case <-time.After(time.Second):
		t.Fatal("event thread did not terminate after stop-this-script")
	}
	select {
	case <-panicReported:
		t.Fatal("stop-this-script reached the panic reporter")
	default:
	}
}
