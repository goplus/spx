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

// ======================== Pen Component ========================
// This file contains pen-related functionality for sprites,
// including pen drawing, color control, and size management.
// All methods proxy to the pen component implementation.

// -----------------------------------------------------------------------------
// Pen Color Parameter Types
// -----------------------------------------------------------------------------

type PenColorParam int

const (
	PenHue PenColorParam = iota
	PenSaturation
	PenBrightness
	PenTransparency
)

// -----------------------------------------------------------------------------
// Pen Control
// -----------------------------------------------------------------------------

func (p *SpriteImpl) PenUp() {
	p.components.Pen().PenUp()
}

func (p *SpriteImpl) PenDown() {
	p.components.Pen().PenDown()
}

func (p *SpriteImpl) Stamp() {
	p.components.Pen().Stamp()
}

// -----------------------------------------------------------------------------
// Pen Color Control
// -----------------------------------------------------------------------------

func (p *SpriteImpl) SetPenColor__0(color Color) {
	p.components.Pen().SetPenColor(color)
}

func (p *SpriteImpl) SetPenColor__1(kind PenColorParam, value float64) {
	p.components.Pen().SetPenColorParam(kind, value)
}

func (p *SpriteImpl) ChangePenColor(kind PenColorParam, delta float64) {
	p.components.Pen().ChangePenColor(kind, delta)
}

// -----------------------------------------------------------------------------
// Pen Size Control
// -----------------------------------------------------------------------------

func (p *SpriteImpl) SetPenSize(size float64) {
	p.components.Pen().SetPenSize(size)
}

func (p *SpriteImpl) ChangePenSize(delta float64) {
	p.components.Pen().ChangePenSize(delta)
}

// -----------------------------------------------------------------------------
// Internal Pen Management (used by other components)
// -----------------------------------------------------------------------------

func (p *SpriteImpl) movePen(x, y float64) {
	p.components.Pen().movePen(x, y)
}

func (p *SpriteImpl) isPenDown() bool {
	return p.components.Pen().isPenDown()
}
