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
	"fmt"
	"math"
	"math/rand"

	"github.com/goplus/spbase/mathf"
	coreevent "github.com/goplus/spx/v2/internal/core/event"
	coreproject "github.com/goplus/spx/v2/internal/core/project"
	"github.com/goplus/spx/v2/internal/engine"
	spxlog "github.com/goplus/spx/v2/internal/log"
	"github.com/goplus/spx/v2/internal/timer"
	"github.com/goplus/spx/v2/internal/ui"
)

func (p *Game) BackdropName() string {
	return p.getCostumeName()
}

func (p *Game) BackdropIndex() int {
	return p.getCostumeIndex()
}

// SetBackdrop func:
//
//	SetBackdrop(backdrop) or
//	SetBackdrop(index) or
//	SetBackdrop(spx.Next)
//	SetBackdrop(spx.Prev)
func (p *Game) setBackdrop(backdrop any, wait bool) {
	if p.goSetCostume(backdrop) {
		p.setupBackdrop()
		p.doWindowSize()
		p.doWhenBackdropChanged(p.getCostumeName(), wait)
	}
}

func (p *Game) SetBackdrop__0(backdrop BackdropName) {
	p.setBackdrop(backdrop, false)
}

func (p *Game) SetBackdrop__1(index float64) {
	p.setBackdrop(index, false)
}

func (p *Game) SetBackdrop__2(index int) {
	p.setBackdrop(index, false)
}

func (p *Game) SetBackdrop__3(action switchAction) {
	p.setBackdrop(action, false)
}

func (p *Game) SetBackdropAndWait__0(backdrop BackdropName) {
	p.setBackdrop(backdrop, true)
}

func (p *Game) SetBackdropAndWait__1(index float64) {
	p.setBackdrop(index, true)
}

func (p *Game) SetBackdropAndWait__2(index int) {
	p.setBackdrop(index, true)
}

func (p *Game) SetBackdropAndWait__3(action switchAction) {
	p.setBackdrop(action, true)
}

func (p *Game) setupBackdrop() {
	imgW, imgH := p.getCostumeSize()
	layout := coreproject.ResolveBackdropLayout(
		imgW,
		imgH,
		float64(p.displayState.WorldWidth),
		float64(p.displayState.WorldHeight),
		p.displayState.MapMode,
	)
	if layout.RepeatScale != nil {
		p.setMaterialParamsVec4("repeat_scale", *layout.RepeatScale, false)
	}
	p.runtimeState.Scale = 1
	p.baseObj.scheduleCostumeUpdate()
	p.engine().SpriteMgr.SetScale(p.runtimeState.SyncSprite.GetId(), mathf.NewVec2(layout.ScaleX, layout.ScaleY))
}

// SetWindowSize sets the window size to the specified width and height.
func (p *Game) SetWindowSize(width int64, height int64) {
	p.engine().PlatformMgr.SetWindowSize(width, height, false)
}

// EraseAll erases all pen drawings.
func (p *Game) EraseAll() {
	p.engine().PenMgr.DestroyAllPens()
}

func (p *Game) getWindowSize() mathf.Vec2 {
	x, y := p.windowSize()
	return mathf.NewVec2(float64(x), float64(y))
}

func (p *Game) windowSize() (int, int) {
	if p.displayState.WindowWidth == 0 {
		p.doWindowSize()
	}
	return p.displayState.WindowWidth, p.displayState.WindowHeight
}

func (p *Game) doWindowSize() {
	if p.displayState.WindowWidth == 0 {
		c := p.costumes[p.costumeIndex]
		p.displayState.WindowWidth, p.displayState.WindowHeight = c.getSize()
	}
}

func (p *Game) worldSize() (int, int) {
	if p.displayState.WorldWidth == 0 {
		p.doWorldSize()
	}
	return p.displayState.WorldWidth, p.displayState.WorldHeight
}

func (p *Game) doWorldSize() {
	if p.displayState.WorldWidth == 0 {
		c := p.costumes[p.costumeIndex]
		p.displayState.WorldWidth, p.displayState.WorldHeight = c.getSize()
	}
}

func (p *Game) KeyPressed(key Key) bool {
	return p.engine().InputMgr.GetKey(int64(key))
}

func (p *Game) MouseX() float64 {
	return p.inputMgr.currentMousePos().X
}

func (p *Game) MouseY() float64 {
	return p.inputMgr.currentMousePos().Y
}

func (p *Game) MousePressed() bool {
	return p.engine().InputMgr.MousePressed()
}

func (p *Game) getMousePos() (x, y float64) {
	return p.MouseX(), p.MouseY()
}

func (p *Game) Username() string {
	panic("todo")
}

func (p *Game) WaitNextFrame() float64 {
	return engine.WaitNextFrame()
}

func (p *Game) Wait(secs float64) {
	engine.Wait(secs)
}

func (p *Game) Timer() float64 {
	return timer.Timer()
}

func (p *Game) ResetTimer() {
	timer.ResetTimer()
}

