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
	coreproject "github.com/goplus/spx/v3/internal/core/project"
	internalengine "github.com/goplus/spx/v3/internal/engine"
	pkgengine "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

type gameQueryPhysicsMgr struct {
	pkgengine.IPhysicsMgr
	result        internalengine.Array
	from, to      mathf.Vec2
	radius        float64
	mask          int64
	ignore        []int64
	areas, bodies bool
}

func (m *gameQueryPhysicsMgr) CheckCollisionRect(pos, size mathf.Vec2, mask int64) internalengine.Array {
	m.from, m.to, m.mask = pos, size, mask
	return m.result
}

func (m *gameQueryPhysicsMgr) CheckCollisionCircle(pos mathf.Vec2, radius float64, mask int64) internalengine.Array {
	m.from, m.radius, m.mask = pos, radius, mask
	return m.result
}

func (m *gameQueryPhysicsMgr) RaycastWithDetails(from, to mathf.Vec2, ignore internalengine.Array, mask int64, areas, bodies bool) internalengine.Array {
	m.from, m.to, m.mask = from, to, mask
	m.ignore = ignore.([]int64)
	m.areas, m.bodies = areas, bodies
	return m.result
}

func setupGameQueryPhysics(t *testing.T) (*Game, *gameQueryPhysicsMgr, *targetLookupSprite) {
	t.Helper()
	setupImmediateEngineCalls(t)
	previous := pkgengine.PhysicsMgr
	manager := &gameQueryPhysicsMgr{}
	pkgengine.PhysicsMgr = manager
	t.Cleanup(func() { pkgengine.PhysicsMgr = previous })
	game := &Game{}
	sprite := newTargetLookupSprite(game, "target", false)
	const id = 89001
	proxy := &internalengine.Sprite{Sprite: pkgengine.Sprite{Id: id}, Target: &sprite.SpriteImpl}
	sprite.runtimeState.SyncSprite = proxy
	registry := pkgengine.Sprites()
	old, existed := registry[id]
	registry[id] = proxy
	t.Cleanup(func() {
		if existed {
			registry[id] = old
		} else {
			delete(registry, id)
		}
	})
	return game, manager, sprite
}

func TestGameIntersectionQueriesResolveLiveSprites(t *testing.T) {
	game, manager, sprite := setupGameQueryPhysics(t)
	// A missing engine object is ignored while the registered sprite is retained.
	manager.result = []internalengine.Object{-1, sprite.getSpriteId()}
	if got := game.IntersectRect(10, 20, 30, 40); !slices.Equal(got, []Sprite{sprite}) {
		t.Fatalf("IntersectRect = %v, want target sprite", got)
	}
	if manager.from != mathf.NewVec2(10, 20) || manager.to != mathf.NewVec2(30, 40) || manager.mask != -1 {
		t.Fatalf("rectangle query = %+v", manager)
	}
	if got := game.IntersectCircle(-10, -20, 15); !slices.Equal(got, []Sprite{sprite}) {
		t.Fatalf("IntersectCircle = %v, want target sprite", got)
	}
	if manager.from != mathf.NewVec2(-10, -20) || manager.radius != 15 || manager.mask != -1 {
		t.Fatalf("circle query = %+v", manager)
	}
	manager.result = []internalengine.Object{}
	if got := game.IntersectCircle(0, 0, 1); len(got) != 0 {
		t.Fatalf("empty query = %v", got)
	}
}

