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
	"reflect"
	"testing"

	"github.com/goplus/spbase/mathf"
	coreproject "github.com/goplus/spx/v4/internal/core/project"
	"github.com/goplus/spx/v4/internal/coroutine"
	pkgengine "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

func TestSpriteSetDragMode(t *testing.T) {
	var sprite SpriteImpl
	if sprite.spriteState.isDraggable {
		t.Fatal("sprites must default to not draggable")
	}
	for _, mode := range []DragMode{NotDraggable, DragMode(-1), DragMode(2)} {
		sprite.SetDragMode(Draggable)
		if !sprite.spriteState.isDraggable {
			t.Fatal("draggable mode did not enable dragging")
		}
		sprite.SetDragMode(mode)
		if sprite.spriteState.isDraggable {
			t.Fatalf("mode %v did not disable dragging", mode)
		}
	}
}

func TestSpriteDragModeLoadsAndClones(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		name := "not draggable"
		config := `{"costumes":[{"name":"idle","imageWidth":10,"imageHeight":10}],"isDraggable":false}`
		if enabled {
			name = "draggable"
			config = `{"costumes":[{"name":"idle","imageWidth":10,"imageHeight":10}],"isDraggable":true}`
		}
		t.Run(name, func(t *testing.T) {
			files := gameTestFS{"sprites/owner/index.json": config}
			game := setupStageLoadTestGame(t, files)
			gamer := reflect.ValueOf(&game.Game).Elem()
			sprite := &layoutInterfaceOwnerSprite{}
			prepared, err := prepareSprite(files, "owner", reflect.TypeOf(*sprite), gamer)
			if err != nil {
				t.Fatal(err)
			}
			game.loadPreparedSprite(sprite, "owner", gamer, &prepared)
			if sprite.spriteState.isDraggable != enabled {
				t.Fatalf("loaded draggable = %v, want %v", sprite.spriteState.isDraggable, enabled)
			}
			var clone SpriteImpl
			clone.InitFrom(&sprite.SpriteImpl)
			if clone.spriteState.isDraggable != enabled {
				t.Fatalf("cloned draggable = %v, want %v", clone.spriteState.isDraggable, enabled)
			}
			clone.SetDragMode(Draggable)
			sprite.SetDragMode(NotDraggable)
			if !clone.spriteState.isDraggable || sprite.spriteState.isDraggable {
				t.Fatal("changing the original's drag mode affected the clone")
			}
		})
	}
}

func TestSpriteDragFramesPreserveOffsetAndRelease(t *testing.T) {
	co, game, sprite := setupSpriteDragTest(t)
	other := newDragTestSprite(game, "foreground", 2)
	game.shapeMgr.add(other)
	start := mathf.NewVec2(12, 24)
	dispatchSpriteDragFrame(co, game, start, &eventLeftButtonDown{Pos: start})
	assertSpriteDragPosition(t, sprite, 10, 20)
	if game.inputMgr.drag.active {
		t.Fatal("stationary press started a drag")
	}
	dispatchSpriteDragFrame(co, game, mathf.NewVec2(42, 64))
	assertSpriteDragPosition(t, sprite, 40, 60)
	if !game.inputMgr.drag.active {
		t.Fatal("pointer movement did not start dragging")
	}
	shapes := game.shapeMgr.getTempShapes()
	if shapes[len(shapes)-1] != sprite {
		t.Fatal("dragged sprite was not brought to the front")
	}
	end := mathf.NewVec2(52, 74)
	dispatchSpriteDragFrame(co, game, end, &eventLeftButtonUp{Pos: end})
	assertSpriteDragPosition(t, sprite, 50, 70)
	if game.inputMgr.drag.target != nil || game.inputMgr.drag.active {
		t.Fatal("release retained the drag")
	}
	dispatchSpriteDragFrame(co, game, mathf.NewVec2(80, 90))
	assertSpriteDragPosition(t, sprite, 50, 70)
}

