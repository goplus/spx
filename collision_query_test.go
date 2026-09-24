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
	"github.com/goplus/spx/v3/internal/base/collision"
	pkgengine "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

func newTouchingPair(t *testing.T, physics bool) (*Game, *touchingSyncSpriteMgr, *touchingQuerySprite, *touchingQuerySprite) {
	t.Helper()
	mgr := newTouchingSyncSpriteMgr()
	installTouchingSyncSpriteMgr(t, mgr)
	game := &Game{physicsEnabled: physics}
	game.initShapeMgr()
	receiver := newTouchingQuerySprite("receiver", 0, 0, 1)
	target := newTouchingQuerySprite("target", 0, 0, 2)
	receiver.g, target.g = game, game
	game.addShape(&receiver.SpriteImpl)
	game.addShape(&target.SpriteImpl)
	mgr.positions[1] = mathf.NewVec2(0, 0)
	mgr.positions[2] = mathf.NewVec2(0, 0)
	return game, mgr, receiver, target
}

func TestTouchingNameReadsCurrentState(t *testing.T) {
	game, mgr, receiver, target := newTouchingPair(t, false)
	target.SetXYpos(10, 0)
	for range 2 {
		if receiver.Touching__1("target") {
			t.Fatal("far target touched receiver")
		}
	}
	if mgr.collisionChecks != 0 {
		t.Fatalf("far target caused %d native checks", mgr.collisionChecks)
	}

	target.SetXYpos(0, 0)
	if !receiver.Touching__1("target") {
		t.Fatal("same-frame move was missed")
	}
	if got := mgr.positions[2]; got != (mathf.Vec2{}) {
		t.Fatalf("native target position = %v, want origin", got)
	}
	target.Hide()
	if receiver.Touching__1("target") {
		t.Fatal("hidden target touched receiver")
	}
	target.Show()
	if !receiver.Touching__1("target") {
		t.Fatal("same-frame show was missed")
	}
	game.removeShape(&target.SpriteImpl)
	if receiver.Touching__1("target") {
		t.Fatal("removed target touched receiver")
	}
	game.addShape(&target.SpriteImpl)
	if !receiver.Touching__1("target") {
		t.Fatal("re-added target was missed")
	}
}

func TestTouchingNameUsesRotatedCostume(t *testing.T) {
	_, mgr, receiver, target := newTouchingPair(t, false)
	receiver.SetXYpos(4, 0)
	target.costumes = []*costume{
		{name: "small", width: 2, height: 10, bitmapResolution: 1},
		{name: "wide", width: 10, height: 2, bitmapResolution: 1},
	}
	target.transform().direction = 90
	mgr.collisionResult = func(pkgengine.Object, pkgengine.Object) bool { return true }
	if receiver.Touching__1("target") {
		t.Fatal("unrotated narrow costume touched receiver")
	}
	target.transform().direction = 0
	target.markProxyDirty()
	if !receiver.Touching__1("target") {
		t.Fatal("rotated costume overlap was culled")
	}
	target.transform().direction = 90
	target.markProxyDirty()
	if receiver.Touching__1("target") {
		t.Fatal("unrotated costume unexpectedly touched receiver")
	}
	target.SetCostume__0("wide")
	if !receiver.Touching__1("target") {
		t.Fatal("same-frame costume enlargement was missed")
	}
	target.SetCostume__0("small")
	target.SetSize(5)
	if !receiver.Touching__1("target") {
		t.Fatal("same-frame scale change was missed")
	}
}

func TestTouchingNameUsesPhysicsShape(t *testing.T) {
	_, mgr, receiver, target := newTouchingPair(t, true)
	receiver.physics().collisionInfo = physicConfig{Type: physicsColliderRect, Params: []float64{1, 1}}
	target.physics().collisionInfo = physicConfig{Type: physicsColliderRect, Params: []float64{2, 10}}
	receiver.SetXYpos(4, 0)
	target.transform().direction = 90
	mgr.collisionResult = func(pkgengine.Object, pkgengine.Object) bool { return true }
	if receiver.Touching__1("target") {
		t.Fatal("unrotated collider touched receiver")
	}
	target.transform().direction = 0
	target.markProxyDirty()
	if !receiver.Touching__1("target") {
		t.Fatal("rotated collider overlap was culled")
	}
	target.physics().collisionInfo = physicConfig{
		Type:   physicsColliderPolygon,
		Params: []float64{-5, -1, 5, -1, 0, 1},
	}
	target.transform().direction = 90
	target.markProxyDirty()
	if !receiver.Touching__1("target") {
		t.Fatal("polygon overlap was culled")
	}
}

func TestTouchingNameFallsBackWithoutCostume(t *testing.T) {
	for _, tc := range []struct {
		name         string
		breakCostume func(*touchingQuerySprite)
	}{
		{"missing", func(sp *touchingQuerySprite) { sp.costumes = nil }},
		{"negative index", func(sp *touchingQuerySprite) { sp.costumeIndex = -1 }},
		{"past end", func(sp *touchingQuerySprite) { sp.costumeIndex = len(sp.costumes) }},
		{"nil costume", func(sp *touchingQuerySprite) { sp.costumes[0] = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, mgr, receiver, target := newTouchingPair(t, true)
			target.physics().collisionInfo = physicConfig{Type: physicsColliderRect, Params: []float64{2, 2}}
			target.runtimeState.IsCostumeDirty = false
			tc.breakCostume(target)
			mgr.collisionResult = func(pkgengine.Object, pkgengine.Object) bool { return true }
			if _, bounded := target.queryBounds(false); bounded {
				t.Fatal("invalid costume produced a query bound")
			}
			if !receiver.Touching__1("target") {
				t.Fatal("native collision fallback was skipped")
			}
		})
	}
}

