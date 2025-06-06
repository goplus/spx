package gdspx

import (
	inengine "github.com/goplus/spx/v2/pkg/gdspx/internal/engine"
	. "github.com/goplus/spx/v2/pkg/gdspx/pkg/engine"
)

func IsWebIntepreterMode() bool {
	return inengine.IsWebIntepreterMode()
}

func LinkEngine(callback EngineCallbackInfo) {
	inengine.Link(EngineCallbackInfo(callback))
}