func TestSpriteDragClickAndSwipeRouting(t *testing.T) {
	for _, moved := range []bool{false, true} {
		name := "click"
		if moved {
			name = "drag"
		}
		t.Run(name, func(t *testing.T) {
			co, game, sprite := setupSpriteDragTest(t)
			clicks, swipes := 0, 0
			sprite.OnClick(func() { clicks++ })
			sprite.OnSwipe__0(90, func() { swipes++ })
			game.OnClick(func() { t.Error("sprite gesture fell through to stage click") })
			game.OnSwipe__0(90, func() { t.Error("sprite drag fell through to stage swipe") })
			point := mathf.NewVec2(10, 20)
			dispatchSpriteDragFrame(co, game, point, &eventLeftButtonDown{Pos: point})
			if clicks != 0 {
				t.Fatal("draggable sprite clicked on mouse down")
			}
			if moved {
				point.X += 100
				dispatchSpriteDragFrame(co, game, point)
			}
			dispatchSpriteDragFrame(co, game, point, &eventLeftButtonUp{Pos: point})
			wantClicks := 1
			if moved {
				wantClicks = 0
			}
			if clicks != wantClicks || swipes != 0 {
				t.Fatalf("clicks/swipes = %d/%d, want %d/0", clicks, swipes, wantClicks)
			}
		})
	}
}

func TestSpriteDragIgnoresNonDraggableForeground(t *testing.T) {
	co, game, sprite := setupSpriteDragTest(t)
	top := newDragTestSprite(game, "top", 2)
	game.shapeMgr.add(top)
	start := mathf.NewVec2(10, 20)
	dispatchSpriteDragFrame(co, game, start, &eventLeftButtonDown{Pos: start})
	dispatchSpriteDragFrame(co, game, mathf.NewVec2(30, 40))
	assertSpriteDragPosition(t, sprite, 30, 40)
	assertSpriteDragPosition(t, top, 10, 20)
}

func TestSpriteDragCancellation(t *testing.T) {
	for _, test := range []struct {
		name   string
		cancel func(*Game, *SpriteImpl)
	}{
		{"disable", func(_ *Game, s *SpriteImpl) { s.SetDragMode(NotDraggable) }},
		{"hide", func(_ *Game, s *SpriteImpl) { s.Hide() }},
		{"destroy", func(_ *Game, s *SpriteImpl) { s.destroy() }},
		{"reset input", func(g *Game, _ *SpriteImpl) { g.inputMgr.init(g) }},
	} {
		t.Run(test.name, func(t *testing.T) {
			co, game, sprite := setupSpriteDragTest(t)
			clicks := 0
			sprite.OnClick(func() { clicks++ })
			start := mathf.NewVec2(10, 20)
			dispatchSpriteDragFrame(co, game, start, &eventLeftButtonDown{Pos: start})
			dispatchSpriteDragFrame(co, game, mathf.NewVec2(30, 40))
			test.cancel(game, sprite)
			if game.inputMgr.drag.target != nil || game.inputMgr.drag.active {
				t.Fatal("cancellation retained the drag")
			}
			end := mathf.NewVec2(50, 60)
			dispatchSpriteDragFrame(co, game, end, &eventLeftButtonUp{Pos: end})
			assertSpriteDragPosition(t, sprite, 30, 40)
			if clicks != 0 {
				t.Fatal("cancelled drag generated a click")
			}
		})
	}
}

