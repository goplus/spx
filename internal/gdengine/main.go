package gdengine

import (
	"github.com/goplus/spx/v2/internal/gdengine/binding/facade"
	engineimpl "github.com/goplus/spx/v2/internal/gdengine/impl"
	. "github.com/goplus/spx/v2/pkg/spx/pkg/engine"
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
	isWebIntepreterMode = facade.LinkFFI()
	mgrs = engineimpl.CreateMgrs()
	coreCallbacks = coreCallbackInfo
	infos := bindCallbacks()
	facade.RegisterCallbacks(infos)
	engineimpl.BindMgr(mgrs)
	facade.OnLinked()
}

func Unlink() {
	mgrs = nil
	facade.UnlinkFFI()
}
