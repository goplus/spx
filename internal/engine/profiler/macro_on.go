//go:build profiler
// +build profiler

package profiler

import spxlog "github.com/goplus/spx/v2/internal/log"

func init() {
	Enabled = true
	spxlog.Info("Profiler enabled (macro_on)")
}
