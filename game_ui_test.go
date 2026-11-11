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
	"encoding/json"
	"math"
	"os"
	"reflect"
	"regexp"
	"slices"
	"syscall"
	"testing"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v4/internal/core/clock"
	coreproject "github.com/goplus/spx/v4/internal/core/project"
	"github.com/goplus/spx/v4/internal/engine"
	"github.com/goplus/spx/v4/internal/engine/coroutine"
	"github.com/goplus/spx/v4/internal/engine/ui"
	pkgengine "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

type bubbleLayoutCameraMgr struct{ pkgengine.ICameraMgr }

const (
	askScenePath = "res://engine/ui/UiAsk.tscn"
	sayScenePath = "res://engine/ui/UiSay.tscn"
)

type askTestBinding struct {
	parent pkgengine.Object
	path   string
}

type askTestUiMgr struct {
	pkgengine.IUiMgr
	nextID       pkgengine.Object
	ids          map[pkgengine.Object]struct{}
	created      map[string]int
	bound        map[askTestBinding]pkgengine.Object
	visible      map[pkgengine.Object]bool
	text         map[pkgengine.Object]string
	askRoot      pkgengine.Object
	onAskVisible func()
	onDestroy    func()
}

type bubbleRenderTestUiMgr struct {
	*askTestUiMgr
	positions map[pkgengine.Object]mathf.Vec2
	sizes     map[pkgengine.Object]mathf.Vec2
}

type askDelayedInputMgr struct{ pkgengine.IInputMgr }

func TestBubbleEntryPointsCreateAndRenewBubbles(t *testing.T) {
	installAskTestUiMgr(t)
	_, sprite, component := newBubbleTestSprite()

	textToken := sprite.sayOrThink(123, ui.StyleSay)
	if textToken.bubble == nil || component.textObj == nil {
		t.Fatal("Say/Think did not create a text bubble")
	}
	component.expireText(textToken)
	renewedText := component.upsertText("renewed", ui.StyleThink)
	if renewedText.generation != textToken.generation+1 || component.textObj.timedRemovalPending {
		t.Fatal("text bubble renewal did not invalidate its pending removal")
	}
	component.expireText(renewedText)
	content := component.textObj.content
	sameText := component.upsertText("renewed", ui.StyleThink)
	component.expireText(renewedText)
	component.textObj.flushPendingRemoval(clock.Frame() + 1)
	if component.textObj == nil || sameText.bubble != renewedText.bubble || sameText.generation != renewedText.generation+1 || component.textObj.timedRemovalPending || component.textObj.content != content {
		t.Fatal("identical text did not renew the current bubble and cancel older removal")
	}
	component.textObj.generation = ^uint64(0)
	if wrapped := component.upsertText("renewed", ui.StyleThink); wrapped.generation != 1 {
		t.Fatalf("wrapped text generation = %d, want 1", wrapped.generation)
	}
	if empty := sprite.sayOrThink("", ui.StyleSay); empty.bubble != nil || component.textObj != nil {
		t.Fatal("empty Say/Think did not stop the text bubble")
	}
	sprite.doStopText()

	quoteToken := sprite.quote("message", "description")
	if quoteToken.bubble == nil || component.quoteObj == nil {
		t.Fatal("Quote did not create a quote bubble")
	}
	component.expireQuote(quoteToken)
	renewedQuote := component.upsertQuote("renewed", "description")
	if renewedQuote.generation != quoteToken.generation+1 || component.quoteObj.timedRemovalPending {
		t.Fatal("quote renewal did not invalidate its pending removal")
	}
	component.expireQuote(renewedQuote)
	sameQuote := component.upsertQuote("renewed", "description")
	component.expireQuote(renewedQuote)
	component.quoteObj.flushPendingRemoval(clock.Frame() + 1)
	if component.quoteObj == nil || sameQuote.bubble != renewedQuote.bubble || sameQuote.generation != renewedQuote.generation+1 || component.quoteObj.timedRemovalPending {
		t.Fatal("identical quote did not renew the current bubble and cancel older removal")
	}
	component.quoteObj.generation = ^uint64(0)
	if wrapped := component.upsertQuote("renewed", "description"); wrapped.generation != 1 {
		t.Fatalf("wrapped quote generation = %d, want 1", wrapped.generation)
	}
	sprite.doStopQuote()
	if component.quoteObj != nil {
		t.Fatal("explicit quote stop did not remove the quote bubble")
	}
	sprite.doStopQuote()

	// Exercise the generated public entry points with untimed calls.
	sprite.Say__0("say")
	sprite.Say__1("say", 0)
	sprite.Think__0("think")
	sprite.Think__1("think", 0)
	sprite.Quote__0("quote")
	sprite.Quote__1("quote", 0)
	sprite.Quote__2("quote", "description")
	sprite.Quote__3("quote", "description", 0)
	sprite.QuoteMsg("", 0)
	component.stopAll()
}

func TestBubbleRemovalUnlocksBeforeDestroy(t *testing.T) {
	for _, kind := range []string{"text", "quote"} {
		for _, removal := range []string{"stop", "expire"} {
			t.Run(kind+"/"+removal, func(t *testing.T) {
				mgr := installAskTestUiMgr(t)
				game, _, component := newBubbleTestSprite()
				var bubble bubbleShape
				var stop func()
				if kind == "text" {
					token := component.upsertText("message", ui.StyleSay)
					bubble, stop = component.textObj, component.stopText
					component.expireText(token)
				} else {
					token := component.upsertQuote("message", "description")
					bubble, stop = component.quoteObj, component.stopQuote
					component.expireQuote(token)
				}
				destroyed := false
				mgr.onDestroy = func() {
					destroyed = true
					if !component.mu.TryLock() {
						t.Fatal("panel destruction ran with the component locked")
					}
					component.mu.Unlock()
					if component.textObj != nil || component.quoteObj != nil {
						t.Fatal("panel destruction ran before the current slot was cleared")
					}
				}
				if removal == "stop" {
					stop()
				} else {
					bubble.flushPendingRemoval(clock.Frame() + 1)
				}
				if !destroyed || game.shapeMgr.findShapeIndex(bubble) >= 0 {
					t.Fatal("bubble removal did not destroy its panel and remove its shape")
				}
			})
		}
	}
}

func TestTimedBubbleEntryPointsScheduleRemoval(t *testing.T) {
	installAskTestUiMgr(t)
	currentScheduler := setupRuntimeScheduler(t)
	clock.Start()
	_, sprite, component := newBubbleTestSprite()

	done := false
	thread := currentScheduler.Create(sprite, func(coroutine.Thread) {
		sprite.Say__1("say", 0.01)
		sprite.Think__1("think", 0.01)
		sprite.Quote__3("quote", "description", 0.01)
		done = true
	})
	currentScheduler.JoinFirstSlice(thread)
	currentScheduler.Update()
	if done {
		t.Fatal("timed bubble calls completed in their issuing frame")
	}
	for range 3 {
		clock.Update(1.0/30, 30)
		currentScheduler.Update()
	}
	if !done {
		t.Fatal("timed bubble calls did not complete after their deadlines")
	}
	if component.textObj == nil || !component.textObj.timedRemovalPending {
		t.Fatal("timed Think did not schedule text bubble removal")
	}
	if component.quoteObj == nil || !component.quoteObj.timedRemovalPending {
		t.Fatal("timed Quote did not schedule quote bubble removal")
	}
	component.stopAll()
}

