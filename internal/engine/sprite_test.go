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

package engine

import (
	"reflect"
	"slices"
	"testing"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v4/internal/engine/wrap"
	gdx "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

type positionWriteSpy struct {
	gdx.ISpriteMgr
	position mathf.Vec2
	fail     bool
	writes   int
}

type textureUpdateCall struct {
	method string
	id     gdx.Object
	path   string
	rect   mathf.Rect2
	scale  mathf.Vec2
}

type textureUpdateSpy struct {
	gdx.ISpriteMgr
	calls []textureUpdateCall
}

type colliderCall struct {
	kind         string
	id           Object
	center, size mathf.Vec2
	radius       float64
	points       gdx.Array
	enabled      bool
}

type colliderSpy struct {
	gdx.ISpriteMgr
	calls []colliderCall
}

type factorySprite struct {
	gdx.Sprite
	started bool
}

type factoryUI struct {
	gdx.UiNode
	started bool
}

type factorySpriteManager struct {
	gdx.ISpriteMgr
	path     string
	position mathf.Vec2
}

type factoryUIManager struct {
	gdx.IUiMgr
	path           string
	parent, result Object
}

func TestSpritePositionWritesMirrorAfterEngineWrite(t *testing.T) {
	setupSpriteManagerTest(t)
	for _, test := range []struct {
		name  string
		write func(*Sprite)
		want  mathf.Vec2
		axes  PositionAxes
	}{
		{"set position", func(s *Sprite) { var sprite gdx.ISpriter = s; sprite.SetPosition(mathf.NewVec2(30, 40)) }, mathf.NewVec2(30, 40), PositionXY},
		{"add position", func(s *Sprite) { s.AddPos(3, 4) }, mathf.NewVec2(13, 24), PositionXY},
		{"add x", func(s *Sprite) { s.AddPosX(3) }, mathf.NewVec2(13, 20), PositionX},
		{"add y", func(s *Sprite) { s.AddPosY(4) }, mathf.NewVec2(10, 24), PositionY},
		{"set x", func(s *Sprite) { s.SetPosX(30) }, mathf.NewVec2(30, 20), PositionX},
		{"set y", func(s *Sprite) { s.SetPosY(40) }, mathf.NewVec2(10, 40), PositionY},
		{"set same position", func(s *Sprite) { s.SetPosition(mathf.NewVec2(10, 20)) }, mathf.NewVec2(10, 20), PositionXY},
		{"add zero position", func(s *Sprite) { s.AddPos(0, 0) }, mathf.NewVec2(10, 20), PositionXY},
		{"add zero x", func(s *Sprite) { s.AddPosX(0) }, mathf.NewVec2(10, 20), PositionX},
		{"add zero y", func(s *Sprite) { s.AddPosY(0) }, mathf.NewVec2(10, 20), PositionY},
		{"set same x", func(s *Sprite) { s.SetPosX(10) }, mathf.NewVec2(10, 20), PositionX},
		{"set same y", func(s *Sprite) { s.SetPosY(20) }, mathf.NewVec2(10, 20), PositionY},
	} {
		t.Run(test.name, func(t *testing.T) {
			manager := &positionWriteSpy{position: mathf.NewVec2(10, 20)}
			gdx.SpriteMgr = manager
			sprite := &Sprite{Sprite: gdx.Sprite{Id: 1}}
			calls := 0
			sprite.MirrorPosition = func(proxy *Sprite, position mathf.Vec2, axes PositionAxes) {
				calls++
				if proxy != sprite || position != test.want || manager.position != position || manager.writes != 1 {
					t.Fatalf("mirror observed proxy=%p position=%v engine=%v writes=%d", proxy, position, manager.position, manager.writes)
				}
				if axes != test.axes {
					t.Fatalf("mirror observed axes=%v, want %v", axes, test.axes)
				}
			}
			test.write(sprite)
			if calls != 1 {
				t.Fatalf("mirror notifications=%d, want 1", calls)
			}
		})
	}
}

func TestSpritePositionWriteFailureDoesNotMirror(t *testing.T) {
	setupSpriteManagerTest(t)
	manager := &positionWriteSpy{fail: true}
	gdx.SpriteMgr = manager
	called := false
	sprite := &Sprite{MirrorPosition: func(*Sprite, mathf.Vec2, PositionAxes) { called = true }}
	func() {
		defer func() {
			if got := recover(); got != "position write failed" {
				t.Fatalf("position write panic=%v", got)
			}
		}()
		sprite.SetPosition(mathf.NewVec2(30, 40))
	}()
	if called {
		t.Fatal("failed engine write changed the mirrored position")
	}
	manager.fail = false
	sprite.MirrorPosition = nil
	sprite.SetPosition(mathf.NewVec2(30, 40))
	if manager.position != mathf.NewVec2(30, 40) || manager.writes != 1 {
		t.Fatal("sprite without a mirror lost its engine write")
	}
}

func TestTweenMirrorsProxyPositionBeforeCompletionCallback(t *testing.T) {
	setupSpriteManagerTest(t)
	resetFrameTestState()
	t.Cleanup(resetFrameTestState)
	manager := &positionWriteSpy{}
	gdx.SpriteMgr = manager
	var cached mathf.Vec2
	sprite := &Sprite{Sprite: gdx.Sprite{Id: 1}, MirrorPosition: func(_ *Sprite, position mathf.Vec2, axes PositionAxes) {
		if axes != PositionXY {
			t.Fatalf("tween notified axes=%v, want %v", axes, PositionXY)
		}
		cached = position
	}}
	state.sprites[sprite.Id] = sprite
	completed := false
	want := mathf.NewVec2(30, 40)
	tweenPos(sprite, want, 1, func() {
		completed = true
		if cached != want || manager.position != want {
			t.Fatalf("tween completion observed cache=%v engine=%v, want %v", cached, manager.position, want)
		}
	})
	advanceTweenTest(0.5)
	if cached != want.Mulf(0.5) || manager.position != cached || completed {
		t.Fatalf("tween midpoint: cache=%v engine=%v completed=%v", cached, manager.position, completed)
	}
	advanceTweenTest(0.5)
	if !completed {
		t.Fatal("tween did not invoke its completion callback")
	}
}

func TestSpriteTextureUpdates(t *testing.T) {
	setupSpriteManagerTest(t)
	previousPaths := assetPaths
	t.Cleanup(func() { assetPaths = previousPaths })
	assetPaths = assetResolver{root: "assets/", projectRoot: "."}

	const id = gdx.Object(42)
	const renderScale = 1.5
	rect := mathf.Rect2{Position: mathf.NewVec2(3, 5), Size: mathf.NewVec2(7, 11)}
	sprite := &Sprite{Sprite: gdx.Sprite{Id: id}}
	assetPath := ToAssetPath("sprites/cat.png")
	scaleCall := textureUpdateCall{method: "scale", id: id, scale: mathf.NewVec2(renderScale, renderScale)}
	for _, update := range []struct {
		name string
		call func(string, float64, bool)
		rect mathf.Rect2
	}{
		{name: "texture", call: sprite.UpdateTexture},
		{name: "atlas", rect: rect, call: func(path string, scale float64, update bool) {
			sprite.UpdateTextureAtlas(path, rect, scale, update)
		}},
	} {
		t.Run(update.name, func(t *testing.T) {
			textureCall := textureUpdateCall{method: update.name, id: id, path: assetPath, rect: update.rect}
			invalidPathCall := textureUpdateCall{method: update.name, id: id, rect: update.rect}
			for _, tt := range []struct {
				name          string
				path          string
				updateTexture bool
				want          []textureUpdateCall
			}{
				{name: "update", path: "sprites/cat.png", updateTexture: true, want: []textureUpdateCall{textureCall, scaleCall}},
				{name: "scale only", path: "sprites/cat.png", want: []textureUpdateCall{scaleCall}},
				{name: "invalid path", path: "https://example.com/cat.png", updateTexture: true, want: []textureUpdateCall{invalidPathCall, scaleCall}},
				{name: "invalid path scale only", path: "https://example.com/cat.png", want: []textureUpdateCall{scaleCall}},
				{name: "empty path", updateTexture: true},
				{name: "empty path scale only"},
			} {
				t.Run(tt.name, func(t *testing.T) {
					spy := &textureUpdateSpy{}
					gdx.SpriteMgr = spy
					update.call(tt.path, renderScale, tt.updateTexture)

					if !slices.Equal(spy.calls, tt.want) {
						t.Fatalf("engine calls = %+v, want %+v", spy.calls, tt.want)
					}
				})
			}
		})
	}
}

func TestSpriteColliderMethodsSelectTriggerAndPreserveCoordinates(t *testing.T) {
	setupSpriteManagerTest(t)
	manager := &colliderSpy{}
	gdx.SpriteMgr = manager
	sprite := &Sprite{Sprite: gdx.Sprite{Id: 42}}
	center, size := mathf.NewVec2(-3, 5), mathf.NewVec2(8, 10)
	for _, trigger := range []bool{false, true} {
		manager.calls = nil
		sprite.SetColliderShapeRect(trigger, center, size)
		sprite.SetColliderShapeCircle(trigger, center, 2.5)
		sprite.SetColliderShapeCapsule(trigger, center, size)
		sprite.SetColliderShapePolygon(trigger, center, []float64{-1.5, 2.25})
		sprite.SetColliderEnabled(trigger, true)
		prefix, enabled := "Collider", "CollisionEnabled"
		if trigger {
			prefix, enabled = "Trigger", "TriggerEnabled"
		}
		want := []colliderCall{{kind: prefix + "Rect", id: 42, center: center, size: size}, {kind: prefix + "Circle", id: 42, center: center, radius: 2.5}, {kind: prefix + "Capsule", id: 42, center: center, size: size}, {kind: prefix + "Polygon", id: 42, center: center, points: []float32{-1.5, 2.25}}, {kind: enabled, id: 42, enabled: true}}
		if !reflect.DeepEqual(manager.calls, want) {
			t.Fatalf("trigger=%v calls=%#v, want %#v", trigger, manager.calls, want)
		}
	}
}

func TestSpriteAnimationSubscriptionsCanBeRemoved(t *testing.T) {
	sprite := &Sprite{Sprite: gdx.Sprite{OnAnimationLoopedEvent: gdx.NewEvent0(), OnAnimationFinishedEvent: gdx.NewEvent0()}}
	looped, finished := 0, 0
	sprite.RegisterOnAnimationLooped(func() { looped++ })
	sprite.RegisterOnAnimationFinished(func() { finished++ })
	sprite.V_OnAnimationLooped()
	sprite.V_OnAnimationFinished()
	sprite.UnRegisterOnAnimationLooped()
	sprite.UnRegisterOnAnimationFinished()
	sprite.V_OnAnimationLooped()
	sprite.V_OnAnimationFinished()
	if looped != 1 || finished != 1 {
		t.Fatalf("animation callbacks=%d/%d", looped, finished)
	}
}

func TestRuntimeFactoriesRegisterBeforeStart(t *testing.T) {
	setupSpriteManagerTest(t)
	previous, previousUI, previousTypes := state, gdx.UiMgr, spriteTypes
	t.Cleanup(func() { state, gdx.UiMgr, spriteTypes = previous, previousUI, previousTypes })
	state = runtimeState{sprites: make(map[Object]gdx.ISpriter), uiNodes: make(map[Object]gdx.IUiNode)}
	spriteTypes = make(map[string]reflect.Type)
	(runtimeBridge{}).RegisterSpriteType(reflect.TypeFor[factorySprite]())
	sprites := &factorySpriteManager{}
	ui := &factoryUIManager{result: 20}
	gdx.SpriteMgr, gdx.UiMgr = sprites, ui
	position := mathf.NewVec2(3, -5)
	for _, tc := range []struct {
		name   string
		create func() *factorySprite
		id     Object
	}{
		{"prefab", func() *factorySprite {
			return (runtimeBridge{}).CreateSprite(reflect.TypeFor[factorySprite](), position).Addr().Interface().(*factorySprite)
		}, 10},
		{"bare", func() *factorySprite { return CreateBareSpriteForType[factorySprite](position) }, 11},
		{"backdrop", CreateBackdropForType[factorySprite], 12},
		{"bare bridge", func() *factorySprite {
			return (runtimeBridge{}).CreateEmptySprite(reflect.TypeFor[factorySprite](), position).Addr().Interface().(*factorySprite)
		}, 11},
		{"backdrop bridge", func() *factorySprite {
			return (runtimeBridge{}).CreateBackdrop(reflect.TypeFor[factorySprite]()).Addr().Interface().(*factorySprite)
		}, 12},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sprite := tc.create()
			if sprite.GetId() != tc.id || !sprite.started || sprite.OnTriggerEnterEvent == nil {
				t.Fatalf("uninitialized sprite: %#v", sprite)
			}
			if tc.id != 12 && sprites.position != position {
				t.Fatalf("creation position = %v", sprites.position)
			}
			if tc.name == "prefab" && sprites.path != "res://assets/prefabs/factorySprite.tscn" {
				t.Fatalf("prefab path = %q", sprites.path)
			}
			if tc.id == 12 && sprites.path != "" {
				t.Fatalf("backdrop path = %q", sprites.path)
			}
		})
	}
	for _, tc := range []struct {
		name   string
		create func() *factoryUI
		path   string
		parent Object
	}{
		{"engine default", func() *factoryUI { return CreateEngineUIForType[factoryUI]("") }, "res://engine/ui/factoryUI.tscn", 0},
		{"engine override", func() *factoryUI { return CreateEngineUIForType[factoryUI]("Panel") }, "res://engine/ui/Panel.tscn", 0},
		{"project UI", func() *factoryUI {
			return (runtimeBridge{}).CreateUI(reflect.TypeFor[factoryUI](), "Panel", false).Addr().Interface().(*factoryUI)
		}, "res://assets/ui/Panel.tscn", 0},
		{"bind public", func() *factoryUI { return BridgeBindUI[factoryUI](30, "Child") }, "Child", 30},
		{"bind", func() *factoryUI {
			return (runtimeBridge{}).BindUI(reflect.TypeFor[factoryUI](), 30, "Child").Addr().Interface().(*factoryUI)
		}, "Child", 30},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ui.parent, ui.path = 0, ""
			node := tc.create()
			if node.GetId() != 20 || !node.started || node.OnUiClickEvent == nil || ui.path != tc.path {
				t.Fatalf("node=%#v, path=%q, want %q", node, ui.path, tc.path)
			}
			if ui.parent != tc.parent {
				t.Fatalf("bind parent = %d, want %d", ui.parent, tc.parent)
			}
		})
	}
	ui.result = 0
	if got := BindUIForType[factoryUI](30, "missing"); got != nil || len(state.uiNodes) != 1 {
		t.Fatalf("failed binding registered a node: %v, %v", got, state.uiNodes)
	}
	(runtimeBridge{}).BindSceneInstantiatedSprite(40, "factorySprite")
	if sprite, ok := state.sprites[40].(*factorySprite); !ok || !sprite.started {
		t.Fatalf("scene sprite = %#v", state.sprites[40])
	}
	(runtimeBridge{}).BindSceneInstantiatedSprite(41, "missing")
	if _, ok := state.sprites[41]; ok {
		t.Fatal("unknown sprite type was registered")
	}
}

