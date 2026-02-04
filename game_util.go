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
	"math/rand"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v2/internal/engine"
	"github.com/goplus/spx/v2/internal/enginewrap"
	spxlog "github.com/goplus/spx/v2/internal/log"
)

// -------------------------------------------------------------------------------------
// Window and World Size Utilities

func (p *Game) getWindowSize() mathf.Vec2 {
	x, y := p.windowSize_()
	return mathf.NewVec2(float64(x), float64(y))
}

func (p *Game) windowSize_() (int, int) {
	if p.windowWidth == 0 {
		p.doWindowSize()
	}
	return p.windowWidth, p.windowHeight
}

func (p *Game) doWindowSize() {
	if p.windowWidth == 0 {
		c := p.costumes[p.costumeIndex_]
		p.windowWidth, p.windowHeight = c.getSize()
	}
}

func (p *Game) worldSize_() (int, int) {
	if p.worldWidth == 0 {
		p.doWorldSize()
	}
	return p.worldWidth, p.worldHeight
}

func (p *Game) doWorldSize() {
	if p.worldWidth == 0 {
		c := p.costumes[p.costumeIndex_]
		p.worldWidth, p.worldHeight = c.getSize()
	}
}

func (p *Game) SetWindowSize(width int64, height int64) {
	platformMgr.SetWindowSize(width, height, false)
}

// -------------------------------------------------------------------------------------
// Touch and Collision Utilities

func (p *Game) touchingPoint(dst *SpriteImpl, x, y float64) bool {
	return dst.touchPoint(x, y)
}

func (p *Game) touchingSpriteBy(dst *SpriteImpl, name string) *SpriteImpl {
	if dst == nil {
		return nil
	}

	// Use optimized spatial partitioning version
	// This reduces expensive pixel-perfect collision checks
	return p.findTouchingSpriteOptimized(dst, name)
}

// -------------------------------------------------------------------------------------
// Object Position Utilities

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
			worldW, worldH := p.worldSize_()
			mx, my := rand.Intn(worldW), rand.Intn(worldH)
			return float64(mx - (worldW >> 1)), float64((worldH >> 1) - my)
		}
	case Sprite:
		return spriteOf(v).getXY()
	}
	engine.Panic("objectPos: unexpected input")
	return 0, 0
}

// -------------------------------------------------------------------------------------
// Pen Utilities

func (p *Game) EraseAll() {
	penMgr.DestroyAllPens()
}

// -------------------------------------------------------------------------------------
// Shape Management Utilities

func (p *Game) getItems() []Shape {
	return p.spriteMgr.all()
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

// -------------------------------------------------------------------------------------
// Random Number Utilities

func Rand__0(from, to int) float64 {
	if to < from {
		to = from
	}
	return float64(from + rand.Intn(to-from+1))
}

func Rand__1(from, to float64) float64 {
	if to < from {
		to = from
	}
	return rand.Float64()*(to-from) + from
}

// -------------------------------------------------------------------------------------
// Math Utilities

// Iround returns an integer value, while math.Round returns a float value.
func Iround(v float64) int {
	if v >= 0 {
		return int(v + 0.5)
	}
	return int(v - 0.5)
}

// -------------------------------------------------------------------------------------
// Color Utilities

type Color struct {
	r, g, b, a float64
}

func toMathfColor(c Color) mathf.Color {
	return mathf.Color{R: c.r, G: c.g, B: c.b, A: c.a}
}

func toSpxColor(c mathf.Color) Color {
	return Color{c.R, c.G, c.B, c.A}
}

// HSB creates a color from HSB values.
// h, s, b in range [0, 100], just like Scratch
func HSB(h, s, b float64) Color {
	color := mathf.NewColorHSV(h*3.6, s/100, b/100)
	color.A = 1
	return toSpxColor(color)
}

// HSBA creates a color from HSBA values.
// h, s, b, a in range [0, 100], just like Scratch
func HSBA(h, s, b, a float64) Color {
	color := HSB(h, s, b)
	color.a = a / 100
	return color
}

// -------------------------------------------------------------------------------------
// Type Conversion Utilities

func f64Tof32(slice []float64) []float32 {
	return enginewrap.F64Tof32(slice)
}

func f32Tof64(slice []float32) []float64 {
	return enginewrap.F32Tof64(slice)
}

// -----------------------------------------------------------------------------
// Configuration Parsing Utilities
// -----------------------------------------------------------------------------

// parseDefaultValue parses a pointer value with a default fallback.
func parseDefaultValue[T any](pval *T, defaultValue T) T {
	if pval == nil {
		return defaultValue
	}
	return *pval
}

// parseLayerMaskValue parses layer mask value
func parseLayerMaskValue(pval *int64) int64 {
	return parseDefaultValue(pval, 1)
}

// parseColliderShapeType parses collider shape type from string
func parseColliderShapeType(typeName string, defaultValue int64) int64 {
	switch typeName {
	case "none":
		return physicsColliderNone
	case "auto":
		return physicsColliderAuto
	case "circle":
		return physicsColliderCircle
	case "rect":
		return physicsColliderRect
	case "capsule":
		return physicsColliderCapsule
	case "polygon":
		return physicsColliderPolygon
	case "":
		return defaultValue
	default:
		spxlog.Warn("Invalid colliderShapeType value '%s', using default", typeName)
		return defaultValue
	}
}

// parsePixelCollisionPrecision parses the precision string and returns the corresponding enum value
func parsePixelCollisionPrecision(precision *string) pixelCollisionPrecision {
	if precision == nil {
		return pixelCollisionPrecisionLow // default
	}

	switch *precision {
	case "high":
		return pixelCollisionPrecisionHigh
	case "medium":
		return pixelCollisionPrecisionMedium
	case "low":
		return pixelCollisionPrecisionLow
	default:
		spxlog.Warn("Invalid pixelCollisionPrecision value '%s', using default 'low'", *precision)
		return pixelCollisionPrecisionLow
	}
}

// setDefaultIfZero sets the value to defaultValue if it's currently zero.
func setDefaultIfZero[T comparable](pval *T, defaultValue T) {
	var zero T
	if *pval == zero {
		*pval = defaultValue
	}
}

// toRotationStyle converts a string representation to a RotationStyle constant
func toRotationStyle(style string) RotationStyle {
	switch style {
	case "left-right":
		return LeftRight
	case "none":
		return None
	case "normal":
		return Normal
	default:
		spxlog.Warn("Unrecognized rotationStyle value '%s', using default 'Normal'.", style)
		return Normal
	}
}

// -------------------------------------------------------------------------------------
// Exit Functions

func Exit__0(code int) {
	engine.RequestExit(int64(code))
}

func Exit__1() {
	engine.RequestExit(0)
}

// -------------------------------------------------------------------------------------
// Panic Functions

func doPanic(args ...any) {
	engine.Panic(args...)
}

func doPanicf(format string, args ...any) {
	engine.Panicf(format, args...)
}
