package enginewrap

import (
	"github.com/goplus/spx/v2/internal/engine/platform"
	spxlog "github.com/goplus/spx/v2/internal/log"
)

var mainCallback func(call func())

func Init(call func(f func())) {
	mainCallback = call
}

func callInMainThread(call func()) {
	if platform.IsMainThread() {
		call()
		return
	}
	if mainCallback == nil {
		panic("enginewrap: Init must be called before using manager methods off the main thread")
	}
	mainCallback(call)
}

const (
	MOUSE_BUTTON_LEFT   int64 = 1
	MOUSE_BUTTON_RIGHT  int64 = 2
	MOUSE_BUTTON_MIDDLE int64 = 3
)

// =============== input ===================
func (pself *inputMgrImpl) MousePressed() bool {
	return inputMgr.GetMouseState(MOUSE_BUTTON_LEFT) || inputMgr.GetMouseState(MOUSE_BUTTON_RIGHT)
}

// =============== window ===================

func (pself *platformMgrImpl) SetRunnableOnUnfocused(flag bool) {
	if !flag {
		spxlog.Warn("SetRunnableOnUnfocused(false) is not implemented yet")
	}
}
