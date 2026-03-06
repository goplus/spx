//go:build pure_engine

package facade

import (
	"github.com/goplus/spx/v2/pkg/spx/pkg/engine"
)

func LinkFFI() bool {
	return true
}

func OnLinked() {

}

func UnlinkFFI() {

}

func RegisterCallbacks(_ engine.CallbackInfo) {

}
