package gdspx

import (
	inengine "github.com/goplus/spx/v2/internal/gdengine"
	engine "github.com/goplus/spx/v2/pkg/spx/pkg/engine"
)

func IsWebIntepreterMode() bool {
	return inengine.IsWebIntepreterMode()
}

func LinkEngine(coreCallbackInfo engine.CoreCallbackInfo) {
	inengine.Link(coreCallbackInfo)
}

func UnlinkEngine() {
	inengine.Unlink()
}
