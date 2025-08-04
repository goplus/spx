package api

import (
	"github.com/goplus/spx/v2/internal/engine"
)

// Go starts a new spx coroutine that executes the given function concurrently.
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
//	api.Go(func() {
//	    // ... do something
//	    for !done {
//	        // Do some work here
//	        api.WaitNextFrame() // CRITICAL: Yield control to prevent freezing
//	    }
//	})
//
// Example of simple delayed execution:
//
//	api.Go(func() {
//	    api.Wait(2.0)
//	    fmt.Println("Hello after 2 seconds")
//	})
func Go(fn func()) {
	engine.Go(engine.GetGame(), func() {
		fn()
	})
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
//	actualTime := api.Wait(1.5) // Wait for 1.5 seconds
func Wait(secs float64) float64 {
	return engine.Wait(secs)
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
//	        api.WaitNextFrame() // Yield control every 100 iterations
//	    }
//	}
func WaitNextFrame() float64 {
	return engine.WaitNextFrame()
}
