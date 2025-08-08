package spx

import (
	"sync/atomic"
	"time"

	"github.com/goplus/spx/v2/internal/engine"
)

func isSpxEnv() bool {
	return engine.GetGame() != nil
}

// IsInCoroutine checks whether the current execution context is within an SPX coroutine.
// Returns true if running inside an SPX coroutine, false if running in a regular Go goroutine
// or the main thread.
//
// This function is useful for determining the appropriate execution strategy when your code
// needs to work in both SPX coroutine and regular Go contexts.
//
// Example:
//
//	if spx.IsInCoroutine() {
//	    // Use SPX-specific functions like Wait() or WaitNextFrame()
//	    spx.Wait(1.0)
//	} else {
//	    // Use regular Go functions
//	    time.Sleep(time.Second)
//	}
func IsInCoroutine() bool {
	return engine.IsInCoroutine()
}

// ExecuteNative executes the given function in a native Go goroutine and waits for its completion.
// While waiting, if it is in spx corotine, it yields control via WaitNextFrame to avoid blocking
// the SPX main thread.
//
// This function is essential when you need to perform blocking Go operations (such as network requests,
// file I/O, or system calls) from within an SPX coroutine without freezing the game engine.
//
// If called from outside an SPX coroutine context, the function executes synchronously.
//
// Example:
//
//	spx.ExecuteNative(func() {
//	    // Perform blocking network request
//	    resp, err := http.Get("https://api.example.com/data")
//	    if err != nil {
//	        log.Printf("Error: %v", err)
//	        return
//	    }
//	    defer resp.Body.Close()
//	    // Process response...
//	})
func ExecuteNative(fn func(owner any)) {
	// if not in spx coro, just run it
	if !engine.IsInCoroutine() {
		fn(nil)
		return
	}
	owner := engine.GetCoroutineOwner()
	done := &atomic.Bool{}
	// Execute the actual logic in a go routine to avoid blocking
	go func() {
		defer done.Store(true)
		fn(owner)
	}()
	// Wait for completion while yielding control to SPX
	for !done.Load() {
		WaitNextFrame()
	}
}

// Executes the given function in an SPX coroutine from the current Go goroutine context and waits for completion.
// This function blocks until fn finishes execution.
// Use this when you need to synchronously wait for the SPX coroutine to complete.
//
// Parameters:
//
//	owner - The SPX coroutine owner. When the owner is destroyed, all coroutines created by this owner will be properly stopped.
//	fn - The function to execute in the coroutine context.
func Execute(owner any, fn func(owner any)) {
	// in spx coro, just run it
	if engine.IsInCoroutine() {
		fn(owner)
		return
	}

	done := make(chan struct{}, 1)
	Go(owner, func(any) {
		defer close(done)
		fn(owner)
	})
	<-done
}

// Starts a new spx coroutine that executes the given function concurrently.
// This is useful for running multiple operations in parallel without blocking
// the main execution flow.
//
// Parameters:
//
//	owner - The SPX coroutine owner. When the owner is destroyed, all coroutines created by this owner will be properly stopped.
//	        If nil, the current coroutine's owner or the game instance will be used as the owner.
//	fn - The function to execute in the coroutine context.
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
//	spx.Go(owner, func(owner any) {
//	    // ... do something
//	    for !done {
//	        // Do some work here
//	        spx.WaitNextFrame() // CRITICAL: Yield control to prevent freezing
//	    }
//	})
//
// Example of simple delayed execution:
//
//	spx.Go(owner, func(owner any) {
//	    spx.Wait(2.0)
//	    fmt.Println("Hello after 2 seconds")
//	})
func Go(owner any, fn func(owner any)) {
	if isSpxEnv() {
		if owner == nil {
			if IsInCoroutine() {
				owner = engine.GetCoroutineOwner()
			} else {
				owner = engine.GetGame()
			}
		}
		engine.Go(owner, func() {
			fn(owner)
		})
	} else {
		go fn(owner)
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
	if engine.IsInCoroutine() {
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
	if engine.IsInCoroutine() {
		return engine.WaitNextFrame()
	} else {
		// Fallback to a regular wait
		startTime := time.Now()
		time.Sleep(time.Millisecond * 16) // Approx 60 FPS
		return time.Since(startTime).Seconds()
	}
}
