//go:build !js

/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
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
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/goplus/spx/v4/internal/coroutine"
	gdx "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
	"github.com/visualfc/gid"
)

func TestDrainAndReleaseGameKeepsBindingWhenDrainIsInterrupted(t *testing.T) {
	isolateGameBinding(t)
	current := setupExecuteTest(t)
	platform := &executeMainThreadPlatform{}
	gdx.PlatformMgr = platform
	binding, err := bindGame(new(bindingTestGame), "old")
	if err != nil {
		t.Fatal(err)
	}
	beginGameClose(binding)
	workerStarted, releaseWorker := make(chan struct{}), make(chan struct{})
	release := sync.OnceFunc(func() { close(releaseWorker) })
	t.Cleanup(func() { release(); current.RunAfterStop(func() { releaseGame(binding) }) })
	worker := current.Create("pending worker", func(coroutine.Thread) {
		current.WaitToDo(func() { close(workerStarted); <-releaseWorker })
	})
	waitForEngineSignal(t, workerStarted, time.Second, "native worker did not start")
	cleanupStarted := make(chan struct{})
	cleanupOwner := current.Create("cleanup exits engine caller", func(thread coroutine.Thread) {
		defer current.CleanupMainThread(runtime.Goexit)
		close(cleanupStarted)
		<-thread.Context().Done()
	})
	waitForEngineSignal(t, cleanupStarted, time.Second, "cleanup owner did not start")
	exited := make(chan struct{})
	var cleaned, returned atomic.Bool
	go func() {
		defer close(exited)
		platform.mainGID.Store(gid.Get())
		drainAndReleaseGame(current, binding, func() { cleaned.Store(true) })
		returned.Store(true)
	}()
	waitForEngineSignal(t, exited, time.Second, "drain did not pump Goexit cleanup")
	current.Join(cleanupOwner)
	current.Join(worker)
	if cleaned.Load() || returned.Load() {
		t.Fatal("interrupted drain ran game cleanup or returned normally")
	}
	if activeGame.Load() != binding {
		t.Fatal("interrupted drain released an active native worker's game")
	}
	if _, err := beginGame(new(bindingTestGame), "too early"); !errors.Is(err, ErrGameAlreadyRunning) {
		t.Fatalf("interrupted drain allowed rebinding: %v", err)
	}
	probe := current.Create("before recovery", func(coroutine.Thread) { t.Error("interrupted drain reopened admission") })
	if !probe.Stopped() {
		t.Fatal("interrupted drain admitted a new script")
	}
	release()
	drainAndReleaseGame(current, binding, nil)
	if activeGame.Load() != nil {
		t.Fatal("completed recovery retained game binding")
	}
	fresh := current.Create("after recovery", func(coroutine.Thread) {})
	current.Join(fresh)
	if fresh.Stopped() {
		t.Fatal("successful drain recovery did not reopen admission")
	}
}
