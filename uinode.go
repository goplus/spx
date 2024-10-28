package spx

import (
	gdspx "github.com/realdream-ai/gdspx/pkg/engine"

	"github.com/goplus/spx/internal/engine"
)

type ProxyUi struct {
	gdspx.UiNode
}

func NewUiNode(path string) *ProxyUi {
	node := engine.SyncCreateUiNode[ProxyUi](path)
	return node
}
func (pthis *ProxyUi) SetText(text string) {
	engine.SyncUiSetText(pthis.UiNode.Id, text)
}