func (p *Game) Ask(msg any) {
	msgStr, ok := msg.(string)
	if !ok {
		msgStr = fmt.Sprint(msg)
	}
	if msgStr == "" {
		spxlog.Warn("ask: msg should not be empty")
		return
	}
	p.ask(false, msgStr, func(answer string) {})
}

func (p *Game) Answer() string {
	return p.dialogState.AnswerVal
}

func (p *Game) ask(isSprite bool, question string, callback func(string)) {
	if p.dialogState.AskPanel == nil {
		p.dialogState.AskPanel = ui.NewUiAsk()
		p.addShape(p.dialogState.AskPanel)
	}
	hasAnswer := false
	p.dialogState.AskPanel.Show(isSprite, question, func(msg string) {
		p.dialogState.AnswerVal = msg
		callback(msg)
		hasAnswer = true
	})
	for {
		if hasAnswer {
			break
		}
		p.dialogState.AskPanel.Update()
		engine.WaitNextFrame()
	}
}

type EffectKind int

const (
	ColorEffect EffectKind = iota
	FishEyeEffect
	WhirlEffect
	PixelateEffect
	MosaicEffect
	BrightnessEffect
	GhostEffect

	enumNumOfEffect // max index of enum
)

var greffNames = []string{
	ColorEffect:      "color_amount",
	FishEyeEffect:    "fisheye_amount",
	WhirlEffect:      "whirl_amount",
	MosaicEffect:     "uv_amount",
	PixelateEffect:   "pixleate_amount",
	BrightnessEffect: "brightness_amount",
	GhostEffect:      "alpha_amount",
}

func (kind EffectKind) String() string {
	return greffNames[kind]
}

func (p *Game) SetGraphicEffect(kind EffectKind, val float64) {
	p.baseObj.setGraphicEffect(kind, val)
}

func (p *Game) ChangeGraphicEffect(kind EffectKind, delta float64) {
	p.baseObj.changeGraphicEffect(kind, delta)
}

func (p *Game) ClearGraphicEffects() {
	p.baseObj.clearGraphicEffects()
}

// MsgName represents a message name for broadcasting events.
type MsgName = coreevent.MsgName

func (p *Game) doBroadcast(msg MsgName, data any, wait bool) {
	if isDebugInstrEnabled() {
		spxlog.Debug("Broadcast: msg=%s, wait=%v", msg, wait)
	}
	p.scriptEvents.doWhenIReceive(msg, data, wait)
}

func (p *Game) Broadcast__0(msg MsgName) {
	p.doBroadcast(msg, nil, false)
}

func (p *Game) Broadcast__1(msg MsgName, data any) {
	p.doBroadcast(msg, data, false)
}

func (p *Game) BroadcastAndWait__0(msg MsgName) {
	p.doBroadcast(msg, nil, true)
}

func (p *Game) BroadcastAndWait__1(msg MsgName, data any) {
	p.doBroadcast(msg, data, true)
}

type PropertyName = string

func (p *Game) setStageMonitor(target string, val PropertyName, visible bool) bool {
	for _, item := range p.spriteMgr.items {
		if sp, ok := item.(*Monitor); ok && sp.val == val && sp.target == target {
			sp.setVisible(visible)
			return true
		}
	}
	return false
}

func (p *Game) HideVar(name PropertyName) {
	p.setStageMonitor("", name, false)
}

func (p *Game) ShowVar(name PropertyName) {
	p.setStageMonitor("", name, true)
}

// Rand__0 returns a random integer between from and to (inclusive).
func Rand__0(from, to int) float64 {
	if to < from {
		to = from
	}
	return float64(from + rand.Intn(to-from+1))
}

// Rand__1 returns a random float64 between from and to.
func Rand__1(from, to float64) float64 {
	if to < from {
		to = from
	}
	return rand.Float64()*(to-from) + from
}

// Iround returns an integer value, while math.Round returns a float value.
func Iround(v float64) int {
	if v >= 0 {
		return int(v + 0.5)
	}
	return int(v - 0.5)
}

// Exit__0 exits the program with the specified exit code.
func Exit__0(code int) {
	engine.RequestExit(int64(code))
}

// Exit__1 exits the program with exit code 0.
func Exit__1() {
	engine.RequestExit(0)
}

func (p *Game) touchingPoint(dst *SpriteImpl, x, y float64) bool {
	return dst.touchPoint(x, y)
}

func (p *Game) touchingSpriteBy(dst *SpriteImpl, name string) *SpriteImpl {
	if dst == nil {
		return nil
	}
	// Use optimized spatial partitioning version.
	return p.findTouchingSpriteOptimized(dst, name)
}

func (p *Game) objectPos(obj any) (float64, float64) {
	switch v := obj.(type) {
	case SpriteName:
		if sp := p.spriteMgr.findSprite(v); sp != nil {
			return sp.getXY()
		}
		engine.Panic("objectPos: sprite not found - " + v)
	case specialObj:
		if v == Mouse {
			return p.getMousePos()
		}
	case Pos:
		if v == Random {
			worldW, worldH := p.worldSize()
			mx, my := rand.Intn(worldW), rand.Intn(worldH)
			return float64(mx - (worldW >> 1)), float64((worldH >> 1) - my)
		}
	case Sprite:
		return spriteOf(v).getXY()
	}
	engine.Panic("objectPos: unexpected input")
	return 0, 0
}

