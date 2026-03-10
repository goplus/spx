package state

import (
	"sync/atomic"

	"github.com/goplus/spx/v2/internal/engine"
)

type BaseObjRuntimeState struct {
	SyncSprite     *engine.Sprite
	Scale          float64
	IsCostumeSet   bool
	IsCostumeDirty bool
	Layer          int
	IsLayerDirty   bool
	HasShader      bool
	IsAnimating    bool
	hasDestroyed   atomic.Bool
}

func (s *BaseObjRuntimeState) IsDestroyed() bool {
	return s.hasDestroyed.Load()
}

func (s *BaseObjRuntimeState) MarkDestroyed() {
	s.hasDestroyed.Store(true)
}
