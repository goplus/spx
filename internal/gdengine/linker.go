package gdengine

import (
	"github.com/goplus/spx/v2/internal/gdengine/binding/facade"
	engineimpl "github.com/goplus/spx/v2/internal/gdengine/impl"
	. "github.com/goplus/spx/v2/pkg/spx/pkg/engine"
	gdspx "github.com/goplus/spx/v2/pkg/spx/pkg/gdspx"
)

var (
	mgrs                []IManager
	coreCallbacks       CoreCallbackInfo
	sprites             = make([]ISpriter, 0)
	isWebIntepreterMode bool
)

func init() {
	gdspx.SetLinkerBridge(linkerBridge{})
}

type linkerBridge struct{}

func (linkerBridge) IsWebIntepreterMode() bool {
	return IsWebIntepreterMode()
}

func (linkerBridge) Link(coreCallbackInfo CoreCallbackInfo) {
	Link(coreCallbackInfo)
}

func (linkerBridge) Unlink() {
	Unlink()
}

func IsWebIntepreterMode() bool {
	return isWebIntepreterMode
}

func Link(coreCallbackInfo CoreCallbackInfo) {
	isWebIntepreterMode = facade.LinkFFI()
	coreCallbacks = coreCallbackInfo
	infos := bindCallbacks()
	facade.RegisterCallbacks(infos)
	mgrs = engineimpl.CreateMgrs()
	engineimpl.BindMgr(mgrs)
	facade.OnLinked()
}

func Unlink() {
	mgrs = nil
	facade.UnlinkFFI()
}
