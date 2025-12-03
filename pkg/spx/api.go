package spx

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/goplus/spx/v2/internal/engine"
)

func isSpxEnv() bool {
	return engine.GetGame() != nil
}

// IsAbortThreadError checks whether the given error is an SPX coroutine termination error.
// This function is used to determine if an error was thrown due to SPX coroutine termination.
//
// Parameters:
//
//	err - The error to check
//
// Returns:
//
//	true if the error is an SPX coroutine termination error, false otherwise
//
// Example:
//
//	defer func() {
//	    if r := recover(); r != nil {
//	        if spx.IsAbortThreadError(r) {
//	            // Handle SPX coroutine termination
//	            return
//	        }
//	        // Handle other panics
//	        panic(r)
//	    }
//	}()
func IsAbortThreadError(err any) bool {
	return engine.IsAbortThreadError(err)
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
// While waiting, if called from within an SPX coroutine, it yields control via WaitNextFrame to avoid blocking
// the SPX main thread.
//
// The function receives the context from the current SPX coroutine, which will be canceled when
// the coroutine is aborted (e.g., game reset, owner destroyed). The native function SHOULD respect the
// context and return when ctx.Done() is closed to allow proper cleanup.
//
// This function is essential when you need to perform blocking Go operations (such as network requests,
// file I/O, or system calls) from within an SPX coroutine without freezing the game engine.
//
// If called from outside an SPX coroutine context, the function receives context.Background() and nil owner.
//
// Parameters:
//
//	fn - The function to execute, receiving the context and owner (nil if not in a coroutine).
//
// Example - HTTP request with context:
//
//	spx.ExecuteNative(func(ctx context.Context, owner any) {
//	    // Create cancellable HTTP request
//	    req, _ := http.NewRequestWithContext(ctx, "GET", "https://api.example.com/data", nil)
//	    resp, err := http.DefaultClient.Do(req)
//	    if err != nil {
//	        if ctx.Err() == context.Canceled {
//	            log.Println("Request canceled")
//	            return
//	        }
//	        log.Printf("Error: %v", err)
//	        return
//	    }
//	    defer resp.Body.Close()
//	    // Process response...
//	})
func ExecuteNative(fn func(ctx context.Context, owner any)) {
	ctx := engine.GetCurrentThreadContext()
	// if not in spx coro, just run it
	if !engine.IsInCoroutine() {
		fn(ctx, nil)
		return
	}
	owner := engine.GetCoroutineOwner()
	done := &atomic.Bool{}
	// Execute the actual logic in a go routine to avoid blocking
	go func() {
		defer done.Store(true)
		fn(ctx, owner)
	}()
	// Wait for completion while yielding control to SPX
	for !done.Load() {
		WaitNextFrame()
	}
}

// Execute executes the given function in an SPX coroutine from the current Go goroutine context and waits for completion.
// This function blocks until fn finishes execution.
//
// If already in an SPX coroutine, the function executes directly in the current coroutine.
// If not in a coroutine, a new SPX coroutine is created and the caller blocks until it completes.
//
// The function receives the current coroutine's context, which will be canceled when the coroutine
// is aborted (e.g., game reset, owner destroyed).
//
// Parameters:
//
//	owner - The SPX coroutine owner. When the owner is destroyed, all coroutines created by this owner will be properly stopped.
//	fn - The function to execute in the coroutine context, receiving the context and owner.
//
// Example:
//
//	spx.Execute(sprite, func(ctx context.Context, owner any) {
//	    // This runs in an SPX coroutine and can use SPX APIs
//	    spx.Wait(1.0)
//	    sprite := owner.(*MySprite)
//	    sprite.Say("Hello")
//	})
func Execute(owner any, fn func(ctx context.Context, owner any)) {
	// in spx coro, just run it
	if engine.IsInCoroutine() {
		fn(engine.GetCurrentThreadContext(), owner)
		return
	}

	done := make(chan struct{}, 1)
	Go(owner, func(ctx context.Context, owner any) {
		defer close(done)
		fn(ctx, owner)
	})
	<-done
}

// Go starts a new SPX coroutine that executes the given function concurrently.
// This is useful for running multiple operations in parallel without blocking
// the main execution flow.
//
// The function receives a context that will be canceled when the coroutine is aborted
// (e.g., game reset, owner destroyed). The function SHOULD check ctx.Done() for long-running
// operations to allow proper cleanup.
//
// Parameters:
//
//	owner - The SPX coroutine owner. When the owner is destroyed, all coroutines created by this owner will be properly stopped.
//	        If nil, the current coroutine's owner or the game instance will be used as the owner.
//	fn - The function to execute in the coroutine context, receiving the context and owner.
//
// IMPORTANT: For long-running tasks, you MUST call Wait() or WaitNextFrame()
// periodically to yield control back to the engine. Without these calls,
// the main thread will wait indefinitely for the coroutine to complete,
// causing the entire game to freeze.
//
// Note: The function will be executed in the game engine's coroutine context.
// Any panics in the function will be handled by the engine's panic recovery mechanism.
//
// Example of correct usage for long-running tasks with context:
//
//	done := false
//	spx.Go(owner, func(ctx context.Context, owner any) {
//	    for !done {
//	        select {
//	        case <-ctx.Done():
//	            // Context canceled (e.g., game reset)
//	            fmt.Println("Coroutine canceled")
//	            return
//	        default:
//	            // Do some work here
//	            spx.WaitNextFrame() // CRITICAL: Yield control to prevent freezing
//	        }
//	    }
//	})
//
// Example of simple delayed execution:
//
//	spx.Go(owner, func(ctx context.Context, owner any) {
//	    spx.Wait(2.0)
//	    fmt.Println("Hello after 2 seconds")
//	})
func Go(owner any, fn func(ctx context.Context, owner any)) {
	if isSpxEnv() {
		if owner == nil {
			if IsInCoroutine() {
				owner = engine.GetCoroutineOwner()
			} else {
				owner = engine.GetGame()
			}
		}
		engine.Go(owner, func(ctx context.Context) {
			fn(ctx, owner)
		})
	} else {
		go fn(context.Background(), owner)
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
