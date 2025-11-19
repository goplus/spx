//go:build !profiler
// +build !profiler

package profiler

func init() {
	Enabled = false
	//println("Profiler Disabled (macro_off)")
}
