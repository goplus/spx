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
	"math/rand"
	"reflect"
	"time"

	"github.com/goplus/spbase/mathf"
	coreproject "github.com/goplus/spx/v2/internal/core/project"
	"github.com/goplus/spx/v2/internal/engine"
	spxlog "github.com/goplus/spx/v2/internal/log"
	itime "github.com/goplus/spx/v2/internal/time"
	"github.com/goplus/spx/v2/internal/ui"
)

// -----------------------------------------------------------------------------
// Stage Display
// -----------------------------------------------------------------------------
func (p *Game) BackdropName() string {
	return p.getCostumeName()
}

func (p *Game) BackdropIndex() int {
	return p.getCostumeIndex()
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

func (p *Game) SetWindowSize(width int64, height int64) {
	p.engine().PlatformMgr.SetWindowSize(width, height, false)
}

func (p *Game) EraseAll() {
	p.engine().PenMgr.DestroyAllPens()
}

// -----------------------------------------------------------------------------
// Visual Effects
// -----------------------------------------------------------------------------
type EffectKind int

const (
	ColorEffect EffectKind = iota
	FishEyeEffect
	WhirlEffect
	PixelateEffect
	MosaicEffect
	BrightnessEffect
	GhostEffect

	enumNumOfEffect
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

// -----------------------------------------------------------------------------
// Input and Timing
// -----------------------------------------------------------------------------
func (p *Game) KeyPressed(key Key) bool {
	return p.engine().InputMgr.GetKey(int64(key))
}

func (p *Game) MouseX() float64 {
	if x, _, ok := engine.GetPollingMousePos(); ok {
		return x
	}
	return p.liveMousePos().X
}

func (p *Game) MouseY() float64 {
	if _, y, ok := engine.GetPollingMousePos(); ok {
		return y
	}
	return p.liveMousePos().Y
}

func (p *Game) MousePressed() bool {
	return engine.AnyMouseButtonPressedForPolling()
}

func (p *Game) getMousePos() (x, y float64) {
	return p.MouseX(), p.MouseY()
}

func (p *Game) liveMousePos() mathf.Vec2 {
	curMousePos := p.engine().InputMgr.GetGlobalMousePos()
	return mathf.Vec2{X: float64(curMousePos.X), Y: float64(curMousePos.Y)}
}

func (p *Game) Username() string {
	return ""
}

func (p *Game) WaitNextFrame() Seconds {
	return engine.WaitNextFrame()
}

func (p *Game) Wait(secs Seconds) {
	engine.Wait(secs)
}

func (p *Game) Timer() Seconds {
	return itime.Timer()
}

func (p *Game) ResetTimer() {
	itime.ResetTimer()
}

func (p *Game) Now() time.Time {
	return time.Now()
}

func (p *Game) Dayssince2000() float64 {
	base := time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC)
	return time.Since(base).Hours() / 24
}

// -----------------------------------------------------------------------------
// Dialog
// -----------------------------------------------------------------------------
func (p *Game) Ask(msg any) {
	msgStr, ok := msg.(string)
	if !ok {
		msgStr = fmt.Sprint(msg)
	}
	if msgStr == "" {
		spxlog.Warn("Ask: message should not be empty")
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

// -----------------------------------------------------------------------------
// Property
// -----------------------------------------------------------------------------
type PropertyName = string

func (p *Game) propertyRootValue() reflect.Value {
	if p.gamer != nil {
		return reflect.ValueOf(p.gamer).Elem()
	}
	return reflect.ValueOf(p).Elem()
}

func (p *Game) resolveTargetProperty(target string, name PropertyName) (Value, bool) {
	if name == "" {
		return Value{}, false
	}

	resolvedTarget, from := p.resolvePropertyTarget(target)
	if from < 0 {
		return Value{}, false
	}

	eval := coreproject.ResolveMemberValueEval(resolvedTarget, name, from)
	if eval == nil {
		return Value{}, false
	}
	return Value{data: eval()}, true
}

func (p *Game) resolvePropertyTarget(target string) (reflect.Value, int) {
	root := p.propertyRootValue()
	if target == "" {
		return root, 1 // spx.Game
	}

	val := coreproject.FindFieldPtr(root, target, 0)
	if val == nil {
		return reflect.Value{}, -1
	}

	v := reflect.ValueOf(val).Elem()
	if _, ok := val.(Sprite); ok {
		return v, 2 // (spx.Sprite, *Game)
	}
	return v, 0 // normal target field
}

func (p *Game) GetTargetProperty(target string, name PropertyName) Value {
	if val, ok := p.resolveTargetProperty(target, name); ok {
		return val
	}
	return Value{}
}

// -----------------------------------------------------------------------------
// Monitor
// -----------------------------------------------------------------------------

func (p *Game) setStageMonitor(target string, val PropertyName, visible bool) bool {
	for _, item := range p.shapeMgr.items {
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

// -----------------------------------------------------------------------------
// Position
// -----------------------------------------------------------------------------
func (p *Game) touchingPoint(dst *SpriteImpl, x, y float64) bool {
	return dst.touchPoint(x, y)
}

func (p *Game) touchingSpriteBy(dst *SpriteImpl, name string) *SpriteImpl {
	if dst == nil {
		return nil
	}
	return p.findTouchingSpriteOptimized(dst, name)
}

func (p *Game) objectPos(obj Target) (float64, float64) {
	switch v := obj.(type) {
	case SpriteName:
		if sp := p.shapeMgr.findSprite(v); sp != nil {
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

// -----------------------------------------------------------------------------
// Path Finding
// -----------------------------------------------------------------------------
func (p *Game) applyPathFinderSettings(settings coreproject.SystemSettings) {
	p.pathfindingState.PathCellSizeX = settings.PathCellSizeX
	p.pathfindingState.PathCellSizeY = settings.PathCellSizeY
}

func (p *Game) SetupPathFinder__0() {
	p.setupPathFinder(true, false)
}

func (p *Game) SetupPathFinder__1(xGridSize, yGridSize, xCellSize, yCellSize float64, withJump, withDebug bool) {
	p.engine().NavigationMgr.SetupPathFinderWithSize(
		mathf.NewVec2(xGridSize, yGridSize),
		mathf.NewVec2(xCellSize, yCellSize),
		withJump,
		withDebug,
	)
}

func (p *Game) setupPathFinder(withJump, withDebug bool) {
	cellSize := mathf.NewVec2(float64(p.pathfindingState.PathCellSizeX), float64(p.pathfindingState.PathCellSizeY))
	gridSize := mathf.NewVec2(float64(p.displayState.WorldWidth), float64(p.displayState.WorldHeight)).Div(cellSize)
	p.engine().NavigationMgr.SetupPathFinderWithSize(gridSize, cellSize, withJump, withDebug)
}

func (p *Game) FindPath__0(xFrom, yFrom, xTo, yTo float64) []float64 {
	return p.FindPath__2(xFrom, yFrom, xTo, yTo, false, true)
}

func (p *Game) FindPath__1(xFrom, yFrom, xTo, yTo float64, withDebug bool) []float64 {
	return p.FindPath__2(xFrom, yFrom, xTo, yTo, withDebug, true)
}

func (p *Game) FindPath__2(xFrom, yFrom, xTo, yTo float64, withDebug, withJump bool) []float64 {
	p.lifecycleState.OncePathFinder.Do(func() {
		p.setupPathFinder(withJump, withDebug)
	})

	arr := p.engine().NavigationMgr.FindPath(mathf.NewVec2(xFrom, yFrom), mathf.NewVec2(xTo, yTo), withJump)
	result := arr.([]float32)
	return engine.F32Tof64(result)
}