func TestSpriteDragReplayUsesRecordedPointerAndButtonState(t *testing.T) {
	for _, withEdges := range []bool{false, true} {
		name := "held snapshots"
		if withEdges {
			name = "ordered edges"
		}
		t.Run(name, func(t *testing.T) {
			resetInputSessionTest(t)
			co, game, sprite := setupSpriteDragTest(t)
			live := setupInputSourceManager(t)
			live.point = mathf.NewVec2(-200, -100)
			points := []mathf.Vec2{{X: 12, Y: 24}, {X: 42, Y: 64}, {X: 52, Y: 74}, {X: 80, Y: 90}}
			frames := make([]InputReplayFrame, len(points))
			for i, point := range points {
				buttons := uint8(0)
				if i < 2 {
					buttons = 1
				}
				frames[i] = InputReplayFrame{
					Frame: int64(i), Time: float64(i) / 30,
					State: InputReplayState{Mouse: InputReplayMouse{X: point.X, Y: point.Y}, Buttons: buttons},
				}
			}
			if withEdges {
				frames[0].MouseEvents = []InputReplayMouseEvent{{Button: 1, Pressed: true}}
				frames[2].MouseEvents = []InputReplayMouseEvent{{Button: 1, Pressed: false}}
			}
			if _, err := PrepareInputReplay(validRuntimeReplay(frames...)); err != nil {
				t.Fatal(err)
			}
			session := claimPreparedSession(t, game)
			t.Cleanup(func() { game.abortInputSession("test finished") })
			want := []mathf.Vec2{{X: 10, Y: 20}, {X: 40, Y: 60}, {X: 50, Y: 70}, {X: 50, Y: 70}}
			for i := range frames {
				if !game.inputMgr.prepareInputFrame(1.0 / 30) {
					t.Fatalf("replay frame %d was not prepared", i)
				}
				game.dispatchFrameEvents()
				co.Update()
				session.endFrame()
				assertSpriteDragPosition(t, sprite, want[i].X, want[i].Y)
			}
			if game.inputMgr.drag.target != nil {
				t.Fatal("replayed release retained the drag")
			}
		})
	}
}

func TestSpriteDragResetsAcrossReplayTransitions(t *testing.T) {
	resetInputSessionTest(t)
	co, game, sprite := setupSpriteDragTest(t)
	live := setupInputSourceManager(t)
	start := mathf.NewVec2(10, 20)
	dispatchSpriteDragFrame(co, game, start, &eventLeftButtonDown{Pos: start})
	dispatchSpriteDragFrame(co, game, mathf.NewVec2(30, 40))
	initial := InputReplayState{Mouse: InputReplayMouse{X: 80, Y: 90}, Buttons: 1}
	replay := validRuntimeReplay(InputReplayFrame{Frame: 0, State: initial})
	replay.Initial = initial
	if _, err := PrepareInputReplay(replay); err != nil {
		t.Fatal(err)
	}
	session := claimPreparedSession(t, game)
	t.Cleanup(func() { game.abortInputSession("test finished") })
	if !game.inputMgr.prepareInputFrame(1.0 / 30) {
		t.Fatal("first replay frame was not prepared")
	}
	game.dispatchFrameEvents()
	co.Update()
	session.endFrame()
	if game.inputMgr.drag.target != nil {
		t.Fatal("first replay tick retained a drag from live input")
	}
	assertSpriteDragPosition(t, sprite, 30, 40)

	game.inputMgr.onLeftButtonDown(mathf.NewVec2(30, 40))
	game.inputMgr.updateDrag(mathf.NewVec2(50, 60))
	game.abortInputSession("return to live input")
	live.point = mathf.NewVec2(-200, -100)
	if !game.inputMgr.prepareInputFrame(1.0 / 30) {
		t.Fatal("live handoff frame was not prepared")
	}
	game.dispatchFrameEvents()
	co.Update()
	if game.inputMgr.drag.target != nil {
		t.Fatal("live handoff retained a drag from replay input")
	}
	assertSpriteDragPosition(t, sprite, 50, 60)
}