func TestBubbleObservesCameraChangesAfterDirtyFlagIsCleared(t *testing.T) {
	sprite := &SpriteImpl{}
	sprite.spriteState.isVisible = true
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
	_, game := setupRuntimeEventGame(t)
	sprite := &SpriteImpl{}
	sprite.spriteState.isVisible = true
	quote := &quoterBubble{
		bubbleBase: bubbleBase{sprite: sprite, camera: &cameraImpl{}, isDirty: true},
	}
	text := &textBubble{
		bubbleBase: bubbleBase{sprite: sprite, camera: &cameraImpl{}, isDirty: true},
	}
	component := &bubbleComponent{sprite: sprite, textObj: text}
	sprite.components.bubble = component
	items := []Shape{sprite, quote, text}
	game.shapeMgr.items = items
	withEventRegistrationBarrier(game, game.dispatchShapeUpdates)
	if !quote.isDirty || !text.isDirty {
		t.Fatal("update phase unexpectedly committed bubble visuals")
	}

	game.shapeMgr.flushBubbleVisuals(items)
	if quote.isDirty || text.isDirty {
		t.Fatal("frame-end phase did not commit bubble visuals")
	}
	if len(game.shapeMgr.bubbles) != 2 {
		t.Fatalf("collected %d bubbles, want 2", len(game.shapeMgr.bubbles))
	}
}

func TestTimedTextRemovalWaitsForNextFrame(t *testing.T) {
	game, sprite, component := newBubbleTestSprite()
	bubble := &textBubble{
		bubbleBase: bubbleBase{
			sprite:              sprite,
			timedRemovalPending: true,
			timedRemovalFrame:   10,
			generation:          1,
		},
	}
	component.textObj = bubble
	game.shapeMgr.add(bubble)

	for _, frame := range []int64{9, 10} {
		bubble.flushPendingRemoval(frame)
		if component.textObj != bubble || game.shapeMgr.findShapeIndex(bubble) < 0 {
			t.Fatalf("timed text bubble was removed before its grace frame ended: %d", frame)
		}
	}
	bubble.timedRemovalPending = false
	bubble.flushPendingRemoval(11)
	if component.textObj != bubble {
		t.Fatal("cancelled text expiration removed the bubble")
	}
	bubble.timedRemovalPending = true

	bubble.flushPendingRemoval(11)
	if component.textObj != nil || game.shapeMgr.findShapeIndex(bubble) >= 0 {
		t.Fatal("timed bubble was not removed on the following frame")
	}

	// A stale shape must not remove the replacement now occupying its slot.
	replacement := *bubble
	replacement.timedRemovalPending = true
	component.textObj = &replacement
	game.shapeMgr.add(&replacement)
	bubble.timedRemovalPending = true
	bubble.flushPendingRemoval(12)
	if component.textObj != &replacement || game.shapeMgr.findShapeIndex(&replacement) < 0 {
		t.Fatal("stale text bubble removed its replacement")
	}
	replacement.flushPendingRemoval(12)
	if component.textObj != nil || replacement.timedRemovalPending {
		t.Fatal("current text bubble did not commit its expiration")
	}
}

func TestTextBubbleTokenOnlyExpiresCurrentGeneration(t *testing.T) {
	bubble := &textBubble{bubbleBase: bubbleBase{generation: 2}}
	component := &bubbleComponent{textObj: bubble}

	for _, token := range []bubbleToken{{}, {bubble: &bubble.bubbleBase, generation: 1}} {
		component.expireText(token)
		if bubble.timedRemovalPending {
			t.Fatalf("invalid token %+v scheduled bubble removal", token)
		}
	}

	component.expireText(bubbleToken{bubble: &bubble.bubbleBase, generation: 2})
	if !bubble.timedRemovalPending {
		t.Fatal("current token did not schedule timed bubble removal")
	}

	bubble.timedRemovalPending = false
	component.textObj = &textBubble{bubbleBase: bubbleBase{generation: 2}}
	component.expireText(bubbleToken{bubble: &bubble.bubbleBase, generation: 2})
	if bubble.timedRemovalPending || component.textObj.timedRemovalPending {
		t.Fatal("token scheduled removal of a different bubble with the same generation")
	}

	component.textObj = nil
	component.expireText(bubbleToken{bubble: &bubble.bubbleBase, generation: 2})
}

func TestTimedQuoteRemovalWaitsForNextFrame(t *testing.T) {
	game, sprite, component := newBubbleTestSprite()
	quote := &quoterBubble{
		bubbleBase: bubbleBase{
			sprite:              sprite,
			generation:          1,
			timedRemovalPending: true,
			timedRemovalFrame:   10,
		},
	}
	component.quoteObj = quote
	game.shapeMgr.add(quote)

	for _, frame := range []int64{9, 10} {
		quote.flushPendingRemoval(frame)
		if component.quoteObj != quote || game.shapeMgr.findShapeIndex(quote) < 0 {
			t.Fatalf("timed quote bubble was removed before its grace frame ended: %d", frame)
		}
	}
	quote.timedRemovalPending = false
	quote.flushPendingRemoval(11)
	if component.quoteObj != quote {
		t.Fatal("cancelled quote expiration removed the bubble")
	}
	quote.timedRemovalPending = true

	quote.flushPendingRemoval(11)
	if component.quoteObj != nil || game.shapeMgr.findShapeIndex(quote) >= 0 {
		t.Fatal("timed quote was not removed on the following frame")
	}

	// A stale shape must not remove the replacement now occupying its slot.
	replacement := *quote
	replacement.timedRemovalPending = true
	component.quoteObj = &replacement
	game.shapeMgr.add(&replacement)
	quote.timedRemovalPending = true
	quote.flushPendingRemoval(12)
	if component.quoteObj != &replacement || game.shapeMgr.findShapeIndex(&replacement) < 0 {
		t.Fatal("stale quote bubble removed its replacement")
	}
	replacement.flushPendingRemoval(12)
	if component.quoteObj != nil || replacement.timedRemovalPending {
		t.Fatal("current quote bubble did not commit its expiration")
	}
}

func TestQuoteBubbleTokenOnlyExpiresCurrentGeneration(t *testing.T) {
	quote := &quoterBubble{bubbleBase: bubbleBase{generation: 2}}
	component := &bubbleComponent{quoteObj: quote}

	for _, token := range []bubbleToken{{}, {bubble: &quote.bubbleBase, generation: 1}} {
		component.expireQuote(token)
		if quote.timedRemovalPending {
			t.Fatalf("invalid token %+v scheduled bubble removal", token)
		}
	}

	component.expireQuote(bubbleToken{bubble: &quote.bubbleBase, generation: 2})
	if !quote.timedRemovalPending {
		t.Fatal("current token did not schedule timed quote removal")
	}

	quote.timedRemovalPending = false
	component.quoteObj = &quoterBubble{bubbleBase: bubbleBase{generation: 2}}
	component.expireQuote(bubbleToken{bubble: &quote.bubbleBase, generation: 2})
	if quote.timedRemovalPending || component.quoteObj.timedRemovalPending {
		t.Fatal("token scheduled removal of a different quote with the same generation")
	}

	component.quoteObj = nil
	component.expireQuote(bubbleToken{bubble: &quote.bubbleBase, generation: 2})
}

func TestLayoutTextBubblesPreservesEmptyActiveSlice(t *testing.T) {
	for _, active := range [][]*textBubble{nil, {}} {
		shapes := shapeManager{activeTextBubbles: active, textBubbles: active}
		shapes.collectBubbles(nil)
		shapes.layoutTextBubbles()
		if len(shapes.activeTextBubbles) != 0 || (shapes.activeTextBubbles == nil) != (active == nil) {
			t.Fatalf("empty topology changed active slice: before=%#v after=%#v", active, shapes.activeTextBubbles)
		}
		if len(shapes.textBubbles) != 0 || (shapes.textBubbles == nil) != (active == nil) {
			t.Fatalf("empty topology changed collected slice: before=%#v after=%#v", active, shapes.textBubbles)
		}
	}
}

