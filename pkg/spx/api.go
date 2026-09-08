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
	"time"

	"github.com/goplus/spx/v3/internal/engine"
)

func isSpxEnv() bool {
	return engine.GetGame() != nil
}

// IsAbortThreadError reports whether err is the SPX abort-thread sentinel.
func IsAbortThreadError(err any) bool {
	return engine.IsAbortThreadError(err)
}

// IsInCoroutine reports whether the caller is running in an SPX coroutine.
func IsInCoroutine() bool {
	return engine.IsInCoroutine()
}

// ExecuteNative runs fn in a native Go goroutine and waits for it, yielding the
// calling SPX coroutine so blocking operations do not stall the scheduler.
// fn receives the coroutine's context and owner. It should return when the context
// is canceled so coroutine shutdown and game reset can finish cleanup.
//
// Outside an SPX coroutine, fn runs directly with context.Background() and nil owner.
func ExecuteNative(fn func(ctx context.Context, owner any)) {
	engine.ExecuteNative(fn)
}

// Execute runs fn and blocks until it completes. With an active game, it uses
// the current SPX coroutine or creates one for owner; fn receives a context
// canceled when that coroutine is stopped, including on game reset. Otherwise,
// fn runs in a Go goroutine with context.Background() and the supplied owner.
//
// When a new SPX coroutine is created, nil owner defaults to the game. In an
// existing coroutine, fn receives the supplied owner without changing the
// coroutine's owner or cancellation context.
func Execute(owner any, fn func(ctx context.Context, owner any)) {
	if isSpxEnv() {
		engine.Execute(owner, fn)
		return
	}

	done := make(chan struct{}, 1)
	go func() {
		defer close(done)
		fn(context.Background(), owner)
	}()
	<-done
}

// Go starts fn in an SPX coroutine when a game is active. A nil owner defaults
// to the current coroutine's owner or the game. fn receives a context canceled
// when the coroutine is stopped, including on game reset.
// Without an active game, it starts a Go goroutine with context.Background()
// and the supplied owner.
//
// Long-running SPX callbacks must yield with Wait or WaitNextFrame and respect
// context cancellation so the scheduler can advance frames and stop scripts.
// Panics in SPX callbacks are handled by the engine.
//
// Example:
//
//	spx.Go(owner, func(ctx context.Context, owner any) {
//	    for ctx.Err() == nil {
//	        // Perform one iteration of work.
//	        spx.WaitNextFrame()
//	    }
//	})
func Go(owner any, fn func(ctx context.Context, owner any)) {
	if isSpxEnv() {
		engine.GoWithOwner(owner, fn)
	} else {
		go fn(context.Background(), owner)
	}
}

// Wait pauses for secs seconds and returns the actual elapsed time in seconds.
// In an SPX coroutine it yields to the scheduler; otherwise it blocks the calling
// goroutine with time.Sleep. Fractional seconds are supported.
func Wait(secs float64) float64 {
	if engine.IsInCoroutine() {
		return engine.Wait(secs)
	} else {
		startTime := time.Now()
		time.Sleep(time.Duration(secs * float64(time.Second)))
		return time.Since(startTime).Seconds()
	}
}

// WaitNextFrame normally suspends an SPX coroutine until the next frame. In
// run-without-screen-refresh mode, it returns immediately until the execution
// budget is exhausted, then yields until the next frame. In both cases, it
// returns the engine's delta time in seconds.
// Outside a coroutine, it sleeps for approximately 16 milliseconds and returns
// the actual elapsed time in seconds.
func WaitNextFrame() float64 {
	if engine.IsInCoroutine() {
		return engine.WaitNextFrameIfNeeded()
	} else {
		startTime := time.Now()
		time.Sleep(time.Millisecond * 16)
		return time.Since(startTime).Seconds()
	}
}