func (s *positionWriteSpy) GetPosition(Object) mathf.Vec2 { return s.position }

func (s *positionWriteSpy) SetPosition(_ Object, position mathf.Vec2) {
	if s.fail {
		panic("position write failed")
	}
	s.position = position
	s.writes++
}

func (s *textureUpdateSpy) SetTexture(id gdx.Object, path string) {
	s.calls = append(s.calls, textureUpdateCall{method: "texture", id: id, path: path})
}

func (s *textureUpdateSpy) SetTextureAtlas(id gdx.Object, path string, rect mathf.Rect2) {
	s.calls = append(s.calls, textureUpdateCall{method: "atlas", id: id, path: path, rect: rect})
}

func (s *textureUpdateSpy) SetRenderScale(id gdx.Object, scale mathf.Vec2) {
	s.calls = append(s.calls, textureUpdateCall{method: "scale", id: id, scale: scale})
}

func (m *colliderSpy) SetColliderRect(id Object, center, size mathf.Vec2) {
	m.calls = append(m.calls, colliderCall{kind: "ColliderRect", id: id, center: center, size: size})
}

func (m *colliderSpy) SetColliderCapsule(id Object, center, size mathf.Vec2) {
	m.calls = append(m.calls, colliderCall{kind: "ColliderCapsule", id: id, center: center, size: size})
}