func TestLayoutTextBubblesTracksPointerIdentity(t *testing.T) {
	setupImmediateEngineCalls(t)
	original := pkgengine.CameraMgr
	pkgengine.CameraMgr = bubbleLayoutCameraMgr{}
	t.Cleanup(func() { pkgengine.CameraMgr = original })
	newBubble := func(id uint64) *textBubble {
		sprite := newRenderOffsetTestSprite()
		sprite.spriteState.isVisible = true
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
	layout := func(items []Shape) {
		shapes.collectBubbles(items)
		shapes.layoutTextBubbles()
	}
	layout([]Shape{second, first})
	if !slices.Equal(shapes.activeTextBubbles, []*textBubble{first, second}) || !first.hasLayout || !second.hasLayout {
		t.Fatal("new bubbles were not laid out in stable ID order")
	}
	storage := &shapes.activeTextBubbles[0]
	firstLayout, secondLayout := first.layout, second.layout
	layout([]Shape{first, second})
	if &shapes.activeTextBubbles[0] != storage || first.layout != firstLayout || second.layout != secondLayout {
		t.Fatal("unchanged bubble identities replaced storage or resolved cached layouts")
	}

	replacement := *first
	layout([]Shape{second, &replacement})
	if !slices.Equal(shapes.activeTextBubbles, []*textBubble{&replacement, second}) {
		t.Fatal("equal-valued replacement bubble was not detected by pointer identity")
	}
	second.sprite.spriteState.isVisible = false
	layout([]Shape{&replacement, second})
	if !slices.Equal(shapes.activeTextBubbles, []*textBubble{&replacement}) {
		t.Fatal("hidden bubble remained active")
	}
	if shapes.activeTextBubbles[:2][1] != nil || shapes.textBubbles[:2][1] != nil {
		t.Fatal("removed bubble reference was retained in reusable storage")
	}
	replacement.panel = nil
	layout([]Shape{&replacement, second})
	if len(shapes.activeTextBubbles) != 0 || shapes.activeTextBubbles[:1][0] != nil {
		t.Fatal("panel removal did not clear the final active bubble")
	}
	for _, bubble := range shapes.textBubbles[:cap(shapes.textBubbles)] {
		if bubble != nil {
			t.Fatal("filtered bubble remained in collected storage")
		}
	}

	for _, reason := range []string{"hidden", "panel-less"} {
		for removed, position := range []string{"first", "middle", "last"} {
			t.Run(reason+"/"+position, func(t *testing.T) {
				first, second, third := newBubble(1), newBubble(2), newBubble(3)
				bubbles := []*textBubble{first, second, third}
				if reason == "hidden" {
					bubbles[removed].sprite.spriteState.isVisible = false
				} else {
					bubbles[removed].panel = nil
				}
				var shapes shapeManager
				shapes.collectBubbles([]Shape{first, second, third})
				storage := shapes.textBubbles[:cap(shapes.textBubbles)]
				shapes.layoutTextBubbles()
				want := slices.Delete(slices.Clone(bubbles), removed, removed+1)
				if !slices.Equal(shapes.textBubbles, want) || &shapes.textBubbles[0] != &storage[0] || storage[2] != nil {
					t.Fatal("filtering lost order, replaced storage, or retained removed bubble")
				}
				shapes.collectBubbles(nil)
				shapes.layoutTextBubbles()
				shapes.init()
				for _, bubble := range storage {
					if bubble != nil {
						t.Fatal("empty frame and reset retained bubble")
					}
				}
			})
		}
	}
}

func TestAskQuestionPlacement(t *testing.T) {
	tests := []struct {
		name          string
		useStage      bool
		spriteVisible bool
		wantPanel     bool
		wantBubble    bool
	}{
		{name: "stage", useStage: true, wantPanel: true},
		{name: "visible sprite", spriteVisible: true, wantBubble: true},
		{name: "hidden sprite", wantPanel: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			mgr := installAskTestUiMgr(t)
			game := &Game{}
			game.shapeMgr.init()
			mgr.onAskVisible = func() {
				game.dialogState.AskPanel.OnCheck("answer")
			}

			const question = "How many?"
			if test.useStage {
				game.Ask(question)
			} else {
				sprite := &SpriteImpl{g: game}
				sprite.spriteState.isVisible = test.spriteVisible
				sprite.Ask(question)
			}

			askBody := mgr.bound[askTestBinding{parent: mgr.askRoot, path: "MF/Frame/AskBody"}]
			if got := mgr.visible[askBody]; got != test.wantPanel {
				t.Errorf("question panel visible = %v, want %v", got, test.wantPanel)
			}
			askLabel := mgr.bound[askTestBinding{parent: mgr.askRoot, path: "MF/Frame/AskBody/LabelAsk"}]
			if test.wantPanel && mgr.text[askLabel] != question {
				t.Errorf("question panel text = %q, want %q", mgr.text[askLabel], question)
			}
			if got := mgr.created[sayScenePath] != 0; got != test.wantBubble {
				t.Errorf("question bubble created = %v, want %v", got, test.wantBubble)
			}
			if got := game.Answer(); got != "answer" {
				t.Errorf("answer = %q, want %q", got, "answer")
			}
		})
	}
}

func TestBubbleRefreshUsesCurrentTextLayoutAndQuoteBounds(t *testing.T) {
	mgr := &bubbleRenderTestUiMgr{
		askTestUiMgr: installAskTestUiMgr(t),
		positions:    make(map[pkgengine.Object]mathf.Vec2), sizes: make(map[pkgengine.Object]mathf.Vec2),
	}
	pkgengine.UiMgr = mgr
	originalCamera, originalScale := pkgengine.CameraMgr, engine.WindowScale()
	pkgengine.CameraMgr = bubbleLayoutCameraMgr{}
	engine.SetWindowScale(1)
	t.Cleanup(func() { pkgengine.CameraMgr = originalCamera; engine.SetWindowScale(originalScale) })
	sprite := newRenderOffsetTestSprite()
	sprite.g.displayState.WindowWidth, sprite.g.displayState.WindowHeight = 480, 360
	sprite.spriteState.isVisible = true
	bubble := &textBubble{bubbleBase: bubbleBase{sprite: sprite, camera: &cameraImpl{}, isDirty: true}, panel: ui.NewUiSay(), content: ui.NewSayBubbleContent("first", ui.StyleSay)}
	manager := shapeManager{}
	sprite.spriteState.isVisible = false
	manager.flushBubbleVisuals([]Shape{bubble})
	if bubble.hasLayout || !bubble.isDirty || slices.Contains(bubbleRenderedTexts(mgr.text), "first") {
		t.Fatal("hidden bubble initialized or rendered its layout")
	}
	sprite.spriteState.isVisible = true
	manager.flushBubbleVisuals([]Shape{bubble})
	if bubble.isDirty || !slices.Contains(bubbleRenderedTexts(mgr.text), "first") {
		t.Fatalf("text bubble was not rendered: dirty=%v text=%v", bubble.isDirty, mgr.text)
	}
	layout := ui.NewSayBubbleLayoutContext(mathf.NewVec2(480, 360)).NewLayout(1, mathf.Vec2{}, mathf.NewVec2(20, 20), ui.NewSayBubbleContent("planned", ui.StyleThink))
	bubble.setLayout(layout)
	bubble.onUpdate()
	if bubble.isDirty || !slices.Contains(bubbleRenderedTexts(mgr.text), "planned") {
		t.Fatal("precomputed layout did not replace the visible text")
	}
	sprite.spriteState.isVisible = false
	bubble.markDirty()
	bubble.onUpdate()
	if !bubble.isDirty {
		t.Fatal("hidden bubble consumed its pending visual update")
	}
	sprite.spriteState.isVisible = true
	panel := bubble.panel
	bubble.panel = nil
	bubble.markDirty()
	manager.flushBubbleVisuals([]Shape{bubble})
	bubble.panel = panel
	quote := &quoterBubble{bubbleBase: bubbleBase{sprite: sprite, camera: bubble.camera, isDirty: true}, panel: ui.NewUiQuote(), message: "quotation", description: "speaker"}
	quote.onUpdate()
	container := mgr.bound[askTestBinding{parent: quote.panel.GetId(), path: "C"}]
	_, size := quote.getBounds()
	if mgr.sizes[container] != size.Addf(20) || !slices.Contains(bubbleRenderedTexts(mgr.text), "quotation") || !slices.Contains(bubbleRenderedTexts(mgr.text), "speaker") || quote.isDirty {
		t.Fatalf("quote layout = size %v, text %v, dirty %v", mgr.sizes[container], mgr.text, quote.isDirty)
	}
}

