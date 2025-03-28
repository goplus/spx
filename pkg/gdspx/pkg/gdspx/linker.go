package gdspx

import (
	inengine "github.com/goplus/spx/pkg/gdspx/internal/engine"
	. "github.com/goplus/spx/pkg/gdspx/pkg/engine"
)

func IsWebIntepreterMode() bool {
	return inengine.IsWebIntepreterMode()
}

func LinkEngine(callback EngineCallbackInfo) {
	inengine.Link(EngineCallbackInfo(callback))
}