func TestSpriteDragTouchingIsDirectionalUntilRelease(t *testing.T) {
	for _, named := range []bool{false, true} {
		name := "sprite"
		if named {
			name = "sprite name"
		}
		t.Run(name, func(t *testing.T) {
			game, _, receiver, target := newTouchingPair(t, false)
			game.inputMgr.init(game)
			target.SetDragMode(Draggable)
			touchTarget := func() bool { return receiver.Touching__0(target) }
			touchReceiver := func() bool { return target.Touching__0(receiver) }
			if named {
				touchTarget = func() bool { return receiver.Touching__1("target") }
				touchReceiver = func() bool { return target.Touching__1("receiver") }
			}
			if !touchTarget() || !touchReceiver() {
				t.Fatal("overlapping sprites did not touch before dragging")
			}
			game.inputMgr.drag = spriteDragState{target: &target.SpriteImpl, active: true}
			if touchTarget() {
				t.Fatal("another sprite detected a dragged target")
			}
			if !touchReceiver() {
				t.Fatal("dragged sprite could not detect another sprite")
			}
			game.inputMgr.onLeftButtonUp(mathf.Vec2{})
			if !touchTarget() || !touchReceiver() {
				t.Fatal("sensing did not recover after release")
			}
		})
	}
}

func TestSpriteDragBlocksScriptMotionWithoutDrawingPen(t *testing.T) {
	spy := &dragPenSpy{spyPenMgr: setupSpyPenMgr(t)}
	pkgengine.PenMgr = spy
	co, game, sprite := setupSpriteDragTest(t)
	sprite.PenDown()
	initialPenMoves := spy.moveCalls
	initialPenDowns, initialPenUps := spy.penDownCalls, spy.penUpCalls
	start := mathf.NewVec2(10, 20)
	dispatchSpriteDragFrame(co, game, start, &eventLeftButtonDown{Pos: start})
	dispatchSpriteDragFrame(co, game, mathf.NewVec2(30, 40))
	for _, move := range []func(){
		func() { sprite.SetXYpos(90, 100) },
		func() { sprite.SetXpos(90) },
		func() { sprite.SetYpos(100) },
		func() { sprite.ChangeXYpos(90, 100) },
		func() { sprite.ChangeXpos(90) },
		func() { sprite.ChangeYpos(100) },
		func() { sprite.Move__0(10) },
	} {
		move()
		assertSpriteDragPosition(t, sprite, 30, 40)
	}
	if spy.moveCalls != initialPenMoves {
		t.Fatalf("drag produced %d pen movements, want none", spy.moveCalls-initialPenMoves)
	}
	end := mathf.NewVec2(50, 60)
	dispatchSpriteDragFrame(co, game, end, &eventLeftButtonUp{Pos: end})
	if spy.moveCalls != initialPenMoves {
		t.Fatal("drag release drew a pen stroke")
	}
	sprite.SetXYpos(70, 80)
	assertSpriteDragPosition(t, sprite, 70, 80)
	wantMoves := []dragPenMove{
		{position: mathf.NewVec2(50, 60), reset: true},
		{position: mathf.NewVec2(70, 80), drawing: true},
	}
	if got := spy.moves[initialPenMoves:]; !reflect.DeepEqual(got, wantMoves) {
		t.Fatalf("pen movements after release = %v, want %v", got, wantMoves)
	}
	if spy.penDownCalls != initialPenDowns || spy.penUpCalls != initialPenUps || !spy.down {
		t.Fatal("resuming after drag toggled the pen and could draw an extra point")
	}
	if spy.restoreCalls != 1 || !spy.lastRestoreDown {
		t.Fatal("resuming after drag did not restore the pen-down state")
	}
}