func TestAskWaitsForAnswerAcrossFrames(t *testing.T) {
	mgr := installAskTestUiMgr(t)
	originalInput := pkgengine.InputMgr
	pkgengine.InputMgr = askDelayedInputMgr{}
	t.Cleanup(func() { pkgengine.InputMgr = originalInput })
	t.Cleanup(clock.ClearTimers)
	currentScheduler := setupRuntimeScheduler(t)
	clock.Start()
	game := &Game{}
	game.shapeMgr.init()
	answered := false
	thread := currentScheduler.Create(game, func(coroutine.Thread) { game.Ask("later?"); answered = true })
	currentScheduler.JoinFirstSlice(thread)
	if answered || game.dialogState.AskPanel == nil || !mgr.visible[mgr.askRoot] {
		t.Fatal("Ask did not suspend while displaying its question")
	}
	game.dialogState.AskPanel.OnCheck("later answer")
	clock.Update(1.0/30, 30)
	currentScheduler.Update()
	if !answered || game.Answer() != "later answer" {
		t.Fatalf("Ask resumed=%v, answer=%q", answered, game.Answer())
	}
	currentScheduler.Join(thread)
}

func TestCloneBubbleIsLazyAndIndependent(t *testing.T) {
	game := setupCloneLimitGame(t)
	original := newCloneLimitSprite(game, "original")
	originalBubble := original.bubble()
	originalText := &textBubble{bubbleBase: original.newBubbleBase()}
	originalBubble.textObj = originalText
	game.shapeMgr.add(originalText)

	clone := createRuntimeClone(&original.SpriteImpl)
	if clone.components.bubble != nil {
		t.Fatal("clone inherited an allocated bubble component")
	}
	if originalBubble.textObj != originalText {
		t.Fatal("cloning changed the original bubble")
	}

	cloneBubble := clone.bubble()
	if cloneBubble == originalBubble || cloneBubble.sprite != clone {
		t.Fatal("lazy bubble component is not owned by the clone")
	}
	cloneText := &textBubble{bubbleBase: clone.newBubbleBase()}
	cloneBubble.textObj = cloneText
	game.shapeMgr.add(cloneText)

	clone.Destroy()
	if cloneBubble.textObj != nil || cloneBubble.quoteObj != nil {
		t.Fatal("destroy left clone bubble state active")
	}
	for _, shape := range game.getAllShapes() {
		if shape == cloneText {
			t.Fatal("destroy left the clone bubble in the shape list")
		}
	}
	if originalBubble.textObj != originalText {
		t.Fatal("destroying the clone changed the original bubble")
	}
}

func TestSpriteDialogueDebugModeKeepsPresentationAndAnswer(t *testing.T) {
	game := setupCloneLimitGame(t)
	game.debugState.DebugInstr = true
	mgr := installAskTestUiMgr(t)
	sprite := newCloneLimitSprite(game, "speaker")
	sprite.Show()
	sprite.SayWith("hello", 0)
	sprite.ThinkWith("thought", 0)
	sprite.QuoteMsgEx("quote", "source", 0)
	if sprite.bubble().textObj.msg != "thought" || sprite.bubble().quoteObj.message != "quote" {
		t.Fatal("debug logging changed dialogue presentation")
	}
	mgr.onAskVisible = func() { game.dialogState.AskPanel.OnCheck("answer") }
	sprite.Ask("question")
	if game.Answer() != "answer" || sprite.bubble().textObj != nil {
		t.Fatal("debug Ask did not store the answer and retire its bubble")
	}
	sprite.bubble().stopAll()
}

func TestBubbleLayoutIDWrapsWithoutUsingZero(t *testing.T) {
	manager := shapeManager{nextTextBubbleLayoutID: ^uint64(0)}
	bubble := &textBubble{}
	manager.add(bubble)
	if bubble.layoutID != 1 {
		t.Fatalf("wrapped layout ID = %d, want 1", bubble.layoutID)
	}
}

func TestShapeManagerRelayoutsChangedBubbleInput(t *testing.T) {
	mgr := setupCameraTestMgr(t)
	mgr.zoom = mathf.NewVec2(1, 1)
	previousScale := engine.WindowScale()
	engine.SetWindowScale(1)
	t.Cleanup(func() { engine.SetWindowScale(previousScale) })
	sprite := newRenderOffsetTestSprite()
	sprite.spriteState.isVisible = true
	sprite.g.displayState.WindowWidth, sprite.g.displayState.WindowHeight = 480, 360
	bubble := &textBubble{bubbleBase: bubbleBase{sprite: sprite}, layoutID: 1, panel: &ui.UiSay{}, content: ui.NewSayBubbleContent("hello", ui.StyleSay)}
	manager := shapeManager{textBubbles: []*textBubble{bubble}}
	manager.layoutTextBubbles()
	previous := bubble.layout
	sprite.SetXYpos(80, 60)
	manager.layoutTextBubbles()
	if bubble.layout.SameInput(previous) {
		t.Fatal("moving bubble owner did not refresh cached layout input")
	}
}

func (bubbleLayoutCameraMgr) GetCameraPosition() mathf.Vec2 { return mathf.Vec2{} }

func (bubbleLayoutCameraMgr) GetCameraZoom() mathf.Vec2 { return mathf.NewVec2(1, 1) }

func (bubbleLayoutCameraMgr) GetViewportRect() mathf.Rect2 {
	return mathf.NewRect2(0, 0, 480, 360)
}

func (m *askTestUiMgr) CreateNode(path string) pkgengine.Object {
	id := m.newID()
	m.created[path]++
	if path == askScenePath {
		m.askRoot = id
	}
	return id
}

func (m *askTestUiMgr) BindNode(parent pkgengine.Object, path string) pkgengine.Object {
	id := m.newID()
	m.bound[askTestBinding{parent: parent, path: path}] = id
	return id
}

func (m *askTestUiMgr) DestroyNode(pkgengine.Object) bool {
	if m.onDestroy != nil {
		m.onDestroy()
	}
	return true
}

func (m *askTestUiMgr) SetVisible(id pkgengine.Object, visible bool) {
	m.visible[id] = visible
	if id == m.askRoot && visible && m.onAskVisible != nil {
		onAskVisible := m.onAskVisible
		m.onAskVisible = nil
		onAskVisible()
	}
}

func (m *askTestUiMgr) SetText(id pkgengine.Object, text string) {
	m.text[id] = text
}

func (m *askTestUiMgr) GetText(id pkgengine.Object) string {
	return m.text[id]
}