func (m *colliderSpy) SetColliderCircle(id Object, center mathf.Vec2, radius float64) {
	m.calls = append(m.calls, colliderCall{kind: "ColliderCircle", id: id, center: center, radius: radius})
}

func (m *colliderSpy) SetColliderPolygon(id Object, center mathf.Vec2, points gdx.Array) {
	m.calls = append(m.calls, colliderCall{kind: "ColliderPolygon", id: id, center: center, points: points})
}

func (m *colliderSpy) SetTriggerRect(id Object, center, size mathf.Vec2) {
	m.calls = append(m.calls, colliderCall{kind: "TriggerRect", id: id, center: center, size: size})
}

func (m *colliderSpy) SetTriggerCapsule(id Object, center, size mathf.Vec2) {
	m.calls = append(m.calls, colliderCall{kind: "TriggerCapsule", id: id, center: center, size: size})
}

func (m *colliderSpy) SetTriggerCircle(id Object, center mathf.Vec2, radius float64) {
	m.calls = append(m.calls, colliderCall{kind: "TriggerCircle", id: id, center: center, radius: radius})
}

func (m *colliderSpy) SetTriggerPolygon(id Object, center mathf.Vec2, points gdx.Array) {
	m.calls = append(m.calls, colliderCall{kind: "TriggerPolygon", id: id, center: center, points: points})
}

