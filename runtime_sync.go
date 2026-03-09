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

import "github.com/goplus/spx/v2/internal/engine"

// syncUpdateLogic updates game logic and fires start events.
func (p *Game) syncUpdateLogic() error {
	p.startFlag.Do(func() {
		p.fireEvent(&eventStart{})
	})
	return nil
}

// syncEnginePositions synchronizes sprite positions from the physics engine.
// This is done in batch for performance optimization.
func (p *Game) syncEnginePositions() error {
	items := p.getTempShapes()
	spriteIDs, sprites := p.collectPhysicsSyncTargets(items)
	positions := engine.SyncBatchGetPositions(spriteIDs)
	for i, sprite := range sprites {
		sprite.syncPhysicsPosition(float64(positions[i*2]), float64(positions[i*2+1]))
	}
	return nil
}

// syncUpdateInput updates input state from the engine.
func (p *Game) syncUpdateInput() {
	p.inputMgr.setMousePos(engine.MainThreadGetMousePos())
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

func (p *Game) collectPhysicsSyncTargets(items []Shape) ([]int64, []*SpriteImpl) {
	spriteIDs := make([]int64, 0, len(items))
	sprites := make([]*SpriteImpl, 0, len(items))
	for _, item := range items {
		sprite, ok := item.(*SpriteImpl)
		if !ok || !sprite.shouldSyncPhysicsPosition() {
			continue
		}
		spriteIDs = append(spriteIDs, int64(sprite.syncSprite.Id))
		sprites = append(sprites, sprite)
	}
	return spriteIDs, sprites
}

// flushSyncBuffer sends batched updates to the engine if there are any changes.
func (p *Game) flushSyncBuffer() {
	if p.syncBuffer.UpdateCount() > 0 || p.syncBuffer.DeleteCount() > 0 {
		engine.SyncBatchUpdateSprites(p.syncBuffer.Serialize())
	}
}
