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
	"github.com/goplus/spx/v4/internal/coroutine"
	"github.com/goplus/spx/v4/internal/engine"
	itime "github.com/goplus/spx/v4/internal/time"
	"github.com/goplus/spx/v4/internal/ui"
	pkgengine "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

func installBubbleTestUiMgr(t *testing.T) {
	t.Helper()
	installAskTestUiMgr(t)
}

func newBubbleTestSprite() (*Game, *SpriteImpl, *bubbleComponent) {
	game := &Game{}
	game.shapeMgr.init()
	sprite := &SpriteImpl{g: game}
	sprite.spriteState.IsVisible = true
	component := &bubbleComponent{sprite: sprite}
	sprite.components.bubble = component
	return game, sprite, component
}

func TestBubbleEntryPointsCreateAndRenewBubbles(t *testing.T) {
	installBubbleTestUiMgr(t)
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

func TestTimedBubbleEntryPointsScheduleRemoval(t *testing.T) {
	installBubbleTestUiMgr(t)
	co := setupRuntimeScheduler(t)
	itime.Start(nil)
	_, sprite, component := newBubbleTestSprite()

	done := false
	thread := co.Create(sprite, func(coroutine.Thread) {
		sprite.Say__1("say", 0.01)
		sprite.Think__1("think", 0.01)
		sprite.Quote__3("quote", "description", 0.01)
		done = true
	})
	co.JoinFirstSlice(thread)
	co.Update()
	if done {
		t.Fatal("timed bubble calls completed in their issuing frame")
	}
	for range 3 {
		itime.Update(1.0/30, 30)
		co.Update()
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
	_, game := setupRuntimeEventGame(t)
	sprite := &SpriteImpl{}
	sprite.spriteState.IsVisible = true
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
	var game Game
	game.shapeMgr.init()
	sprite := &SpriteImpl{g: &game}
	component := &bubbleComponent{sprite: sprite}
	sprite.components.bubble = component
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

	bubble.flushPendingRemoval(10)
	if component.textObj != bubble || game.shapeMgr.findShapeIndex(bubble) < 0 {
		t.Fatal("timed bubble was removed during its expiration frame")
	}

	bubble.flushPendingRemoval(11)
	if component.textObj != nil || game.shapeMgr.findShapeIndex(bubble) >= 0 {
		t.Fatal("timed bubble was not removed on the following frame")
	}
}

func TestTextBubbleTokenOnlyExpiresCurrentGeneration(t *testing.T) {
	bubble := &textBubble{bubbleBase: bubbleBase{generation: 2}}
	component := &bubbleComponent{textObj: bubble}

	component.expireText(bubbleToken{bubble: &bubble.bubbleBase, generation: 1})
	if bubble.timedRemovalPending {
		t.Fatal("stale token scheduled removal of a newer bubble generation")
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
	var game Game
	game.shapeMgr.init()
	sprite := &SpriteImpl{g: &game}
	component := &bubbleComponent{sprite: sprite}
	sprite.components.bubble = component
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

	quote.flushPendingRemoval(10)
	if component.quoteObj != quote || game.shapeMgr.findShapeIndex(quote) < 0 {
		t.Fatal("timed quote was removed during its expiration frame")
	}

	quote.flushPendingRemoval(11)
	if component.quoteObj != nil || game.shapeMgr.findShapeIndex(quote) >= 0 {
		t.Fatal("timed quote was not removed on the following frame")
	}
}

func TestQuoteBubbleTokenOnlyExpiresCurrentGeneration(t *testing.T) {
	quote := &quoterBubble{bubbleBase: bubbleBase{generation: 2}}
	component := &bubbleComponent{quoteObj: quote}

	component.expireQuote(bubbleToken{bubble: &quote.bubbleBase, generation: 1})
	if quote.timedRemovalPending {
		t.Fatal("stale token scheduled removal of a newer quote generation")
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

type bubbleLayoutCameraMgr struct{ pkgengine.ICameraMgr }

func (bubbleLayoutCameraMgr) GetCameraPosition() mathf.Vec2 { return mathf.Vec2{} }
func (bubbleLayoutCameraMgr) GetCameraZoom() mathf.Vec2     { return mathf.NewVec2(1, 1) }

func TestLayoutTextBubblesTracksPointerIdentity(t *testing.T) {
	setupImmediateEngineCalls(t)
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
	second.sprite.spriteState.IsVisible = false
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
					bubbles[removed].sprite.spriteState.IsVisible = false
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

func (m *askTestUiMgr) newID() pkgengine.Object {
	m.nextID++
	m.ids[m.nextID] = struct{}{}
	return m.nextID
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

func (*askTestUiMgr) DestroyNode(pkgengine.Object) bool { return true }

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
				sprite.spriteState.IsVisible = test.spriteVisible
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

type bubbleRenderTestUiMgr struct {
	*askTestUiMgr
	positions map[pkgengine.Object]mathf.Vec2
	sizes     map[pkgengine.Object]mathf.Vec2
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

func (bubbleLayoutCameraMgr) GetViewportRect() mathf.Rect2 {
	return mathf.NewRect2(0, 0, 480, 360)
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
	sprite.spriteState.IsVisible = true
	bubble := &textBubble{bubbleBase: bubbleBase{sprite: sprite, camera: &cameraImpl{}, isDirty: true}, panel: ui.NewUiSay(), msg: "first", style: ui.StyleSay}
	bubble.onUpdate(0)
	if bubble.isDirty || !slices.Contains(bubbleRenderedTexts(mgr.text), "first") {
		t.Fatalf("text bubble was not rendered: dirty=%v text=%v", bubble.isDirty, mgr.text)
	}
	layout := ui.NewSayBubbleLayoutContext(mathf.NewVec2(480, 360)).NewLayout(1, mathf.Vec2{}, mathf.NewVec2(20, 20), ui.NewSayBubbleContent("planned", ui.StyleThink))
	bubble.setLayout(layout)
	bubble.onUpdate(0)
	if bubble.isDirty || !slices.Contains(bubbleRenderedTexts(mgr.text), "planned") {
		t.Fatal("precomputed layout did not replace the visible text")
	}
	sprite.spriteState.IsVisible = false
	bubble.markDirty()
	bubble.onUpdate(0)
	if !bubble.isDirty {
		t.Fatal("hidden bubble consumed its pending visual update")
	}
	sprite.spriteState.IsVisible = true
	quote := &quoterBubble{bubbleBase: bubbleBase{sprite: sprite, camera: bubble.camera, isDirty: true}, panel: ui.NewUiQuote(), message: "quotation", description: "speaker"}
	quote.onUpdate(0)
	container := mgr.bound[askTestBinding{parent: quote.panel.GetId(), path: "C"}]
	_, size := quote.getBounds()
	if mgr.sizes[container] != size.Addf(20) || !slices.Contains(bubbleRenderedTexts(mgr.text), "quotation") || !slices.Contains(bubbleRenderedTexts(mgr.text), "speaker") || quote.isDirty {
		t.Fatalf("quote layout = size %v, text %v, dirty %v", mgr.sizes[container], mgr.text, quote.isDirty)
	}
}

func bubbleRenderedTexts(values map[pkgengine.Object]string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	return out
}

type askDelayedInputMgr struct{ pkgengine.IInputMgr }

func (askDelayedInputMgr) GetKey(int64) bool { return false }

func TestAskWaitsForAnswerAcrossFrames(t *testing.T) {
	mgr := installAskTestUiMgr(t)
	originalInput := pkgengine.InputMgr
	pkgengine.InputMgr = askDelayedInputMgr{}
	t.Cleanup(func() { pkgengine.InputMgr = originalInput })
	t.Cleanup(itime.ClearTimers)
	co := setupRuntimeScheduler(t)
	itime.Start(nil)
	game := &Game{}
	game.shapeMgr.init()
	answered := false
	thread := co.Create(game, func(coroutine.Thread) { game.Ask("later?"); answered = true })
	co.JoinFirstSlice(thread)
	if answered || game.dialogState.AskPanel == nil || !mgr.visible[mgr.askRoot] {
		t.Fatal("Ask did not suspend while displaying its question")
	}
	game.dialogState.AskPanel.OnCheck("later answer")
	itime.Update(1.0/30, 30)
	co.Update()
	if !answered || game.Answer() != "later answer" {
		t.Fatalf("Ask resumed=%v, answer=%q", answered, game.Answer())
	}
	co.Join(thread)
}
