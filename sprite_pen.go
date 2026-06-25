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

// -----------------------------------------------------------------------------
// Color
// -----------------------------------------------------------------------------
type PenColorParam int

const (
	PenHue PenColorParam = iota
	PenSaturation
	PenBrightness
	PenTransparency
	PenNone
)

// -----------------------------------------------------------------------------
// Pen
// -----------------------------------------------------------------------------
func (p *SpriteImpl) PenUp() {
	p.pen().PenUp()
}

func (p *SpriteImpl) PenDown() {
	p.pen().PenDown()
}

func (p *SpriteImpl) Stamp() {
	p.pen().Stamp()
}

// -----------------------------------------------------------------------------
// Color Control
// -----------------------------------------------------------------------------
func (p *SpriteImpl) SetPenColor__0(color Color) {
	p.pen().SetPenColor(color)
}

func (p *SpriteImpl) SetPenColor__1(kind PenColorParam, value float64) {
	p.pen().SetPenColorParam(kind, value)
}

func (p *SpriteImpl) ChangePenColor(kind PenColorParam, delta float64) {
	p.pen().ChangePenColor(kind, delta)
}

func (p *SpriteImpl) SetPenHue(value float64) {
	p.pen().setPenHue(value)
}

func (p *SpriteImpl) ChangePenHue(delta float64) {
	p.pen().changePenHue(delta)
}

func (p *SpriteImpl) SetPenShade(value float64) {
	p.pen().SetPenShade(value)
}

func (p *SpriteImpl) ChangePenShade(delta float64) {
	p.pen().ChangePenShade(delta)
}

// -----------------------------------------------------------------------------
// Size
// -----------------------------------------------------------------------------
func (p *SpriteImpl) SetPenSize(size float64) {
	p.pen().SetPenSize(size)
}

func (p *SpriteImpl) ChangePenSize(delta float64) {
	p.pen().ChangePenSize(delta)
}

// -----------------------------------------------------------------------------
// Internals
// -----------------------------------------------------------------------------
func (p *SpriteImpl) movePen(x, y float64) {
	p.pen().movePen(x, y)
}
