package ffi

import (
	"unsafe"

	"github.com/goplus/spx/v2/pkg/gdspx/pkg/engine"
)
import "C"

var (
	resolveCFunc func(string) unsafe.Pointer
	callbacks    engine.CallbackInfo
)

//go:linkname main main.main
func main()

func Link() bool {
	return false
}

func Linked() {

}

func Unlink() {

}

func BindCallback(info engine.CallbackInfo) {
	callbacks = info
}

//export gdspx_init
func gdspx_init(lookupFunc uintptr, classes, configuration unsafe.Pointer) uint8 {
	_ = classes // reserved for future class registration
	resolveCFunc = func(s string) unsafe.Pointer {
		return getProcAddress(lookupFunc, s)
	}

	builtinAPI.resolveAPIFunctions()
	api.resolveAPIFunctions()
	init := (*initialization)(configuration)
	*init = initialization{}
	init.minimum_initialization_level = initializationLevel(GDExtensionInitializationLevelScene)
	doInitialization(init)
	registerEngineCallback()
	return 1
}
