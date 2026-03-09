/*
 * Copyright (c) 2021 The XGo Authors (xgo.dev). All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package spx

import (
	coreruntime "github.com/goplus/spx/v2/internal/core/runtime"
	"github.com/goplus/spx/v2/internal/engine"
)

// syncUpdateLogic updates game logic and fires start events.
func (p *Game) syncUpdateLogic() error {
	coreruntime.SyncOnce(&p.StartFlag, func() {
		p.fireEvent(&eventStart{})
	})
	return nil
}

// syncEnginePositions synchronizes sprite positions from the physics engine.
// This is done in batch for performance optimization.
func (p *Game) syncEnginePositions() error {
	coreruntime.SyncBatchPositions(
		p.getTempShapes(),
		func(item Shape) bool {
			sprite, ok := item.(*SpriteImpl)
			return ok && sprite.shouldSyncPhysicsPosition()
		},
		func(item Shape) int64 {
			return int64(item.(*SpriteImpl).SyncSprite.Id)
		},
		engine.SyncBatchGetPositions,
		func(item Shape, x, y float64) {
			item.(*SpriteImpl).syncPhysicsPosition(x, y)
		},
	)
	return nil
}

// syncUpdateInput updates input state from the engine.
func (p *Game) syncUpdateInput() {
	coreruntime.SyncMousePos(engine.MainThreadGetMousePos(), p.inputMgr.setMousePos)
}

// syncUpdateProxy updates all sprite proxies and synchronizes them with the engine.
func (p *Game) syncUpdateProxy() {
	p.camera.onUpdate()
	p.spriteMgr.flushActivate()
	p.syncBuffer.Clear()
	p.spriteMgr.syncProxyStates(p.syncBuffer)
	p.spriteMgr.flushDestroy(p.syncBuffer)
	p.flushSyncBuffer()
	p.camera.setDirtyFlag(false)
}

// flushSyncBuffer sends batched updates to the engine if there are any changes.
func (p *Game) flushSyncBuffer() {
	coreruntime.FlushSerializedBuffer(
		p.syncBuffer.UpdateCount(),
		p.syncBuffer.DeleteCount(),
		p.syncBuffer.Serialize,
		engine.SyncBatchUpdateSprites,
	)
}