func TestGameRaycastResolvesTargetsAndIgnores(t *testing.T) {
	game, manager, sprite := setupGameQueryPhysics(t)
	manager.result = []int64{1, sprite.getSpriteId(), internalengine.ConvertToInt64(12.5), internalengine.ConvertToInt64(-7.25), 0, 0}
	for _, test := range []struct {
		name   string
		call   func() (bool, Sprite, float64, float64)
		ignore []int64
	}{
		{"default", func() (bool, Sprite, float64, float64) { return game.Raycast__0(1, 2, 3, 4) }, nil},
		{"single", func() (bool, Sprite, float64, float64) { return game.Raycast__1(1, 2, 3, 4, sprite) }, []int64{sprite.getSpriteId()}},
		{"multiple with nil", func() (bool, Sprite, float64, float64) { return game.Raycast__2(1, 2, 3, 4, []Sprite{nil, sprite}) }, []int64{sprite.getSpriteId()}},
	} {
		t.Run(test.name, func(t *testing.T) {
			hit, target, x, y := test.call()
			if !hit || target != sprite || x != 12.5 || y != -7.25 {
				t.Fatalf("raycast = %v, %v, %v, %v", hit, target, x, y)
			}
			if manager.from != mathf.NewVec2(1, 2) || manager.to != mathf.NewVec2(3, 4) || manager.mask != -1 || !manager.areas || !manager.bodies || !slices.Equal(manager.ignore, test.ignore) {
				t.Fatalf("raycast arguments = %+v", manager)
			}
		})
	}
	for _, test := range []struct {
		name   string
		result internalengine.Array
		hit    bool
	}{
		{"miss", make([]int64, 6), false},
		{"missing sprite", []int64{1, -1, 0, 0, 0, 0}, true},
		{"malformed result", []int64{1}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			manager.result = test.result
			hit, target, x, y := game.Raycast__0(1, 2, 3, 4)
			if hit != test.hit || target != nil || x != 0 || y != 0 {
				t.Fatalf("raycast = %v, %v, %v, %v", hit, target, x, y)
			}
		})
	}
}

type gameDebugDrawMgr struct {
	pkgengine.IDebugMgr
	lines  [][2]mathf.Vec2
	colors []mathf.Color
}

func (m *gameDebugDrawMgr) DebugDrawLine(from, to mathf.Vec2, color mathf.Color) {
	m.lines = append(m.lines, [2]mathf.Vec2{from, to})
	m.colors = append(m.colors, color)
}

func TestGameDebugDrawLinesValidatesCompleteSegments(t *testing.T) {
	setupImmediateEngineCalls(t)
	previous := pkgengine.DebugMgr
	manager := &gameDebugDrawMgr{}
	pkgengine.DebugMgr = manager
	t.Cleanup(func() { pkgengine.DebugMgr = previous })
	var game Game
	color := NewColor("#12345678")
	for _, points := range [][]float64{nil, {1, 2}, {1, 2, 3}, {1, 2, 3, 4, 5}} {
		game.DebugDrawLines(points, color)
	}
	if len(manager.lines) != 0 {
		t.Fatal("incomplete point pairs drew segments")
	}
	game.DebugDrawLines([]float64{1, 2, 3, 4, 5, 6}, color)
	want := [][2]mathf.Vec2{{mathf.NewVec2(1, 2), mathf.NewVec2(3, 4)}, {mathf.NewVec2(3, 4), mathf.NewVec2(5, 6)}}
	if !slices.Equal(manager.lines, want) {
		t.Fatalf("segments = %v, want %v", manager.lines, want)
	}
	for _, got := range manager.colors {
		if got != toMathfColor(color) {
			t.Fatalf("segment color = %v, want %v", got, toMathfColor(color))
		}
	}
}

func TestGameResetCollisionLayerStateClearsCollisionLayerState(t *testing.T) {
	var g Game
	g.sprCollisionInfos = map[string]*spriteCollisionInfo{
		"SpriteA": {Index: 0, Layer: 1 << 0, Mask: 1 << 1},
	}
	g.sprCollisionData = []*spriteCollisionData{{
		info: g.sprCollisionInfos["SpriteA"],
	}}

	g.resetCollisionLayerState()

	if g.sprCollisionInfos != nil {
		t.Fatalf("sprCollisionInfos = %#v, want nil", g.sprCollisionInfos)
	}
	if g.sprCollisionData != nil {
		t.Fatalf("sprCollisionData = %#v, want nil", g.sprCollisionData)
	}
}

