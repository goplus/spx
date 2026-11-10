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

package engine

import (
	"fmt"
	"slices"
	"testing"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v4/internal/coroutine"
	gdx "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

type bridgeMainThreadPlatform struct{ gdx.IPlatformMgr }

type bridgeDestroySprite struct {
	gdx.ISpriter
	calls int
}

type bridgeDestroyNode struct {
	gdx.IUiNode
	calls int
}

type adapterResourceSpy struct {
	gdx.IResMgr
	files map[string]string
}

type adapterCameraSpy struct {
	gdx.ICameraMgr
	position, zoom mathf.Vec2
}

type adapterPlatformSpy struct {
	bridgeMainThreadPlatform
	debug bool
}

type adapterExtSpy struct {
	gdx.IExtMgr
	modes []int64
}

type backdropProxySpy struct {
	factorySpriteManager
	settings []string
	texture  string
	scale    mathf.Vec2
}

func TestManagersReturnsStableSingleton(t *testing.T) {
	first := Managers()
	if first == nil {
		t.Fatal("Managers() returned nil")
	}
	if got := Managers(); got != first {
		t.Fatalf("Managers() = %p, want stable singleton %p", got, first)
	}
}

func TestWindowScale(t *testing.T) {
	previous := WindowScale()
	t.Cleanup(func() { SetWindowScale(previous) })
	for _, scale := range []float64{0.5, 1, 2.5} {
		SetWindowScale(scale)
		if got := WindowScale(); got != scale {
			t.Fatalf("WindowScale = %v, want %v", got, scale)
		}
	}
}

func TestBackendAdaptersForwardResourcesCameraAndLayerMode(t *testing.T) {
	setupSpriteManagerTest(t)
	previousRes, previousCamera, previousPlatform, previousExt := gdx.ResMgr, gdx.CameraMgr, gdx.PlatformMgr, gdx.ExtMgr
	previousScheduler, previousScale, previousMode := scheduler, windowScale, state.layerSortMode
	t.Cleanup(func() {
		gdx.ResMgr, gdx.CameraMgr, gdx.PlatformMgr, gdx.ExtMgr = previousRes, previousCamera, previousPlatform, previousExt
		SetScheduler(previousScheduler)
		windowScale, state.layerSortMode = previousScale, previousMode
	})
	resources := &adapterResourceSpy{files: map[string]string{"data": "contents", "res://.config": `{"extasset":"shared"}`}}
	camera := &adapterCameraSpy{position: mathf.NewVec2(10, -5), zoom: mathf.NewVec2(4, 4)}
	platform, ext := &adapterPlatformSpy{}, &adapterExtSpy{}
	gdx.ResMgr, gdx.CameraMgr, gdx.PlatformMgr, gdx.ExtMgr = resources, camera, platform, ext
	SetScheduler(coroutine.New(nil))
	SetWindowScale(2)
	if ReadAllText("data") != "contents" || !HasFile("data") || HasFile("missing") || ListDirectories("assets") != "directories:assets" {
		t.Fatal("resource arguments/results changed")
	}
	SetDebugMode(true)
	if !platform.debug {
		t.Fatal("debug setting not forwarded")
	}
	if got := readExtAssetDirFromProjectConfig("res://"); got != "shared" {
		t.Fatalf("extension assets=%q", got)
	}
	resources.files["res://.config"] = "invalid"
	if readExtAssetDirFromProjectConfig("res://") != "" || readExtAssetDirFromProjectConfig("missing/") != "" {
		t.Fatal("invalid/missing project config accepted")
	}
	world := mathf.NewVec2(13, 2)
	wantView := mathf.NewVec2(6, 14)
	if BridgeWorldToView(world) != wantView {
		t.Fatal("camera transform did not apply position, zoom, and window scale")
	}
	BridgeSetCameraPosition(mathf.NewVec2(8, 9))
	if camera.position != mathf.NewVec2(8, 9) {
		t.Fatal("camera position not forwarded")
	}
	for _, name := range []string{"vertical", "none", ""} {
		if err := SetLayerSortMode(name); err != nil {
			t.Fatal(err)
		}
		if HasLayerSortMethod() != (name == "vertical") {
			t.Fatalf("layer mode %q", name)
		}
	}
	if err := SetLayerSortMode("invalid"); err == nil || !slices.Equal(ext.modes, []int64{1, 0, 0}) {
		t.Fatalf("layer commands=%v,error=%v", ext.modes, err)
	}
}

func TestPublicFactoriesInitializeProxyAndUI(t *testing.T) {
	setupSpriteManagerTest(t)
	previousState, previousScheduler, previousPlatform, previousUI, previousPaths := state, scheduler, gdx.PlatformMgr, gdx.UiMgr, assetPaths
	t.Cleanup(func() {
		state, assetPaths = previousState, previousPaths
		SetScheduler(previousScheduler)
		gdx.PlatformMgr, gdx.UiMgr = previousPlatform, previousUI
	})
	state = runtimeState{sprites: map[Object]gdx.ISpriter{}, uiNodes: map[Object]gdx.IUiNode{}}
	gdx.PlatformMgr = bridgeMainThreadPlatform{}
	SetScheduler(coroutine.New(nil))
	manager := &backdropProxySpy{}
	gdx.SpriteMgr = manager
	ui := &factoryUIManager{result: 20}
	gdx.UiMgr = ui
	assetPaths = assetResolver{root: "assets/", projectRoot: "."}
	target := &struct{}{}
	proxy := NewBackdropProxy(target, "backdrop.png", 2)
	if proxy.Target != target || manager.texture != ToAssetPath("backdrop.png") || manager.scale != mathf.NewVec2(2, 2) {
		t.Fatalf("backdrop proxy=%#v texture=%q scale=%v", proxy, manager.texture, manager.scale)
	}
	if !slices.Equal(manager.settings, []string{"z:12:-1", "trigger-layer:12:0", "collision-layer:12:0", "collision-mask:12:0", "trigger-mask:12:0"}) {
		t.Fatalf("backdrop initialization=%v", manager.settings)
	}
	bare := BridgeNewBareSprite(target, mathf.NewVec2(4, 5))
	if bare.Target != target || bare.GetId() != 11 || manager.position != mathf.NewVec2(4, 5) {
		t.Fatalf("bare sprite=%#v", bare)
	}
	node := NewUiNode[factoryUI]()
	if !node.started || node.GetId() != 20 || ui.path != "res://engine/ui/factoryUI.tscn" {
		t.Fatalf("UI=%#v path=%s", node, ui.path)
	}
}

func TestBridgeGetSpriteChecksConcreteType(t *testing.T) {
	previous := state
	t.Cleanup(func() { state = previous })
	sprite := &Sprite{}
	other := &gdx.Sprite{}
	state = runtimeState{sprites: map[Object]gdx.ISpriter{1: sprite, 2: other}}

	for _, tc := range []struct {
		name string
		id   Object
		want *Sprite
	}{
		{"engine sprite", 1, sprite},
		{"other implementation", 2, nil},
		{"missing sprite", 3, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := GetSprite(tc.id); got != tc.want {
				t.Fatalf("GetSprite(%d) = %p, want %p", tc.id, got, tc.want)
			}
		})
	}
	if got := (runtimeBridge{}).GetSprite(2); got != other {
		t.Fatalf("runtime bridge GetSprite(2) = %v, want original implementation", got)
	}
}