func TestPhysicsQueryBoundsRespectRotationStyle(t *testing.T) {
	sp := newTouchingQuerySprite("target", 100, 50, 2)
	sp.runtimeState.Scale = 2
	sp.physics().collisionInfo = physicConfig{
		Type:   physicsColliderRect,
		Pivot:  mathf.NewVec2(10, 2),
		Params: []float64{2, 4},
	}
	for _, tc := range []struct {
		name  string
		style RotationStyle
		want  collision.AABB
	}{
		{"left right", LeftRight, collision.AABB{MinX: 78, MinY: 50, MaxX: 82, MaxY: 58}},
		{"none", None, collision.AABB{MinX: 118, MinY: 50, MaxX: 122, MaxY: 58}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sp.transform().rotationStyle = tc.style
			sp.transform().direction = -30
			got, bounded := sp.queryBounds(false)
			if !bounded || got != tc.want {
				t.Fatalf("physics query bounds = %+v, %t; want %+v, true", got, bounded, tc.want)
			}
		})
	}
}

func TestTouchingNameSeesCloneImmediately(t *testing.T) {
	game, mgr, receiver, target := newTouchingPair(t, false)
	target.SetXYpos(20, 0)
	clone := newTouchingQuerySprite("target", 0, 0, 3)
	clone.g = game
	clone.spriteState.Cloned = true
	mgr.positions[3] = mathf.NewVec2(0, 0)
	game.addShape(&clone.SpriteImpl)
	if !receiver.Touching__1("target") {
		t.Fatal("new clone was missed")
	}
	game.removeShape(&clone.SpriteImpl)
	if receiver.Touching__1("target") {
		t.Fatal("removed clone still touched receiver")
	}
}

func TestTouchingNameFollowsShapeOrder(t *testing.T) {
	game, mgr, receiver, target := newTouchingPair(t, false)
	mgr.collisionResult = func(pkgengine.Object, pkgengine.Object) bool { return true }
	first := func(want *SpriteImpl) {
		t.Helper()
		if got := game.touchingSpriteBy(&receiver.SpriteImpl, "target"); got != want {
			t.Fatalf("first touching sprite = %p, want %p", got, want)
		}
	}
	first(&target.SpriteImpl)
	clone := newTouchingQuerySprite("target", 0, 0, 3)
	clone.g = game
	clone.spriteState.Cloned = true
	game.addClonedShape(&target.SpriteImpl, &clone.SpriteImpl)
	first(&clone.SpriteImpl)
	game.activateShape(&clone.SpriteImpl)
	first(&target.SpriteImpl)
	game.goBackLayers(&target.SpriteImpl, -1)
	first(&clone.SpriteImpl)
	game.removeShape(&target.SpriteImpl)
	first(&clone.SpriteImpl)
	game.shapeMgr.init()
	if game.shapeMgr.named != nil {
		t.Fatal("scene reset retained the name index")
	}
	first(nil)
	game.shapeMgr.spritesNamed("missing-a")
	game.shapeMgr.spritesNamed("missing-b")
	if game.shapeMgr.named != nil {
		t.Fatal("empty scene allocated a name index")
	}
}

func BenchmarkTouchingName(b *testing.B) {
	for _, tc := range []struct {
		name    string
		start   int
		rebuild bool
	}{
		{"none", 300, false},
		{"first", 0, false},
		{"last", 299, false},
		{"all", -1, false},
		{"none/rebuild", 300, true},
		{"last/rebuild", 299, true},
	} {
		b.Run(tc.name, func(b *testing.B) {
			mgr := newTouchingSyncSpriteMgr()
			installTouchingSyncSpriteMgr(b, mgr)
			game := &Game{}
			game.initShapeMgr()
			receiver := newTouchingQuerySprite("receiver", 0, 0, 1)
			receiver.g = game
			game.addShape(&receiver.SpriteImpl)
			mgr.positions[1] = mathf.NewVec2(0, 0)
			for i := range 300 {
				name := "other"
				if i == tc.start || tc.start == -1 {
					name = "target"
				}
				target := newTouchingQuerySprite(name, float64(1000+i*10), 0, int64(i+2))
				target.g = game
				game.addShape(&target.SpriteImpl)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if tc.rebuild {
					clear(game.shapeMgr.named)
				}
				if receiver.Touching__1("target") {
					b.Fatal("unexpected collision")
				}
			}
		})
	}
}

func TestTouchingNameUsesMinimumCircleRadius(t *testing.T) {
	for _, tc := range []struct {
		name          string
		radius, scale float64
	}{
		{"small radius", 0.001, 1},
		{"small scale", 1, 0.001},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, mgr, receiver, target := newTouchingPair(t, true)
			for _, sp := range []*touchingQuerySprite{receiver, target} {
				sp.physics().collisionInfo = physicConfig{Type: physicsColliderCircle, Params: []float64{tc.radius}}
				sp.runtimeState.Scale = tc.scale
			}
			target.SetXYpos(0.015, 0)
			mgr.collisionResult = func(pkgengine.Object, pkgengine.Object) bool { return true }
			if !receiver.Touching__1("target") {
				t.Fatal("minimum-radius circle overlap was culled")
			}
		})
	}
}