func (p *Game) addShape(child Shape) {
	p.spriteMgr.addShape(child)
}

func (p *Game) addClonedShape(src, clone Shape) {
	p.spriteMgr.addClonedShape(src, clone)
}

func (p *Game) removeShape(child Shape) {
	p.spriteMgr.removeShape(child)
}

func (p *Game) activateShape(child Shape) {
	p.spriteMgr.activateShape(child)
}

func (p *Game) findSprite(name SpriteName) *SpriteImpl {
	return p.spriteMgr.findSprite(name)
}

func (p *Game) getAllShapes() []Shape {
	return p.spriteMgr.all()
}

func (p *Game) getTempShapes() []Shape {
	return p.spriteMgr.getTempShapes()
}

func (p *Game) gotoFront(spr *SpriteImpl) {
	p.goBackLayers(spr, math.MinInt32)
}

func (p *Game) gotoBack(spr *SpriteImpl) {
	p.goBackLayers(spr, math.MaxInt32)
}

func (p *Game) goBackLayers(spr *SpriteImpl, n int) {
	p.spriteMgr.goBackLayers(spr, n)
}

// Widget Management
type WidgetName = string

type Widget interface {
	GetName() WidgetName
	Visible() bool
	Show()
	Hide()

	Xpos() float64
	Ypos() float64
	SetXpos(x float64)
	SetYpos(y float64)
	SetXYpos(x float64, y float64)
	ChangeXpos(dx float64)
	ChangeYpos(dy float64)
	ChangeXYpos(dx float64, dy float64)

	Size() float64
	SetSize(size float64)
	ChangeSize(delta float64)
}

type ShapeGetter interface {
	getAllShapes() []Shape
}

// GetWidget returns the widget instance with given name. It panics if not found.
// Instead of being used directly, it is meant to be called by `XGot_Game_XGox_GetWidget` only.
// We extract `GetWidget` to keep `XGot_Game_XGox_GetWidget` simple, which simplifies work in ispx,
// see details in https://github.com/goplus/builder/issues/765#issuecomment-2313915805.
func GetWidget(sg ShapeGetter, name WidgetName) Widget {
	items := sg.getAllShapes()
	for _, item := range items {
		widget, ok := item.(Widget)
		if !ok {
			continue
		}
		if widget.GetName() == name {
			return widget
		}
	}
	panic("GetWidget: widget not found - " + name)
}

// GetWidget returns the widget instance (in given type) with given name. It panics if not found.
func XGot_Game_XGox_GetWidget[T any](sg ShapeGetter, name WidgetName) *T {
	widget, ok := GetWidget(sg, name).(any).(*T)
	if !ok {
		panic("GetWidget: type mismatch - " + name)
	}
	return widget
}

// Path Finding System
func (p *Game) applyPathFinderSettings(settings coreproject.SystemSettings) {
	p.pathfindingState.PathCellSizeX = settings.PathCellSizeX
	p.pathfindingState.PathCellSizeY = settings.PathCellSizeY
}

func (p *Game) SetupPathFinder__0() {
	p.setupPathFinder(true, false)
}

func (p *Game) SetupPathFinder__1(x_grid_size, y_grid_size, x_cell_size, y_cell_size float64, with_jump, with_debug bool) {
	p.engine().NavigationMgr.SetupPathFinderWithSize(mathf.NewVec2(x_grid_size, y_grid_size), mathf.NewVec2(x_cell_size, y_cell_size), with_jump, with_debug)
}

func (p *Game) setupPathFinder(with_jump, with_debug bool) {
	cellSize := mathf.NewVec2(float64(p.pathfindingState.PathCellSizeX), float64(p.pathfindingState.PathCellSizeY))
	gridSize := mathf.NewVec2(float64(p.displayState.WorldWidth), float64(p.displayState.WorldHeight)).Div(cellSize)
	p.engine().NavigationMgr.SetupPathFinderWithSize(gridSize, cellSize, with_jump, with_debug)
}

func (p *Game) FindPath__0(x_from, y_from, x_to, y_to float64) []float64 {
	return p.FindPath__2(x_from, y_from, x_to, y_to, false, true)
}

func (p *Game) FindPath__1(x_from, y_from, x_to, y_to float64, with_debug bool) []float64 {
	return p.FindPath__2(x_from, y_from, x_to, y_to, with_debug, true)
}

func (p *Game) FindPath__2(x_from, y_from, x_to, y_to float64, with_debug, with_jump bool) []float64 {
	p.lifecycleState.OncePathFinder.Do(func() {
		p.setupPathFinder(with_jump, with_debug)
	})

	arr := p.engine().NavigationMgr.FindPath(mathf.NewVec2(x_from, y_from), mathf.NewVec2(x_to, y_to), with_jump)
	result := arr.([]float32)
	return engine.F32Tof64(result)
}