func (m *askTestUiMgr) newID() pkgengine.Object {
	m.nextID++
	m.ids[m.nextID] = struct{}{}
	return m.nextID
}

func (*bubbleRenderTestUiMgr) SetScale(pkgengine.Object, mathf.Vec2) {}

func (m *bubbleRenderTestUiMgr) SetPosition(id pkgengine.Object, position mathf.Vec2) {
	m.positions[id] = position
}

func (m *bubbleRenderTestUiMgr) SetGlobalPosition(id pkgengine.Object, position mathf.Vec2) {
	m.positions[id] = position
}

func (m *bubbleRenderTestUiMgr) SetSize(id pkgengine.Object, size mathf.Vec2) {
	m.sizes[id] = size
}

func (askDelayedInputMgr) GetKey(int64) bool { return false }

func newBubbleTestSprite() (*Game, *SpriteImpl, *bubbleComponent) {
	game := &Game{}
	game.shapeMgr.init()
	sprite := &SpriteImpl{g: game}
	sprite.spriteState.isVisible = true
	component := &bubbleComponent{sprite: sprite}
	sprite.components.bubble = component
	return game, sprite, component
}

func newAskTestUiMgr(base pkgengine.IUiMgr) *askTestUiMgr {
	return &askTestUiMgr{
		IUiMgr:  base,
		nextID:  100,
		ids:     make(map[pkgengine.Object]struct{}),
		created: make(map[string]int),
		bound:   make(map[askTestBinding]pkgengine.Object),
		visible: make(map[pkgengine.Object]bool),
		text:    make(map[pkgengine.Object]string),
	}
}

func installAskTestUiMgr(t *testing.T) *askTestUiMgr {
	t.Helper()
	setupImmediateEngineCalls(t)
	original := pkgengine.UiMgr
	mgr := newAskTestUiMgr(original)
	pkgengine.UiMgr = mgr
	t.Cleanup(func() {
		for id := range mgr.ids {
			pkgengine.DeleteUINode(id)
		}
		pkgengine.UiMgr = original
	})
	return mgr
}

func bubbleRenderedTexts(values map[pkgengine.Object]string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	return out
}

type monitorEvalFixture struct {
	Game
	score float64
	unset any
	calls int
}

type monitorPanelSpy struct {
	visible bool
	events  []string
	value   ui.MonitorValue
	scale   float64
	pos     mathf.Vec2
}

type SliderMonitorGame struct {
	Game
	score float64
	Dot   sliderMonitorSprite
}

type sliderMonitorSprite struct {
	SpriteImpl
	*SliderMonitorGame
	score float64
}

type gameWidgetUiMgr struct {
	*askTestUiMgr
	rangeValue float64
}

type ListMonitorGame struct {
	Game
	items  List
	values []string
	Monkey listMonitorSprite
}

type listMonitorSprite struct {
	SpriteImpl
	*ListMonitorGame
	items List
}

func TestMonitorEvalPreservesFormatting(t *testing.T) {
	g := monitorEvalFixture{score: 1.23456789, unset: nil}
	root := reflect.ValueOf(&g).Elem()
	for _, name := range []string{"score", "getVar:score"} {
		eval := monitorEvalForTest(root, "", name, ui.MonitorAppearanceDefault)
		if eval == nil || eval().Text != "1.23456789" {
			t.Fatalf("field binding %q lost precision", name)
		}
		g.score = 2.3456789
		if eval().Text != "2.3456789" {
			t.Fatal("field binding stopped tracking changes")
		}
		g.score = 1.23456789
	}
	for _, name := range []string{"value", "getVar:value"} {
		eval := monitorEvalForTest(root, "", name, ui.MonitorAppearanceDefault)
		if eval == nil || g.calls != 0 {
			t.Fatal("binding must not invoke a getter")
		}
		if eval().Text != "1.23" || g.calls != 1 {
			t.Fatal("auto-property formatting or invocation changed")
		}
		g.calls = 0
	}
	// An explicitly named method is displayed as its function pointer by the
	// existing string evaluator; only the lowercase auto-property is invoked.
	eval := monitorEvalForTest(root, "", "Value", ui.MonitorAppearanceDefault)
	if eval == nil || !regexp.MustCompile(`^0x[0-9a-f]+$`).MatchString(eval().Text) || g.calls != 0 {
		t.Fatal("explicit method behavior changed")
	}
	if eval := monitorEvalForTest(root, "", "unset", ui.MonitorAppearanceDefault); eval == nil || eval().Text != "<nil>" {
		t.Fatal("nil field formatting changed")
	}
	for _, binding := range [][2]string{{"", ""}, {"", "getVar:"}, {"", "missing"}, {"missing", "score"}} {
		if monitorEvalForTest(root, binding[0], binding[1], ui.MonitorAppearanceDefault) != nil {
			t.Errorf("unexpected binding for %v", binding)
		}
	}
}

func TestMonitorRefreshLifecycle(t *testing.T) {
	for name, value := range map[string]ui.MonitorValue{
		"scalar": {Text: "42"},
		"list":   {Items: []string{"one", "two"}},
	} {
		t.Run(name, func(t *testing.T) {
			panel := &monitorPanelSpy{}
			m := &Monitor{
				panel: panel, visible: true, isDirty: true, size: 1,
				eval: func() ui.MonitorValue {
					panel.events = append(panel.events, "eval")
					return value
				},
			}
			refresh := func(delta float64, want ...string) {
				t.Helper()
				panel.events = nil
				m.onUpdate(delta)
				if !slices.Equal(panel.events, want) {
					t.Fatalf("refresh(%v) calls = %v, want %v", delta, panel.events, want)
				}
			}
			visible := []string{"eval", "scale", "position", "render", "show"}
			refresh(0, visible...)
			if !reflect.DeepEqual(panel.value, value) {
				t.Fatal("value was lost during refresh")
			}
			refresh(0.1)
			m.SetXYpos(12, -34)
			m.SetSize(2)
			refresh(0, visible...)
			if panel.pos != mathf.NewVec2(12, -34) || panel.scale != 2 || m.updateTimer != 0.1 {
				t.Fatal("dirty refresh changed the timer or lost widget state")
			}
			refresh(0.1, visible...)
			if m.updateTimer != 0 {
				t.Fatal("periodic refresh did not reset the timer")
			}
			m.Hide()
			refresh(0, "hide")
			refresh(0.1)
			refresh(0.1, "hide")
			m.Show()
			refresh(0, visible...)
			m.Show()
			refresh(0)
			refresh(1, visible...)
			refresh(0)
			m.updateTimer = math.NaN()
			refresh(0)
			m.ChangeXpos(1)
			refresh(0, visible...)
		})
	}
}

func TestMonitorGetterVisibilityTakesEffectNextRefresh(t *testing.T) {
	panel := &monitorPanelSpy{}
	m := &Monitor{panel: panel, visible: true, isDirty: true}
	m.eval = func() ui.MonitorValue {
		m.Hide()
		return ui.MonitorValue{Text: "hidden"}
	}
	m.onUpdate(0)
	if !slices.Equal(panel.events, []string{"scale", "position", "render", "show"}) || m.visible || m.isDirty {
		t.Fatal("getter changed the in-progress refresh's visibility")
	}
	panel.events = nil
	m.onUpdate(monitorUpdateIntervalS)
	if !slices.Equal(panel.events, []string{"hide"}) {
		t.Fatal("getter visibility did not take effect on the next refresh")
	}
}

