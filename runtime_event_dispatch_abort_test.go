//go:build !js && !pure_engine
// +build !js,!pure_engine

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
	"sync/atomic"
	"testing"
	"time"

	"github.com/goplus/spx/v3/internal/coroutine"
	pkgengine "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

func TestAsyncDispatchAbortAfterCallbackStartsDoesNotRunRejectedLifecycle(t *testing.T) {
	co := setupRuntimeEventScheduler(t)
	previousPlatform := pkgengine.PlatformMgr
	pkgengine.PlatformMgr = &eventDispatchPlatform{}
	t.Cleanup(func() { pkgengine.PlatformMgr = previousPlatform })

	callbackStarted := make(chan struct{})
	releaseCallback := make(chan struct{})
	var nested, registered, ran atomic.Bool
	handler := &messageEventHandler{}
	event := scriptEventDispatch{
		mode: coroutine.BatchAsync,
		lifecycle: func(thread coroutine.Thread, _ *eventSink) func() {
			registered.Store(true)
			return handler.start(thread)
		},
		run: func(coroutine.Thread, *eventSink) { ran.Store(true) },
	}
	go func() {
		co.TryRunManagedBetweenScripts("outer", func() {
			close(callbackStarted)
			<-releaseCallback
			nested.Store(true)
			dispatchMatchedScriptEventBatch([]eventSink{{Owner: handler, Handler: handler}}, event)
		})
	}()
	select {
	case <-callbackStarted:
	case <-time.After(time.Second):
		t.Fatal("outer dispatcher callback did not start")
	}

	abortDone := make(chan bool, 1)
	go func() { abortDone <- co.RunAfterAbortAll(20*time.Millisecond, nil) }()
	select {
	case completed := <-abortDone:
		if completed {
			t.Fatal("abort barrier completed while callback was blocked")
		}
	case <-time.After(time.Second):
		t.Fatal("abort barrier did not time out")
	}
	close(releaseCallback)
	// The outer dispatcher must finish even though its child is rejected.
	deadline := time.After(time.Second)
	for !nested.Load() {
		select {
		case <-deadline:
			t.Fatal("nested batch was not registered")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	if ran.Load() {
		t.Fatal("rejected nested handler ran after abort")
	}
	if registered.Load() {
		t.Fatal("rejected nested handler ran lifecycle registration after abort")
	}
	if !co.RunAfterAbortAll(time.Second, nil) {
		t.Fatal("explicit recovery barrier did not complete")
	}
	handler.mu.Lock()
	active := handler.active
	handler.mu.Unlock()
	if active != nil {
		t.Fatalf("rejected lifecycle left active thread %v", active)
	}
}