func TestSpriteDragExplicitPenDownStillDrawsPoint(t *testing.T) {
	spy := &dragPenSpy{spyPenMgr: setupSpyPenMgr(t)}
	pkgengine.PenMgr = spy
	co, game, sprite := setupSpriteDragTest(t)
	sprite.PenDown()
	initialDowns, initialUps := spy.penDownCalls, spy.penUpCalls
	start := mathf.NewVec2(10, 20)
	dispatchSpriteDragFrame(co, game, start, &eventLeftButtonDown{Pos: start})
	end := mathf.NewVec2(50, 60)
	dispatchSpriteDragFrame(co, game, end, &eventLeftButtonUp{Pos: end})
	sprite.PenDown()
	if spy.restoreCalls != 1 || spy.lastRestore != end || !spy.lastRestoreDown {
		t.Fatalf("reset calls/position = %d/%v, want 1/%v", spy.restoreCalls, spy.lastRestore, end)
	}
	if spy.penDownCalls != initialDowns+1 || spy.penUpCalls != initialUps || !spy.down {
		t.Fatal("explicit PenDown after drag must draw one point without an implicit PenUp")
	}
	if sprite.pen().positionReset {
		t.Fatal("explicit PenDown retained the pending reset")
	}
	resets := spy.restoreCalls
	sprite.SetXYpos(70, 80)
	if spy.restoreCalls != resets || spy.penDownCalls != initialDowns+1 {
		t.Fatal("movement repeated a completed pen reset or PenDown")
	}
}

func TestSpriteDragLazyClonePenRestoresDropOriginWithoutDrawingPoint(t *testing.T) {
	spy := &dragPenSpy{spyPenMgr: setupSpyPenMgr(t)}
	pkgengine.PenMgr = spy
	co, game, clone := setupSpriteDragTest(t)
	original := newPenTestSprite()
	original.PenDown()
	clone.components.pen = original.pen().cloneFor(clone)
	clone.spriteState.cloned = true
	if !clone.pen().isPenDown || clone.pen().penObj != nil {
		t.Fatal("clone must inherit pen-down state without allocating a drawing resource")
	}
	initialDowns, initialCreates, initialMoves := spy.penDownCalls, spy.createCalls, len(spy.moves)
	start := mathf.NewVec2(10, 20)
	dispatchSpriteDragFrame(co, game, start, &eventLeftButtonDown{Pos: start})
	end := mathf.NewVec2(50, 60)
	dispatchSpriteDragFrame(co, game, end, &eventLeftButtonUp{Pos: end})
	if spy.createCalls != initialCreates || len(spy.moves) != initialMoves || spy.penDownCalls != initialDowns {
		t.Fatal("dragging a lazy clone allocated or drew its pen")
	}
	clone.SetXYpos(70, 80)
	if spy.createCalls != initialCreates+1 || spy.penDownCalls != initialDowns || spy.penUpCalls != 0 {
		t.Fatal("first clone movement must allocate its pen without an implicit PenDown or PenUp")
	}
	if spy.restoreCalls != 1 || spy.lastRestore != end || !spy.lastRestoreDown {
		t.Fatal("first clone movement did not restore its inherited pen-down state at the drop position")
	}
	wantMoves := []dragPenMove{
		{position: end, reset: true},
		{position: mathf.NewVec2(70, 80), drawing: true},
	}
	if got := spy.moves[initialMoves:]; !reflect.DeepEqual(got, wantMoves) {
		t.Fatalf("clone pen movements = %v, want %v", got, wantMoves)
	}
	if clone.pen().positionReset {
		t.Fatal("first clone movement retained its pending position reset")
	}
}

func TestSpriteDragPenUpBeforeResumeKeepsDrawingStopped(t *testing.T) {
	spy := &dragPenSpy{spyPenMgr: setupSpyPenMgr(t)}
	pkgengine.PenMgr = spy
	co, game, sprite := setupSpriteDragTest(t)
	sprite.PenDown()
	initialDowns, initialUps := spy.penDownCalls, spy.penUpCalls
	initialMoves := spy.moveCalls
	start := mathf.NewVec2(10, 20)
	dispatchSpriteDragFrame(co, game, start, &eventLeftButtonDown{Pos: start})
	end := mathf.NewVec2(50, 60)
	dispatchSpriteDragFrame(co, game, end, &eventLeftButtonUp{Pos: end})
	sprite.PenUp()
	sprite.SetXYpos(70, 80)
	if sprite.pen().positionReset || spy.down || spy.restoreCalls != 0 || spy.moveCalls != initialMoves {
		t.Fatal("PenUp followed by movement resumed a pending drag stroke")
	}
	if spy.penDownCalls != initialDowns || spy.penUpCalls != initialUps+1 {
		t.Fatal("PenUp introduced an unexpected pen transition")
	}
	sprite.PenDown()
	if spy.penDownCalls != initialDowns+1 || spy.restoreCalls != 0 || spy.lastMove != mathf.NewVec2(70, 80) {
		t.Fatal("a later PenDown did not start at the sprite's current position")
	}
}

