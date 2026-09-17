//go:build js || pure_engine

package coroutine

// Direct-platform waits park so host callbacks can run.
const hasMainThreadQueue = false
