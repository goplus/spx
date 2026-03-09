package state

import "github.com/goplus/spx/v2/internal/engine"

type BaseObjRuntimeState struct {
	SyncSprite     *engine.Sprite
	Scale          float64
	HasDestroyed   bool
	IsCostumeSet   bool
	IsCostumeDirty bool
	Layer          int
	IsLayerDirty   bool
	HasShader      bool
	IsAnimating    bool
}