func TestMonitorAppearance(t *testing.T) {
	tests := []struct {
		name  string
		mode  int
		style any
		want  ui.MonitorAppearance
	}{
		{name: "default", mode: 1, want: ui.MonitorAppearanceDefault},
		{name: "default large", mode: 2, want: ui.MonitorAppearanceDefaultLarge},
		{name: "default unsupported mode", mode: 99, want: ui.MonitorAppearanceDefaultLarge},
		{name: "explicit default", mode: 1, style: "default", want: ui.MonitorAppearanceDefault},
		{name: "scratch", mode: 1, style: "scratch", want: ui.MonitorAppearanceCompatible},
		{name: "scratch large", mode: 2, style: "scratch", want: ui.MonitorAppearanceCompatibleLarge},
		{name: "scratch unsupported mode", mode: 99, style: "scratch", want: ui.MonitorAppearanceCompatible},
		{name: "unknown style", mode: 1, style: "custom", want: ui.MonitorAppearanceDefault},
		{name: "invalid style", mode: 2, style: 1.0, want: ui.MonitorAppearanceDefaultLarge},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			shape := coreproject.StageShape{"mode": float64(test.mode)}
			if test.style != nil {
				shape["style"] = test.style
			}
			if got := monitorAppearance(testMonitorShape(t, shape)); got != test.want {
				t.Fatalf("monitorAppearance(%v) = %v, want %v", shape, got, test.want)
			}
		})
	}
}

func TestStageMonitorFixture(t *testing.T) {
	data, err := os.ReadFile("test/StageMonitor/assets/index.json")
	if err != nil {
		t.Fatalf("read StageMonitor fixture: %v", err)
	}
	var project struct {
		ZOrder []coreproject.StageShape `json:"zorder"`
	}
	if err := json.Unmarshal(data, &project); err != nil {
		t.Fatalf("decode StageMonitor fixture: %v", err)
	}

	type monitorFixture struct {
		appearance ui.MonitorAppearance
		target     string
		value      string
		x, y       float64
	}
	want := map[string]monitorFixture{
		"default-implicit": {ui.MonitorAppearanceDefault, "Monkey", "getVar:clicked", -235, 175},
		"default-large":    {ui.MonitorAppearanceDefaultLarge, "", "getVar:downs", -115, 175},
		"scratch-default":  {ui.MonitorAppearanceCompatible, "Monkey", "getVar:clicked", -235, 140},
		"scratch-large":    {ui.MonitorAppearanceCompatibleLarge, "", "getVar:downs", -115, 140},
	}
	for _, shape := range project.ZOrder {
		if shape["type"] != "monitor" {
			continue
		}
		name, _ := shape["name"].(string)
		expected, ok := want[name]
		if !ok {
			t.Errorf("unexpected monitor %q", name)
			continue
		}
		got := monitorFixture{
			appearance: monitorAppearance(testMonitorShape(t, shape)),
			target:     shape["target"].(string),
			value:      shape["val"].(string),
			x:          shape["x"].(float64),
			y:          shape["y"].(float64),
		}
		if got != expected {
			t.Errorf("monitor %q = %+v, want %+v", name, got, expected)
		}
		delete(want, name)
	}
	for name := range want {
		t.Errorf("missing monitor %q", name)
	}
}

func TestTutorialMonitorFixtures(t *testing.T) {
	fixtures := []struct {
		demo   string
		config coreproject.MonitorShape
	}{
		{"02-Dragon", coreproject.MonitorShape{
			Target: "Dragon", Val: "getVar:score", Name: "monitor-1", Label: "score",
			Mode: 1, Style: "scratch", Size: 1, X: -240, Y: 180, Visible: true,
		}},
		{"03-Clone", coreproject.MonitorShape{
			Target: "", Val: "getVar:gid", Name: "monitor-1", Label: "gid",
			Mode: 1, Style: "scratch", Size: 1, X: -240, Y: 180, Visible: true,
		}},
	}
	for _, fixture := range fixtures {
		for _, filename := range []string{"index.json", "index_pack.json"} {
			t.Run(fixture.demo+"/"+filename, func(t *testing.T) {
				data, err := os.ReadFile("tutorial/" + fixture.demo + "/assets/" + filename)
				if err != nil {
					t.Fatal(err)
				}
				var project struct {
					ZOrder []any `json:"zorder"`
				}
				if err := json.Unmarshal(data, &project); err != nil {
					t.Fatal(err)
				}
				monitors := 0
				for _, entry := range project.ZOrder {
					shape, ok := entry.(map[string]any)
					if !ok || shape["type"] != "monitor" {
						continue
					}
					monitors++
					got, err := prepareMonitor(shape)
					if err != nil {
						t.Fatal(err)
					}
					if got.config != fixture.config {
						t.Errorf("monitor config = %+v, want %+v", got.config, fixture.config)
					}
					if got.style.Appearance != ui.MonitorAppearanceCompatible {
						t.Errorf("monitor appearance = %v, want Scratch", got.style.Appearance)
					}
					if got.style.Label != fixture.config.Label || got.style.Color != mathf.NewColorRGBAi(0xee, 0x7d, 0x16, 0xff) {
						t.Errorf("monitor label or color changed: %+v", got.style)
					}
				}
				if monitors != 1 {
					t.Errorf("monitor count = %d, want 1", monitors)
				}
			})
		}
	}
}

func TestMonitorSliderConfig(t *testing.T) {
	for _, mode := range []any{"slider", float64(3)} {
		if got := monitorAppearance(testMonitorShape(t, coreproject.StageShape{"mode": mode})); got != ui.MonitorAppearanceSlider {
			t.Fatalf("mode %v: %v", mode, got)
		}
	}
	for _, tt := range []struct {
		config coreproject.StageShape
		want   ui.MonitorSlider
	}{
		{coreproject.StageShape{}, ui.MonitorSlider{Min: 0, Max: 100, Step: 1}},
		{coreproject.StageShape{"sliderMin": 10.0, "sliderMax": -10.0, "isDiscrete": false}, ui.MonitorSlider{Min: -10, Max: 10, Step: 0.01}},
		{coreproject.StageShape{"sliderMin": math.NaN(), "sliderMax": math.Inf(1)}, ui.MonitorSlider{Min: 0, Max: 100, Step: 1}},
		{coreproject.StageShape{"sliderMin": 5.0, "sliderMax": 5.0}, ui.MonitorSlider{Min: 5, Max: 5, Step: 1}},
		{coreproject.StageShape{"sliderMin": -math.MaxFloat64, "sliderMax": math.MaxFloat64}, ui.MonitorSlider{Min: 0, Max: 100, Step: 1}},
		{coreproject.StageShape{"sliderMax": math.MaxFloat64, "isDiscrete": false}, ui.MonitorSlider{Min: 0, Max: 100, Step: 0.01}},
		{coreproject.StageShape{"sliderMin": "--5", "sliderMax": " 10 ", "isDiscrete": "false"}, ui.MonitorSlider{Min: 0, Max: 100, Step: 1}},
		{coreproject.StageShape{"sliderMin": "-2.5", "sliderMax": "2.5", "isDiscrete": false}, ui.MonitorSlider{Min: -2.5, Max: 2.5, Step: 0.01}},
	} {
		if got := parseMonitorSlider(tt.config); got != tt.want {
			t.Errorf("got %+v, want %+v", got, tt.want)
		}
	}
}

