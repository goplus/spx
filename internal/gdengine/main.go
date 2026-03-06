package gdengine

import (
	engineimpl "github.com/goplus/spx/v2/internal/gdengine/impl"
	"github.com/goplus/spx/v2/internal/wrap"
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
	isWebIntepreterMode = wrap.LinkFFI()
	mgrs = engineimpl.CreateMgrs()
	coreCallbacks = coreCallbackInfo
	infos := bindCallbacks()
	wrap.RegisterCallbacks(infos)
	engineimpl.BindMgr(mgrs)
	wrap.OnLinked()
}

func Unlink() {
	mgrs = nil
	wrap.UnlinkFFI()
}