func (m *colliderSpy) SetCollisionEnabled(id Object, enabled bool) {
	m.calls = append(m.calls, colliderCall{kind: "CollisionEnabled", id: id, enabled: enabled})
}

func (m *colliderSpy) SetTriggerEnabled(id Object, enabled bool) {
	m.calls = append(m.calls, colliderCall{kind: "TriggerEnabled", id: id, enabled: enabled})
}

func (s *factorySprite) OnStart() { s.started = state.sprites[s.GetId()] == s }

func (n *factoryUI) OnStart() { n.started = state.uiNodes[n.GetId()] == n }

func (m *factorySpriteManager) CreateSprite(path string, position mathf.Vec2) Object {
	m.path, m.position = path, position
	return 10
}

func (m *factorySpriteManager) CreateBareSprite(position mathf.Vec2) Object {
	m.position = position
	return 11
}

func (m *factorySpriteManager) CreateBackdrop(path string) Object {
	m.path = path
	return 12
}

func (m *factoryUIManager) CreateNode(path string) Object { m.path = path; return m.result }

func (m *factoryUIManager) BindNode(parent Object, path string) Object {
	m.parent, m.path = parent, path
	return m.result
}

func setupSpriteManagerTest(t *testing.T) {
	t.Helper()
	previous := gdx.SpriteMgr
	wrap.Init(func(call func()) { call() })
	t.Cleanup(func() {
		gdx.SpriteMgr = previous
		wrap.Init(WaitMainThread)
	})
}