func TestTryRaycastResult(t *testing.T) {
	for _, test := range []struct {
		name    string
		data    internalengine.Array
		want    rayCastResult
		wantErr string
	}{
		{
			name: "hit",
			data: []int64{
				1, 42,
				internalengine.ConvertToInt64(12.5), internalengine.ConvertToInt64(-7.25),
				internalengine.ConvertToInt64(0.6), internalengine.ConvertToInt64(0.8),
			},
			want: rayCastResult{Hited: true, SpriteId: 42, PosX: 12.5, PosY: -7.25},
		},
		{
			name: "negative hit marker",
			data: []int64{-2, 43, 0, 0, 0, 0},
			want: rayCastResult{Hited: true, SpriteId: 43},
		},
		{name: "no hit", data: make([]int64, 6)},
		{
			name: "no hit retains payload",
			data: []int64{0, 44, internalengine.ConvertToInt64(-5), internalengine.ConvertToInt64(9), 0, 0},
			want: rayCastResult{SpriteId: 44, PosX: -5, PosY: 9},
		},
		{name: "nil", wantErr: "array type error: expected []int64 but got <nil>"},
		{name: "wrong type", data: make([]float64, 6), wantErr: "array type error: expected []int64 but got []float64"},
		{name: "empty", data: []int64{}, wantErr: "array len error: expected 6 but got 0"},
		{name: "missing normal", data: make([]int64, 4), wantErr: "array len error: expected 6 but got 4"},
		{name: "short", data: make([]int64, 5), wantErr: "array len error: expected 6 but got 5"},
		{name: "long", data: make([]int64, 7), wantErr: "array len error: expected 6 but got 7"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := tryRaycastResult(test.data)
			if test.wantErr != "" {
				if got != nil || err == nil || err.Error() != test.wantErr {
					t.Fatalf("result = %+v, error = %v; want nil, %q", got, err, test.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got == nil || *got != test.want {
				t.Fatalf("raycast result = %+v, want %+v", got, test.want)
			}
		})
	}
}

func (m *gameDebugDrawMgr) DebugDrawRect(pos, size mathf.Vec2, color mathf.Color) {
	m.lines = append(m.lines, [2]mathf.Vec2{pos, size})
	m.colors = append(m.colors, color)
}
func (m *gameDebugDrawMgr) DebugDrawCircle(pos mathf.Vec2, radius float64, color mathf.Color) {
	m.lines = append(m.lines, [2]mathf.Vec2{pos, mathf.NewVec2(radius, radius)})
	m.colors = append(m.colors, color)
}
func TestGameDebugDrawingPrimitives(t *testing.T) {
	setupImmediateEngineCalls(t)
	previous := pkgengine.DebugMgr
	manager := &gameDebugDrawMgr{}
	pkgengine.DebugMgr = manager
	t.Cleanup(func() { pkgengine.DebugMgr = previous })
	var game Game
	color := NewColor("#123456")
	game.DebugDrawRect(1, 2, 3, 4, color)
	game.DebugDrawCircle(5, 6, 7, color)
	game.DebugDrawLine(8, 9, 10, 11, color)
	want := [][2]mathf.Vec2{{mathf.NewVec2(1, 2), mathf.NewVec2(3, 4)}, {mathf.NewVec2(5, 6), mathf.NewVec2(7, 7)}, {mathf.NewVec2(8, 9), mathf.NewVec2(10, 11)}}
	if !slices.Equal(manager.lines, want) {
		t.Fatalf("draw commands=%v", manager.lines)
	}
	for _, got := range manager.colors {
		if got != mathf.NewColorRGBAi(0x12, 0x34, 0x56, 0xff) {
			t.Fatalf("draw color=%v", got)
		}
	}
}
func TestGameCollisionSkipsInvalidSprites(t *testing.T) {
	game, _, sprite := setupGameQueryPhysics(t)
	if got := game.buildSpriteCollisionData([]Sprite{nil}); len(got) != 0 {
		t.Fatalf("nil sprite collision=%v", got)
	}
	sprite.runtimeState.SyncSprite.Target = (*SpriteImpl)(nil)
	if got := game.checkCollision([]internalengine.Object{sprite.getSpriteId()}); len(got) != 0 {
		t.Fatalf("nil target collision=%v", got)
	}
}

func TestApplyPhysicsSettingsForwardsCollisionMode(t *testing.T) {
	game := setupStageLoadTestGame(t, nil)
	mgr := pkgengine.PhysicsMgr.(*gameTestPhysicsMgr)
	want := []bool{true, false, true}
	for _, byPixel := range want {
		game.applyPhysicsSettings(coreproject.SystemSettings{CollisionByPixel: byPixel})
	}
	if !slices.Equal(mgr.collisionModes, want) {
		t.Fatalf("collision modes = %v, want %v", mgr.collisionModes, want)
	}
}