func TestMonitorColorDefaultsAndOverride(t *testing.T) {
	for _, test := range []struct {
		appearance ui.MonitorAppearance
		want       mathf.Color
	}{
		{ui.MonitorAppearanceDefault, mathf.NewColorRGBAi(0x28, 0x9c, 0xfc, 0xff)},
		{ui.MonitorAppearanceCompatible, mathf.NewColorRGBAi(0xff, 0x8c, 0x1a, 0xff)},
		{ui.MonitorAppearanceList, mathf.NewColorRGBAi(0xff, 0x66, 0x1a, 0xff)},
	} {
		for _, invalid := range []any{nil, "invalid color"} {
			if got := parseMonitorColor(coreproject.StageShape{"color": invalid}, test.appearance); got != test.want {
				t.Errorf("appearance %v: color = %v, want %v", test.appearance, got, test.want)
			}
		}
		want := mathf.NewColorRGBAi(0x12, 0x34, 0x56, 0xff)
		if got := parseMonitorColor(coreproject.StageShape{"color": "#123456"}, test.appearance); got != want {
			t.Errorf("appearance %v: explicit color = %v, want %v", test.appearance, got, want)
		}
	}
}

func TestMonitorSliderWritesBeforeRefresh(t *testing.T) {
	game := monitorEvalFixture{score: 25}
	root := reflect.ValueOf(&game).Elem()
	binding, err := bindMonitor(root, "", "score", ui.MonitorAppearanceSlider)
	if err != nil {
		t.Fatal(err)
	}
	panel := &monitorPanelSpy{}
	pending := false
	monitor := Monitor{
		visible: true, panel: panel,
		eval: binding.read,
		updateInput: func() bool {
			if !pending {
				return false
			}
			pending = false
			return binding.write(37)
		},
	}
	pending = true
	monitor.onUpdate(0.001)
	if game.score != 37 || panel.value.Text != "37" {
		t.Fatal("input did not update variable and label in the same frame")
	}
	monitor.visible = false
	pending = true
	monitor.onUpdate(0.001)
	if !pending {
		t.Fatal("hidden slider consumed input")
	}
}

func TestMonitorSliderTargetScope(t *testing.T) {
	game := SliderMonitorGame{score: 1}
	game.Dot.SliderMonitorGame = &game
	game.Dot.score = 2
	root := reflect.ValueOf(&game).Elem()
	binding, err := bindMonitor(root, "Dot", "getVar:score", ui.MonitorAppearanceSlider)
	if err != nil || !binding.write(7.5) || game.Dot.score != 7.5 || game.score != 1 {
		t.Fatal("sprite-local slider wrote the wrong scope")
	}
	eval := binding.read
	if eval == nil || eval().Text != "7.5" {
		t.Fatal("slider reader and writer resolve different variables")
	}
	binding, err = bindMonitor(root, "", "score", ui.MonitorAppearanceSlider)
	if err != nil || !binding.write(9) || game.score != 9 || game.Dot.score != 7.5 {
		t.Fatal("stage slider wrote the wrong scope")
	}
}

func TestParseMonitorModes(t *testing.T) {
	shape := coreproject.StageShape{"target": "", "val": "score", "name": "score", "label": "score", "x": 0.0, "y": 0.0, "visible": true}
	for _, mode := range []any{1.0, 2.0, 3.0, 4.0, "slider", "list"} {
		shape["mode"] = mode
		if _, err := coreproject.ParseMonitorShape(shape); err != nil {
			t.Errorf("mode %v: %v", mode, err)
		}
	}
	for _, mode := range []any{nil, true, "unknown"} {
		shape["mode"] = mode
		if _, err := coreproject.ParseMonitorShape(shape); err == nil {
			t.Errorf("accepted invalid mode %v", mode)
		}
	}
}

func TestMonitorBindingCapabilities(t *testing.T) {
	game := monitorEvalFixture{score: 1}
	root := reflect.ValueOf(&game).Elem()
	for _, appearance := range []ui.MonitorAppearance{ui.MonitorAppearanceDefault, ui.MonitorAppearanceCompatibleLarge} {
		binding, err := bindMonitor(root, "", "score", appearance)
		if err != nil || binding.read == nil || binding.write != nil {
			t.Fatal("display-only monitor gained a writer")
		}
	}
	for _, tt := range []struct {
		target, name string
		want         error
	}{
		{"", "value", syscall.EINVAL},
		{"", "Value", syscall.EINVAL},
		{"", "missing", syscall.ENOENT},
		{"missing", "score", syscall.ENOENT},
		{"", "getVar:", syscall.ENOENT},
	} {
		binding, err := bindMonitor(root, tt.target, tt.name, ui.MonitorAppearanceSlider)
		if err != tt.want || binding.read != nil || binding.write != nil {
			t.Errorf("binding %q/%q: error %v, want %v without partial binding", tt.target, tt.name, err, tt.want)
		}
	}
	if game.calls != 0 {
		t.Fatal("binding invoked a reporter")
	}
}

func TestMonitorConstructionAndWidgetTransforms(t *testing.T) {
	manager := setupGameWidgetUi(t)
	game := &monitorEvalFixture{score: 5}
	prepared, err := prepareMonitor(coreproject.StageShape{"name": "score", "val": "score", "label": "Score", "target": "", "mode": "slider", "x": 1.0, "y": 2.0, "visible": true})
	if err != nil {
		t.Fatal(err)
	}
	monitor, err := newMonitor(reflect.ValueOf(game).Elem(), prepared)
	if err != nil {
		t.Fatal(err)
	}
	if monitor.GetName() != "score" || monitor.updateInput == nil {
		t.Fatalf("monitor=%+v", monitor)
	}
	monitor.SetXpos(10)
	monitor.SetYpos(20)
	monitor.ChangeYpos(-3)
	monitor.ChangeXYpos(4, 5)
	monitor.SetSize(2)
	monitor.ChangeSize(0.5)
	if monitor.Xpos() != 14 || monitor.Ypos() != 22 || monitor.Size() != 2.5 || !monitor.isDirty {
		t.Fatalf("widget transform=%+v", monitor)
	}
	monitor.onUpdate(0)
	if monitor.updateInput() {
		t.Fatal("unchanged slider reports input")
	}
	manager.rangeValue = 42
	if !monitor.updateInput() || game.score != 42 {
		t.Fatalf("slider input=%g", game.score)
	}
	prepared.style.Appearance = ui.MonitorAppearanceDefault
	plain, err := newMonitor(reflect.ValueOf(game).Elem(), prepared)
	if err != nil || plain.updateInput != nil {
		t.Fatalf("plain monitor=%+v, err=%v", plain, err)
	}
}

func TestListMonitorItems(t *testing.T) {
	items := []any{"苹果", 1.23456789, true, nil, NewValue("[b]text[/b]\nnext")}
	list := NewList(items...)
	var retained List
	retained.Init(items...)
	for _, value := range []any{list, &list, retained, &retained, items} {
		want := []string{"苹果", "1.23456789", "true", "", "[b]text[/b]\nnext"}
		if got := listMonitorItems(value); !slices.Equal(got, want) {
			t.Errorf("listMonitorItems(%T) = %q, want %q", value, got, want)
		}
	}
	for _, tt := range []struct {
		value   any
		wantNil bool
	}{
		{nil, true},
		{(*List)(nil), true},
		{[]string(nil), false},
		{NewList(), false},
		{List{}, false},
		{42, true},
	} {
		if got := listMonitorItems(tt.value); len(got) != 0 || (got == nil) != tt.wantNil {
			t.Errorf("listMonitorItems(%T) = %#v, want empty with nil=%v", tt.value, got, tt.wantNil)
		}
	}
	array := [2]int{1, 2}
	for _, value := range []any{array, &array, []int{1, 2}} {
		if got := listMonitorItems(value); !slices.Equal(got, []string{"1", "2"}) {
			t.Errorf("listMonitorItems(%T) = %q", value, got)
		}
	}
}

