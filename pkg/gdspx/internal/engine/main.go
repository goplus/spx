package engine

import (
	"github.com/goplus/spx/v2/pkg/gdspx/internal/wrap"
	. "github.com/goplus/spx/v2/pkg/gdspx/pkg/engine"
)

var (
	mgrs                []IManager
	coreCallbacks       CoreCallbackInfo
	sprites             = make([]ISpriter, 0)
	isWebIntepreterMode bool
)

func IsWebIntepreterMode() bool {
	return isWebIntepreterMode
}

func Link(coreCallbackInfo CoreCallbackInfo) {
	isWebIntepreterMode = wrap.LinkFFI()
	mgrs = wrap.CreateMgrs()
	coreCallbacks = coreCallbackInfo
	infos := bindCallbacks()
	wrap.RegisterCallbacks(infos)
	wrap.BindMgr(mgrs)
	wrap.OnLinked()
}

func Unlink() {
	mgrs = nil
	wrap.UnlinkFFI()
}
