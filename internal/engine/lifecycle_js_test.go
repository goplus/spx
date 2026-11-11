//go:build js && wasm

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

package engine

import (
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/goplus/spx/v4/internal/engine/coroutine"
	gdx "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

func TestRequestExitStopsManagedCallerOnWeb(t *testing.T) {
	isolateGameBinding(t)
	current := coroutine.New(nil)
	setupWebResetTest(t, current)
	game := new(bindingTestGame)
	if _, err := bindGame(game, game); err != nil {
		t.Fatal(err)
	}
	backend := &resetBlockingExt{
		callback: onReset,
		entered:  make(chan int64, 1),
		release:  make(chan struct{}),
	}
	close(backend.release)
	gdx.ExtMgr = backend
	var returned atomic.Bool
	caller := current.Create("exit caller", func(coroutine.Thread) {
		RequestExit(73)
		returned.Store(true)
	})
	deadline := time.Now().Add(time.Second)
	for activeGame.Load() != nil && time.Now().Before(deadline) {
		current.Update()
		runtime.Gosched()
	}
	if activeGame.Load() != nil {
		t.Fatal("public exit did not release the binding")
	}
	current.Join(caller)
	if returned.Load() || caller.Context().Err() == nil {
		t.Fatal("managed exit caller was not stopped")
	}
	select {
	case code := <-backend.entered:
		if code != 73 {
			t.Fatalf("reset code = %d, want 73", code)
		}
	default:
		t.Fatal("public exit did not request backend reset")
	}
	if backend.calls.Load() != 1 || game.reset.Load() != 1 {
		t.Fatalf("reset calls: backend = %d, game = %d; want 1 each", backend.calls.Load(), game.reset.Load())
	}
	waitForResetAdmissionOpen(t, current)
}
