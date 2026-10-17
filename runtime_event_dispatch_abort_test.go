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
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/goplus/spx/v3/internal/coroutine"
	pkgengine "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

func TestAsyncDispatchAbortAfterCallbackStartsDoesNotRunRejectedLifecycle(t *testing.T) {
	co := setupRuntimeScheduler(t)
	platform := &eventDispatchPlatform{}
	previousPlatform := pkgengine.PlatformMgr
	pkgengine.PlatformMgr = platform
	t.Cleanup(func() { pkgengine.PlatformMgr = previousPlatform })

	callbackStarted := make(chan struct{})
	releaseCallback := make(chan struct{})
	registrationDone := make(chan struct{})
	var registered, ran atomic.Bool
	handler := &messageEventHandler{}
	lifecycle := eventLifecycleFunc(func(thread coroutine.Thread) func() {
		registered.Store(true)
		return handler.RegisterInvocation(thread)
	})
	event := scriptEventDispatch{
		mode: coroutine.BatchAsync,
		run:  func(coroutine.Thread, *eventSink) { ran.Store(true) },
	}
	go func() {
		platform.useCurrentAsMainThread()
		co.TryRunFromEngine("outer", func() {
			close(callbackStarted)
			<-releaseCallback
			dispatchMatchedScriptEventBatch([]eventSink{{Owner: handler, Handler: lifecycle}}, event)
			close(registrationDone)
		})
	}()
	select {
	case <-callbackStarted:
	case <-time.After(time.Second):
		t.Fatal("outer dispatcher callback did not start")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	stopDone := make(chan error, 1)
	go func() { stopDone <- co.RunAfterStop(ctx, nil) }()
	select {
	case err := <-stopDone:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("blocked shutdown returned %v, want deadline exceeded", err)
		}
	case <-time.After(time.Second):
		t.Fatal("shutdown barrier did not time out")
	}
	close(releaseCallback)
	select {
	case <-registrationDone:
	case <-time.After(time.Second):
		t.Fatal("nested batch registration did not finish")
	}
	if registered.Load() {
		t.Fatal("rejected nested handler ran lifecycle registration after abort")
	}
	ctx, cancel = context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := co.RunAfterStop(ctx, nil); err != nil {
		t.Fatalf("explicit recovery barrier did not complete: %v", err)
	}
	if ran.Load() {
		t.Fatal("rejected nested handler ran after abort")
	}
	co.StartBatch([]coroutine.Task{{
		Owner:              handler,
		RegisterInvocation: handler.RegisterInvocation,
		Run:                func(coroutine.Thread) { ran.Store(true) },
	}}, coroutine.BatchAsync)
	co.Update()
	if !ran.Load() {
		t.Fatal("handler could not run after the shutdown barrier recovered")
	}
}
