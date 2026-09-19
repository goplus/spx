/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
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
	"testing"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v3/internal/engine"
	itime "github.com/goplus/spx/v3/internal/time"
	pkgengine "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

type cloneCollisionSpriteMgr struct {
	*spyCloneSpriteMgr
	visible     map[pkgengine.Object]bool
	ghost       map[pkgengine.Object]float64
	sensedGhost bool
}

func (m *cloneCollisionSpriteMgr) SetVisible(id pkgengine.Object, visible bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.visible[id] = visible
}

func (m *cloneCollisionSpriteMgr) SetTransform(id pkgengine.Object, _ mathf.Vec2, _ float64, _ mathf.Vec2, visible bool, _ mathf.Vec2) {
	m.SetVisible(id, visible)
}

func (m *cloneCollisionSpriteMgr) SetMaterialParams(id pkgengine.Object, effect string, amount float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if effect == GhostEffect.String() {
		m.ghost[id] = amount
	}
}

func (m *cloneCollisionSpriteMgr) BatchUpdateTransforms(buffer []float32) {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := 2
	for range int(buffer[0]) {
		m.visible[pkgengine.Object(buffer[idx])] = buffer[idx+engine.SyncFieldsPerSprite-1] != 0
		idx += engine.SyncFieldsPerSprite
	}
	for range int(buffer[1]) {
		id := pkgengine.Object(buffer[idx])
		delete(m.visible, id)
		delete(m.ghost, id)
		idx++
	}
}

func (m *cloneCollisionSpriteMgr) CheckCollisionWithSprite(a, b pkgengine.Object, _ float64, _ bool) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Broad phase checked overlap; ghost does not affect sensing.
	if !m.visible[a] || !m.visible[b] {
		return false
	}
	if m.ghost[a] == 1 || m.ghost[b] == 1 {
		m.sensedGhost = true
	}
	return true
}

func TestShortLivedGroundClonesRemainTouchableAcrossFrames(t *testing.T) {
	for _, front := range []string{"Player", "Ground"} {
		t.Run(front+"InFront", func(t *testing.T) {
			co, game := setupRuntimeEventGame(t)
			game.initShapeMgr()
			game.displayState.WorldWidth, game.displayState.WorldHeight = 480, 360
			mgr := &cloneCollisionSpriteMgr{
				spyCloneSpriteMgr: setupCloneSpriteMgr(t),
				visible:           make(map[pkgengine.Object]bool),
				ghost:             make(map[pkgengine.Object]float64),
			}
			pkgengine.SpriteMgr = mgr
			itime.Start(nil)

			ground := newCloneLimitSprite(game, "Ground")
			player := newCloneLimitSprite(game, "Player")
			if front == "Ground" {
				game.shapeMgr.items[0], game.shapeMgr.items[1] = game.shapeMgr.items[1], game.shapeMgr.items[0]
			}
			for _, sprite := range []*cloneLimitSprite{ground, player} {
				sprite.runtimeState.Scale = 1
				sprite.Show()
				sprite.initRuntimeProxy()
			}
			player.SetXYpos(100, 0)
			if player.Touching__1("Ground") {
				t.Fatal("the original Ground must not overlap Player")
			}

			created, deleted := 0, 0
			ground.onClone = func(clone *cloneLimitSprite) {
				clone.SetXYpos(100, 0)
				clone.SetGraphicEffect(GhostEffect, 100)
				created++
				game.Wait(0.01)
				deleted++
				clone.DeleteThisClone()
			}
			ground.OnStart(func() { ground.Clone__0() })
			ground.OnStart(func() { Forever(func() { ground.Clone__0() }) })
			var samples, missedFrames []int64
			player.OnStart(func() {
				Forever(func() {
					frame := itime.Frame()
					touching := player.Touching__1("Ground")
					if frame > 0 {
						samples = append(samples, frame)
						if !touching {
							missedFrames = append(missedFrames, frame)
						}
					}
					engine.RequestRedraw()
				})
			})

			game.handleEvent(&eventStart{generation: game.bootstrapGeneration()})
			const frames = 12
			for range frames {
				co.Update()
				flushCloneProxyUpdates(game)
				itime.Update(1.0/30, 30)
			}
			if len(samples) != frames-1 {
				t.Fatalf("collision samples = %v, want one per frame after startup", samples)
			}
			if len(missedFrames) != 0 {
				t.Errorf("Player missed Ground clones at frames %v", missedFrames)
			}
			if created < frames || deleted < frames-1 {
				t.Errorf("created/deleted clones = %d/%d, want continuous replacement", created, deleted)
			}
			if !mgr.sensedGhost {
				t.Error("collision queries never sensed a clone with GhostEffect 100")
			}
		})
	}
}
