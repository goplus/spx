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

func setupRuntimeScheduler(t *testing.T) *coroutine.Coroutines {
	t.Helper()
	co := coroutine.New(nil)
	installRuntimeScheduler(t, co, nil)
	return co
}

// beforeRestore runs after the test's coroutines stop, while their scheduler
// remains bound. Register other resource cleanup before installing the scheduler.
func installRuntimeScheduler(t *testing.T, co *coroutine.Coroutines, beforeRestore func()) {
	t.Helper()
	original := gco
	gco = co
	engine.SetCoroutines(co)
	t.Cleanup(func() {
		if !co.StopAllAndWait(time.Second) {
			t.Error("coroutines did not stop")
		}
		if beforeRestore != nil {
			beforeRestore()
		}
		gco = original
		engine.SetCoroutines(original)
	})
}

func TestRuntimeSchedulerFixtureDrainsBeforeRestoringBindings(t *testing.T) {
	original := setupRuntimeScheduler(t)
	stopped := make(chan struct{})
	cleaned := false
	t.Run("replacement", func(t *testing.T) {
		co := coroutine.New(nil)
		installRuntimeScheduler(t, co, func() {
			if gco != co {
				t.Error("scheduler restored before resource cleanup")
			}
			select {
			case <-stopped:
			default:
				t.Error("resource cleanup ran before coroutine completion")
			}
			cleaned = true
		})
		started := make(chan struct{})
		co.Create("fixture-drain", func(coroutine.Thread) {
			defer close(stopped)
			if !engine.IsInCoroutine() {
				t.Error("engine scheduler binding does not match the test scheduler")
			}
			close(started)
			engine.WaitNextFrame()
		})
		select {
		case <-started:
		case <-time.After(time.Second):
			t.Fatal("test coroutine did not start")
		}
	})
	if !cleaned || gco != original {
		t.Fatal("scheduler cleanup did not complete and restore its previous binding")
	}
	bound := make(chan bool, 1)
	original.Create("fixture-restored", func(coroutine.Thread) {
		bound <- engine.IsInCoroutine()
	})
	select {
	case ok := <-bound:
		if !ok {
			t.Fatal("engine scheduler binding was not restored")
		}
	case <-time.After(time.Second):
		t.Fatal("restored scheduler did not start a coroutine")
	}
}