type dragPenMove struct {
	position mathf.Vec2
	drawing  bool
	reset    bool
}

type dragPenSpy struct {
	*spyPenMgr
	moves []dragPenMove
	down  bool
}

func (s *dragPenSpy) MovePenTo(obj pkgengine.Object, position mathf.Vec2) {
	s.moves = append(s.moves, dragPenMove{position: position, drawing: s.down})
	s.spyPenMgr.MovePenTo(obj, position)
}

func (s *dragPenSpy) RestorePenState(obj pkgengine.Object, position mathf.Vec2, isDown bool) {
	s.moves = append(s.moves, dragPenMove{position: position, reset: true})
	s.down = isDown
	s.spyPenMgr.RestorePenState(obj, position, isDown)
}

func (s *dragPenSpy) PenDown(obj pkgengine.Object, moveByMouse bool) {
	s.down = true
	s.spyPenMgr.PenDown(obj, moveByMouse)
}

func (s *dragPenSpy) PenUp(obj pkgengine.Object) {
	s.down = false
	s.spyPenMgr.PenUp(obj)
}

func setupSpriteDragTest(t *testing.T) (*coroutine.Coroutines, *Game, *SpriteImpl) {
	t.Helper()
	co, game := setupInputFrameGame(t)
	game.initShapeMgr()
	mgr := setupClickThroughSpriteMgr(t, map[pkgengine.Object]bool{1: true, 2: true})
	pkgengine.SpriteMgr = &dragSpriteMgr{clickThroughSpriteMgr: mgr}
	sprite := newDragTestSprite(game, "dragged", 1)
	sprite.SetDragMode(Draggable)
	game.shapeMgr.add(sprite)
	return co, game, sprite
}

type dragSpriteMgr struct{ *clickThroughSpriteMgr }

func (*dragSpriteMgr) SetTransform(pkgengine.Object, mathf.Vec2, float64, mathf.Vec2, bool, mathf.Vec2) {
}

func newDragTestSprite(game *Game, name string, id pkgengine.Object) *SpriteImpl {
	sprite := newClickTestSprite(game, name, id, false)
	sprite.components.initComponents(sprite, &coreproject.SpriteConfig{X: 10, Y: 20, RotationStyle: "normal"})
	sprite.costumes = []*costume{newCostumeWithSize(10, 10)}
	sprite.runtimeState.scale = 1
	sprite.runtimeState.syncSprite.OnAnimationLoopedEvent = &pkgengine.Event0{}
	sprite.runtimeState.syncSprite.OnAnimationFinishedEvent = &pkgengine.Event0{}
	return sprite
}

func dispatchSpriteDragFrame(co *coroutine.Coroutines, game *Game, point mathf.Vec2, events ...event) {
	game.inputMgr.mousePos = point
	game.inputMgr.frame.pending = &inputFrame{events: events}
	game.dispatchFrameEvents()
	co.Update()
}

func assertSpriteDragPosition(t *testing.T, sprite *SpriteImpl, x, y float64) {
	t.Helper()
	if gotX, gotY := sprite.getXY(); gotX != x || gotY != y {
		t.Fatalf("sprite position = (%v, %v), want (%v, %v)", gotX, gotY, x, y)
	}
}
