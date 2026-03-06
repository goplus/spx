package engine

import (
	"runtime/debug"

	spxlog "github.com/goplus/spx/v2/internal/log"
)

func PrintStack() {
	spxlog.Debug("%s", debug.Stack())
}
