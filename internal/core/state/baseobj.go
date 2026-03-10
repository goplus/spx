package state

import "github.com/goplus/spx/v2/internal/engine"

type BaseObjRuntimeState struct {
	SyncSprite     *engine.Sprite
	Scale          float64
	IsCostumeSet   bool
	IsCostumeDirty bool
	Layer          int
	IsLayerDirty   bool
	HasShader      bool
	IsAnimating    bool
	hasDestroyed   bool
}

func (s *BaseObjRuntimeState) IsDestroyed() bool {
	return s.hasDestroyed
}

func (s *BaseObjRuntimeState) SetDestroyed(destroyed bool) {
	s.hasDestroyed = destroyed
}