func TestListMonitorEvalTracksChanges(t *testing.T) {
	g := ListMonitorGame{items: NewList("first")}
	g.Monkey.ListMonitorGame = &g
	g.Monkey.items = NewList("sprite")
	root := reflect.ValueOf(&g).Elem()
	for _, name := range []string{"items", "getVar:items", "current", "Current"} {
		t.Run(name, func(t *testing.T) {
			g.items = NewList("first")
			eval := monitorEvalForTest(root, "", name, ui.MonitorAppearanceList)
			if eval == nil {
				t.Fatal("list binding failed")
			}
			before := eval().Items
			g.items.Append("second")
			g.items.Set(0, "changed")
			if got := eval().Items; !slices.Equal(got, []string{"changed", "second"}) {
				t.Fatalf("after append/set: %q", got)
			}
			if !slices.Equal(before, []string{"first"}) {
				t.Fatalf("snapshot changed: %q", before)
			}
			g.items.Delete(0)
			if got := eval().Items; !slices.Equal(got, []string{"second"}) {
				t.Fatalf("after delete: %q", got)
			}
			g.items = NewList("replacement")
			if got := eval().Items; !slices.Equal(got, []string{"replacement"}) {
				t.Fatalf("after replacement: %q", got)
			}
			g.items.Clear()
			if len(eval().Items) != 0 {
				t.Fatal("clear not reflected")
			}
		})
	}
	spriteEval := monitorEvalForTest(root, "Monkey", "getVar:items", ui.MonitorAppearanceList)
	if spriteEval == nil || !slices.Equal(spriteEval().Items, []string{"sprite"}) {
		t.Fatal("sprite-local list binding failed")
	}
	globalEval := monitorEvalForTest(root, "Monkey", "values", ui.MonitorAppearanceList)
	g.values = []string{"shared"}
	if globalEval == nil || !slices.Equal(globalEval().Items, g.values) {
		t.Fatal("promoted global slice binding failed")
	}
	g.values = append(g.values, "grown")
	if !slices.Equal(globalEval().Items, g.values) {
		t.Fatal("slice growth not reflected")
	}
	for _, binding := range [][2]string{{"", "getVar:"}, {"", "missing"}, {"missing", "items"}} {
		if monitorEvalForTest(root, binding[0], binding[1], ui.MonitorAppearanceList) != nil {
			t.Errorf("unexpected binding for %v", binding)
		}
	}
}

func TestListMonitorConfiguration(t *testing.T) {
	for _, mode := range []any{"list", float64(4)} {
		if got := monitorAppearance(testMonitorShape(t, coreproject.StageShape{"mode": mode})); got != ui.MonitorAppearanceList {
			t.Errorf("mode %v: %v", mode, got)
		}
	}
	for _, tt := range []struct {
		width, height any
		want          mathf.Vec2
	}{
		{nil, nil, mathf.NewVec2(100, 200)},
		{0.0, 0.0, mathf.NewVec2(100, 200)},
		{180.0, 240.0, mathf.NewVec2(180, 240)},
		{1.0, 1.0, mathf.NewVec2(100, 60)},
		{math.NaN(), math.Inf(1), mathf.NewVec2(100, 200)},
	} {
		if got := parseListMonitorDimensions(coreproject.StageShape{"width": tt.width, "height": tt.height}); got != tt.want {
			t.Errorf("dimensions(%v, %v) = %v, want %v", tt.width, tt.height, got, tt.want)
		}
	}
}

func TestListMonitorVisibility(t *testing.T) {
	g := &Game{}
	global := &Monitor{target: "", val: "getVar:items"}
	local := &Monitor{target: "Monkey", val: "items"}
	g.shapeMgr.add(global)
	g.shapeMgr.add(local)
	g.ShowVar("items")
	if !global.Visible() || local.Visible() || !global.isDirty {
		t.Fatal("ShowVar did not select the global list")
	}
	g.HideVar("items")
	if global.Visible() {
		t.Fatal("HideVar did not hide the list")
	}
	if !g.setStageMonitor("Monkey", "getVar:items", true) || !local.Visible() || global.Visible() {
		t.Fatal("sprite list visibility mismatch")
	}
}

func TestSpriteVariableMonitorsPreferLocalThenStage(t *testing.T) {
	game := &Game{}
	sprite := &SpriteImpl{g: game, name: "actor"}
	local := &Monitor{target: "actor", val: "score"}
	global := &Monitor{target: "", val: "score"}
	fallback := &Monitor{target: "", val: "lives"}
	game.shapeMgr.items = []Shape{local, global, fallback}
	sprite.ShowVar("score")
	if !local.visible || global.visible {
		t.Fatal("local show changed the shadowed stage monitor")
	}
	sprite.HideVar("score")
	if local.visible {
		t.Fatal("local monitor remained visible")
	}
	sprite.ShowVar("lives")
	if !fallback.visible {
		t.Fatal("stage fallback was not shown")
	}
	sprite.HideVar("lives")
	if fallback.visible {
		t.Fatal("stage fallback remained visible")
	}
}

func (g *monitorEvalFixture) Value() float64 {
	g.calls++
	return g.score
}

func (p *monitorPanelSpy) Render(value ui.MonitorValue) {
	p.events = append(p.events, "render")
	p.value = value
}

func (p *monitorPanelSpy) SetVisible(visible bool) {
	p.visible = visible
	if visible {
		p.events = append(p.events, "show")
	} else {
		p.events = append(p.events, "hide")
	}
}

func (p *monitorPanelSpy) UpdateScale(scale float64) {
	p.events = append(p.events, "scale")
	p.scale = scale
}

func (p *monitorPanelSpy) UpdatePos(pos mathf.Vec2) {
	p.events = append(p.events, "position")
	p.pos = pos
}

func (*gameWidgetUiMgr) SetScale(pkgengine.Object, mathf.Vec2) {}

func (*gameWidgetUiMgr) GetCanvasScale(pkgengine.Object) mathf.Vec2 { return mathf.NewVec2(1, 1) }

func (*gameWidgetUiMgr) SetGlobalPosition(pkgengine.Object, mathf.Vec2) {}

func (*gameWidgetUiMgr) SetColor(pkgengine.Object, mathf.Color) {}

func (*gameWidgetUiMgr) SetSize(pkgengine.Object, mathf.Vec2) {}

func (*gameWidgetUiMgr) SetRotation(pkgengine.Object, float64) {}

func (m *gameWidgetUiMgr) SetRange(_ pkgengine.Object, _, _, _, value float64) { m.rangeValue = value }

func (m *gameWidgetUiMgr) GetRangeValue(pkgengine.Object) float64 { return m.rangeValue }

func (g *ListMonitorGame) Current() *List { return &g.items }

func monitorEvalForTest(g reflect.Value, target, val string, appearance ui.MonitorAppearance) func() ui.MonitorValue {
	binding, _ := bindMonitor(g, target, val, appearance)
	return binding.read
}

func testMonitorShape(t *testing.T, fields coreproject.StageShape) coreproject.MonitorShape {
	t.Helper()
	shape := coreproject.StageShape{
		"target": "", "val": "score", "name": "score", "label": "score",
		"mode": 1.0, "x": 0.0, "y": 0.0, "visible": true,
	}
	for key, value := range fields {
		shape[key] = value
	}
	parsed, err := coreproject.ParseMonitorShape(shape)
	if err != nil {
		t.Fatal(err)
	}
	return parsed
}

func setupGameWidgetUi(t *testing.T) *gameWidgetUiMgr {
	t.Helper()
	base := installAskTestUiMgr(t)
	previousCamera := pkgengine.CameraMgr
	pkgengine.CameraMgr = bubbleLayoutCameraMgr{}
	t.Cleanup(func() { pkgengine.CameraMgr = previousCamera })
	manager := &gameWidgetUiMgr{askTestUiMgr: base}
	pkgengine.UiMgr = manager
	return manager
}
