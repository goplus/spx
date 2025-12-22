package gdspx

import (
	inengine "github.com/goplus/spx/v2/pkg/gdspx/internal/engine"
	engine "github.com/goplus/spx/v2/pkg/gdspx/pkg/engine"
)

func IsWebIntepreterMode() bool {
	return inengine.IsWebIntepreterMode()
}

func LinkEngine(callback engine.EngineCallbackInfo) {
	inengine.Link(engine.EngineCallbackInfo(callback))
}

func UnlinkEngine() {
	inengine.Unlink()
}