func TestBridgeClearAllSpritesDestroysAndUnregistersNodes(t *testing.T) {
	previousState, previousScheduler, previousPlatform := state, scheduler, gdx.PlatformMgr
	t.Cleanup(func() {
		state = previousState
		SetScheduler(previousScheduler)
		gdx.PlatformMgr = previousPlatform
	})
	SetScheduler(coroutine.New(nil))
	gdx.PlatformMgr = bridgeMainThreadPlatform{}

	for _, tc := range []struct {
		name  string
		clear func()
	}{
		{"public bridge", ClearAllSprites},
		{"runtime bridge", runtimeBridge{}.ClearAllSprites},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sprite, node := &bridgeDestroySprite{}, &bridgeDestroyNode{}
			state = runtimeState{
				sprites: map[Object]gdx.ISpriter{1: sprite},
				uiNodes: map[Object]gdx.IUiNode{2: node},
			}
			tc.clear()
			if len(state.sprites) != 0 || len(state.uiNodes) != 0 {
				t.Fatalf("remaining sprites/nodes = %d/%d, want 0/0", len(state.sprites), len(state.uiNodes))
			}
			// Clearing an already-empty registry must not destroy nodes again.
			tc.clear()
			if sprite.calls != 1 || node.calls != 1 {
				t.Fatalf("sprite/node Destroy calls = %d/%d, want 1/1", sprite.calls, node.calls)
			}
		})
	}
}

func (bridgeMainThreadPlatform) IsMainThread() bool { return true }

func (s *bridgeDestroySprite) Destroy() bool {
	s.calls++
	return true
}

func (n *bridgeDestroyNode) Destroy() bool {
	n.calls++
	return true
}

func (m *adapterResourceSpy) ReadAllText(path string) string { return m.files[path] }

func (m *adapterResourceSpy) HasFile(path string) bool { _, ok := m.files[path]; return ok }

func (m *adapterResourceSpy) ListDirectories(path string) string { return "directories:" + path }

func (m *adapterCameraSpy) GetCameraPosition() mathf.Vec2 { return m.position }

func (m *adapterCameraSpy) GetCameraZoom() mathf.Vec2 { return m.zoom }

func (m *adapterCameraSpy) SetCameraPosition(value mathf.Vec2) { m.position = value }

func (m *adapterPlatformSpy) SetDebugMode(value bool) { m.debug = value }

func (m *adapterExtSpy) SetLayerSorterMode(value int64) { m.modes = append(m.modes, value) }

func (m *backdropProxySpy) SetZIndex(id Object, value int64) {
	m.settings = append(m.settings, fmt.Sprintf("z:%d:%d", id, value))
}

func (m *backdropProxySpy) SetTriggerLayer(id Object, value int64) {
	m.settings = append(m.settings, fmt.Sprintf("trigger-layer:%d:%d", id, value))
}

func (m *backdropProxySpy) SetCollisionLayer(id Object, value int64) {
	m.settings = append(m.settings, fmt.Sprintf("collision-layer:%d:%d", id, value))
}

func (m *backdropProxySpy) SetCollisionMask(id Object, value int64) {
	m.settings = append(m.settings, fmt.Sprintf("collision-mask:%d:%d", id, value))
}

func (m *backdropProxySpy) SetTriggerMask(id Object, value int64) {
	m.settings = append(m.settings, fmt.Sprintf("trigger-mask:%d:%d", id, value))
}

func (m *backdropProxySpy) SetTexture(_ Object, path string) { m.texture = path }

func (m *backdropProxySpy) SetRenderScale(_ Object, value mathf.Vec2) { m.scale = value }
