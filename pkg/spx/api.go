package spx

import (
	"sync/atomic"
	"time"

	"github.com/goplus/spx/v2/internal/engine"
)

// HasInit checks if the SPX engine has been initialized.
func IsSpxEnv() bool {
	return engine.IsSpxEnv()
}

// Executes the given function in a native Go goroutine from the current SPX coroutine context and waits for completion.
// While waiting, it yields control via waitNextFrame to avoid blocking the SPX main thread.
// Use this when you need to run potentially blocking Go operations (e.g., network requests, file I/O) from within SPX.
func RunGoFromSpx(fn func()) {
	done := &atomic.Bool{}
	// Run the actual logic in a go routine to avoid blocking
	go func() {
		defer done.Store(true)
		fn()
	}()
	// Wait for completion while yielding control to SPX
	for !done.Load() {
		WaitNextFrame()
	}
}

// Executes the given function in an SPX coroutine from the current Go goroutine context and waits for completion.
// This function blocks until fn finishes execution.
// Use this when you need to synchronously wait for the SPX coroutine to complete.
func RunSpxFromGo(fn func()) {
	done := make(chan struct{}, 1)
	StartSpxCoro(func() {
		defer close(done)
		fn()
	})
	<-done
}

// Starts a new spx coroutine that executes the given function concurrently.
// This is useful for running multiple operations in parallel without blocking
// the main execution flow.
//
// IMPORTANT: For long-running tasks, you MUST call Wait() or WaitNextFrame()
// periodically to yield control back to the engine. Without these calls,
// the main thread will wait indefinitely for the coroutine to complete,
// causing the entire game to freeze.
//
// Note: The function will be executed in the game engine's coroutine context.
// Any panics in the function will be handled by the engine's panic recovery mechanism.
//
// Example of correct usage for long-running tasks:
//
//	done := false
//	// ... do something
//	spx.GoAsync(func() {
//	    // ... do something
//	    for !done {
//	        // Do some work here
//	        spx.WaitNextFrame() // CRITICAL: Yield control to prevent freezing
//	    }
//	})
//
// Example of simple delayed execution:
//
//	spx.GoAsync(func() {
//	    spx.Wait(2.0)
//	    fmt.Println("Hello after 2 seconds")
//	})
func StartSpxCoro(fn func()) {
	if IsSpxEnv() {
		engine.Go(engine.GetGame(), func() {
			fn()
		})
	} else {
		go fn()
	}
}

// Wait pauses the current coroutine for the specified number of seconds.
// It returns the actual time waited, which may differ slightly from the requested time
// due to frame timing and engine scheduling.
//
// Parameters:
//
//	secs - The number of seconds to wait (can be fractional, e.g., 0.5 for half a second)
//
// Returns:
//
//	The actual time waited in seconds
//
// Note: This function only works within a spx coroutine context (e.g., inside a Go function).
// Calling this from the main thread will block the entire game.
//
// Example:
//
//	actualTime := spx.Wait(1.5) // Wait for 1.5 seconds
func Wait(secs float64) float64 {
	if IsSpxEnv() {
		return engine.Wait(secs)
	} else {
		// Fallback to a regular wait
		startTime := time.Now()
		time.Sleep(time.Duration(secs * float64(time.Second)))
		return time.Since(startTime).Seconds()
	}
}

// WaitNextFrame pauses the current coroutine until the next frame is rendered.
// This is useful for spreading computationally expensive operations across multiple frames
// to maintain smooth frame rates.
//
// Returns:
//
//	The time elapsed since the last frame in seconds (delta time)
//
// Note: This function only works within a spx coroutine context (e.g., inside a Go function).
// It's commonly used in loops to prevent blocking the main thread for too long.
//
// Example:
//
//	for i := 0; i < 1000; i++ {
//	    // Do some expensive work
//	    if i%100 == 0 {
//	        spx.WaitNextFrame() // Yield control every 100 iterations
//	    }
//	}
func WaitNextFrame() float64 {
	if IsSpxEnv() {
		return engine.WaitNextFrame()
	} else {
		// Fallback to a regular wait
		startTime := time.Now()
		time.Sleep(time.Millisecond * 16) // Approx 60 FPS
		return time.Since(startTime).Seconds()
	}
}
