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
	"slices"
	"testing"

	"github.com/goplus/spbase/mathf"
	internalengine "github.com/goplus/spx/v3/internal/engine"
	"github.com/goplus/spx/v3/internal/enginewrap"
	pkgengine "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

type pendingQueryTestSprite struct {
	SpriteImpl
}

func (*pendingQueryTestSprite) Main() {}

type pendingQuerySpriteMgr struct {
	enginewrap.SpriteMgrImpl
	nextID pkgengine.Object
}

func (m *pendingQuerySpriteMgr) CreateBareSprite(mathf.Vec2) pkgengine.Object {
	m.nextID++
	return m.nextID
}

type pendingQueryPhysicsMgr struct {
	enginewrap.PhysicsMgrImpl
	rectHits       []pkgengine.Object
	circleHits     []pkgengine.Object
	raycastOrder   []pkgengine.Object
	stickyRaycast  pkgengine.Object
	raycastIgnores [][]int64
}

func (m *pendingQueryPhysicsMgr) CheckCollisionRect(mathf.Vec2, mathf.Vec2, int64) pkgengine.Array {
	return append([]pkgengine.Object(nil), m.rectHits...)
}

func (m *pendingQueryPhysicsMgr) CheckCollisionCircle(mathf.Vec2, float64, int64) pkgengine.Array {
	return append([]pkgengine.Object(nil), m.circleHits...)
}

func (m *pendingQueryPhysicsMgr) RaycastWithDetails(
	_, _ mathf.Vec2,
	ignoreSprites pkgengine.Array,
	_ int64,
	_, _ bool,
) pkgengine.Array {
	ignored, _ := ignoreSprites.([]int64)
	ignored = append([]int64(nil), ignored...)
	m.raycastIgnores = append(m.raycastIgnores, ignored)

	if m.stickyRaycast != 0 {
		return pendingQueryRaycastHit(m.stickyRaycast)
	}
	for _, id := range m.raycastOrder {
		if !slices.Contains(ignored, int64(id)) {
			return pendingQueryRaycastHit(id)
		}
	}
	return []int64{0, 0, 0, 0, 0, 0}
}

func pendingQueryRaycastHit(id pkgengine.Object) []int64 {
	return []int64{
		1,
		int64(id),
		internalengine.ConvertToInt64(12.5),
		internalengine.ConvertToInt64(-7.25),
		0,
		0,
	}
}

func installPendingQueryManagers(
	t *testing.T,
	physicsMgr *pendingQueryPhysicsMgr,
) *pendingQuerySpriteMgr {
	t.Helper()

	enginewrap.Init(func(call func()) { call() })
	originalSpriteMgr := pkgengine.SpriteMgr
	originalPhysicsMgr := pkgengine.PhysicsMgr
	spriteMgr := &pendingQuerySpriteMgr{nextID: 900_000}
	pkgengine.SpriteMgr = spriteMgr
	pkgengine.PhysicsMgr = physicsMgr
	t.Cleanup(func() {
		pkgengine.SpriteMgr = originalSpriteMgr
		pkgengine.PhysicsMgr = originalPhysicsMgr
	})
	return spriteMgr
}

func newPendingQueryTestSprite(t *testing.T, pending bool) *pendingQueryTestSprite {
	t.Helper()

	sprite := &pendingQueryTestSprite{}
	sprite.g = &Game{}
	sprite.name = "query-test"
	sprite.sprite = sprite
	sprite.spriteState.IsVisible = true
	sprite.spriteState.IsProxyPublicationPending = pending
	sprite.runtimeState.SyncSprite = internalengine.BridgeNewBareSprite(&sprite.SpriteImpl, mathf.Vec2{})
	t.Cleanup(func() {
		pkgengine.DeleteSprite(sprite.getSpriteId())
	})
	return sprite
}

func TestGameIntersectionsExcludePendingCloneProxy(t *testing.T) {
	physicsMgr := &pendingQueryPhysicsMgr{}
	installPendingQueryManagers(t, physicsMgr)

	pending := newPendingQueryTestSprite(t, true)
	published := newPendingQueryTestSprite(t, false)
	// Visibility must not affect explicit physics queries; only publication does.
	published.spriteState.IsVisible = false
	physicsMgr.rectHits = []pkgengine.Object{pending.getSpriteId(), published.getSpriteId()}
	physicsMgr.circleHits = []pkgengine.Object{published.getSpriteId(), pending.getSpriteId()}

	game := &Game{}
	for name, got := range map[string][]Sprite{
		"rect":   game.IntersectRect(0, 0, 10, 10),
		"circle": game.IntersectCircle(0, 0, 10),
	} {
		if len(got) != 1 || got[0] != published {
			t.Errorf("%s intersection = %v, want only published sprite", name, got)
		}
	}
}

func TestGameRaycastSkipsPendingCloneAndHitsPublishedSpriteBehindIt(t *testing.T) {
	physicsMgr := &pendingQueryPhysicsMgr{}
	installPendingQueryManagers(t, physicsMgr)

	pending := newPendingQueryTestSprite(t, true)
	published := newPendingQueryTestSprite(t, false)
	physicsMgr.raycastOrder = []pkgengine.Object{pending.getSpriteId(), published.getSpriteId()}

	hit, target, hitX, hitY := (&Game{}).Raycast__0(0, 0, 100, 0)
	if !hit || target != published {
		t.Fatalf("Raycast = (%v, %v), want hit on published sprite", hit, target)
	}
	if hitX != 12.5 || hitY != -7.25 {
		t.Fatalf("Raycast position = (%v, %v), want (12.5, -7.25)", hitX, hitY)
	}
	if len(physicsMgr.raycastIgnores) != 2 {
		t.Fatalf("Raycast calls = %d, want 2", len(physicsMgr.raycastIgnores))
	}
	if !slices.Contains(physicsMgr.raycastIgnores[1], int64(pending.getSpriteId())) {
		t.Fatalf("second Raycast ignore list = %v, want pending sprite %d", physicsMgr.raycastIgnores[1], pending.getSpriteId())
	}
}

func TestGameRaycastStopsIfBackendReturnsIgnoredPendingClone(t *testing.T) {
	physicsMgr := &pendingQueryPhysicsMgr{}
	installPendingQueryManagers(t, physicsMgr)

	pending := newPendingQueryTestSprite(t, true)
	physicsMgr.stickyRaycast = pending.getSpriteId()

	hit, target, _, _ := (&Game{}).Raycast__0(0, 0, 100, 0)
	if hit || target != nil {
		t.Fatalf("Raycast = (%v, %v), want no published hit", hit, target)
	}
	if len(physicsMgr.raycastIgnores) != 2 {
		t.Fatalf("Raycast calls = %d, want 2 before repeated-hit guard", len(physicsMgr.raycastIgnores))
	}
}
