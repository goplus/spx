//go:build js

package coroutine

// Direct-platform waits park so host callbacks can run.
const hasMainThreadQueue = false
