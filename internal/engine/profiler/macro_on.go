//go:build profiler
// +build profiler

package profiler

var Enabled bool = true

func init() {
	println("Profiler Enabled")
}
