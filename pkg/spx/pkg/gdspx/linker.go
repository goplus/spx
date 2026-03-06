package gdspx

import (
	engine "github.com/goplus/spx/v2/pkg/spx/pkg/engine"
)

type LinkerBridge interface {
	IsWebIntepreterMode() bool
	Link(coreCallbackInfo engine.CoreCallbackInfo)
	Unlink()
}

var linkerBridge LinkerBridge

func SetLinkerBridge(bridge LinkerBridge) {
	linkerBridge = bridge
}

func requireLinkerBridge() LinkerBridge {
	if linkerBridge == nil {
		panic("gdspx linker bridge is not initialized")
	}
	return linkerBridge
}

// IsWebIntepreterMode is a compatibility shim around the runtime linker bridge.
func IsWebIntepreterMode() bool {
	return requireLinkerBridge().IsWebIntepreterMode()
}

// LinkEngine is a compatibility shim around the runtime linker bridge.
func LinkEngine(coreCallbackInfo engine.CoreCallbackInfo) {
	requireLinkerBridge().Link(coreCallbackInfo)
}

// UnlinkEngine is a compatibility shim around the runtime linker bridge.
func UnlinkEngine() {
	requireLinkerBridge().Unlink()
}
