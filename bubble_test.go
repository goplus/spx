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
	"github.com/goplus/spx/v3/internal/engine"
	"github.com/goplus/spx/v3/internal/enginewrap"
	"github.com/goplus/spx/v3/internal/ui"
	pkgengine "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

func TestBubbleObservesCameraChangesAfterDirtyFlagIsCleared(t *testing.T) {
	sprite := &SpriteImpl{}
	sprite.spriteState.IsVisible = true
	camera := &cameraImpl{}
	bubble := bubbleBase{sprite: sprite, camera: camera, isDirty: true}

	if !bubble.checkNeedsUpdate() {
		t.Fatal("new bubble should need an initial update")
	}
	bubble.markClean()
	if bubble.checkNeedsUpdate() {
		t.Fatal("clean bubble unexpectedly needs an update")
	}

	camera.setDirtyFlag(true)
	camera.setDirtyFlag(false)
	if !bubble.checkNeedsUpdate() {
		t.Fatal("camera change was lost when its dirty flag was cleared")
	}

	bubble.markClean()
	if bubble.checkNeedsUpdate() {
		t.Fatal("bubble still needs an update after observing the camera change")
	}
}

func TestBubbleObservesSpriteChangesAfterProxySync(t *testing.T) {
	installTouchingSyncSpriteMgr(t, newTouchingSyncSpriteMgr())
	sprite := newTouchingTestSprite("sprite", 0, 0, 1)
	camera := &cameraImpl{}
	bubble := bubbleBase{sprite: sprite, camera: camera, isDirty: true}
	bubble.markClean()

	sprite.markProxyDirty()
	buffer := engine.NewSpriteSyncBuffer(1)
	sprite.collectProxyUpdate(buffer)
	if got := buffer.UpdateCount(); got != 1 {
		t.Fatalf("proxy sync batched %d transforms, want 1", got)
	}
	if !bubble.checkNeedsUpdate() {
		t.Fatal("sprite change was lost after its proxy was synchronized")
	}
	bubble.markClean()
	if bubble.checkNeedsUpdate() {
		t.Fatal("bubble still needs an update after observing the sprite change")
	}
}

func TestBubbleVisualsAreDeferredUntilFrameEnd(t *testing.T) {
	sprite := &SpriteImpl{}
	sprite.spriteState.IsVisible = true
	bubble := &quoterBubble{
		bubbleBase: bubbleBase{sprite: sprite, camera: &cameraImpl{}, isDirty: true},
	}
	items := []Shape{bubble}
	var shapes shapeManager

	shapes.flushActivate(items)
	if !bubble.isDirty {
		t.Fatal("update phase unexpectedly committed bubble visuals")
	}

	shapes.flushBubbleVisuals(items)
	if bubble.isDirty {
		t.Fatal("frame-end phase did not commit bubble visuals")
	}
}

func TestLayoutTextBubblesPreservesEmptyActiveSlice(t *testing.T) {
	for _, active := range [][]*textBubble{nil, {}} {
		shapes := shapeManager{activeTextBubbles: active}
		shapes.layoutTextBubbles(nil)
		if len(shapes.activeTextBubbles) != 0 || (shapes.activeTextBubbles == nil) != (active == nil) {
			t.Fatalf("empty topology changed active slice: before=%#v after=%#v", active, shapes.activeTextBubbles)
		}
	}
}

type bubbleLayoutCameraMgr struct{ pkgengine.ICameraMgr }

func (bubbleLayoutCameraMgr) GetCameraPosition() mathf.Vec2 { return mathf.Vec2{} }
func (bubbleLayoutCameraMgr) GetCameraZoom() mathf.Vec2     { return mathf.NewVec2(1, 1) }

func TestLayoutTextBubblesTracksPointerIdentity(t *testing.T) {
	enginewrap.Init(func(call func()) { call() })
	original := pkgengine.CameraMgr
	pkgengine.CameraMgr = bubbleLayoutCameraMgr{}
	t.Cleanup(func() { pkgengine.CameraMgr = original })
	newBubble := func(id uint64) *textBubble {
		sprite := newRenderOffsetTestSprite()
		sprite.spriteState.IsVisible = true
		sprite.g.displayState.WindowWidth = 480
		sprite.g.displayState.WindowHeight = 360
		return &textBubble{
			bubbleBase: bubbleBase{sprite: sprite},
			layoutID:   id,
			panel:      &ui.UiSay{},
			content:    ui.NewSayBubbleContent("same message", ui.StyleSay),
		}
	}
	first, second := newBubble(1), newBubble(2)
	shapes := shapeManager{}
	shapes.layoutTextBubbles([]Shape{second, first})
	if !slices.Equal(shapes.activeTextBubbles, []*textBubble{first, second}) || !first.hasLayout || !second.hasLayout {
		t.Fatal("new bubbles were not laid out in stable ID order")
	}
	storage := &shapes.activeTextBubbles[0]
	firstLayout, secondLayout := first.layout, second.layout
	shapes.layoutTextBubbles([]Shape{first, second})
	if &shapes.activeTextBubbles[0] != storage || first.layout != firstLayout || second.layout != secondLayout {
		t.Fatal("unchanged bubble identities replaced storage or resolved cached layouts")
	}

	replacement := *first
	shapes.layoutTextBubbles([]Shape{second, &replacement})
	if !slices.Equal(shapes.activeTextBubbles, []*textBubble{&replacement, second}) {
		t.Fatal("equal-valued replacement bubble was not detected by pointer identity")
	}
	second.sprite.spriteState.IsVisible = false
	shapes.layoutTextBubbles([]Shape{&replacement, second})
	if !slices.Equal(shapes.activeTextBubbles, []*textBubble{&replacement}) {
		t.Fatal("hidden bubble remained active")
	}
	if shapes.activeTextBubbles[:2][1] != nil {
		t.Fatal("removed bubble reference was retained in reusable storage")
	}
	replacement.panel = nil
	shapes.layoutTextBubbles([]Shape{&replacement, second})
	if len(shapes.activeTextBubbles) != 0 || shapes.activeTextBubbles[:1][0] != nil {
		t.Fatal("panel removal did not clear the final active bubble")
	}
}
