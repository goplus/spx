/*
 Copyright 2021 The GoPlus Authors (goplus.org)

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package spx

import (
	"image/color"
	"math"
)

type Sprite struct {
	Base
	g *Game

	x, y          float64
	scale         float64
	direction     float64
	rotationStyle RotationStyle
	say           *sayOrThinker

	penColor color.RGBA
	penShade float64
	penHue   float64
	penWidth float64

	visible     bool
	isDraggable bool
	isCloned    bool
	isPenDown   bool
}

type Object interface {
	objMark()
}

type specialObj int

func (p specialObj) objMark() {}
func (p *Sprite) objMark()    {}

var (
	Mouse Object = specialObj(1)
	Edge  Object = specialObj(2)
)

const (
	Right float64 = 90
	Left  float64 = -90
	Up    float64 = 0
	Down  float64 = 180
)

// -----------------------------------------------------------------------------

func (p *Sprite) Clone(data interface{}) *Sprite {
	panic("todo")
}

func (p *Sprite) Destroy() { // delete this clone
	panic("todo")
}

// -----------------------------------------------------------------------------

func (p *Sprite) Hide() {
	panic("todo")
}

func (p *Sprite) Show() {
	panic("todo")
}

// -----------------------------------------------------------------------------

func (p *Sprite) CostumeName() string {
	return p.costumeName()
}

func (p *Sprite) CostumeIndex() int {
	return p.costumeIndex()
}

func (p *Sprite) SetCostume(costume interface{}) {
	p.setCostume(costume)
}

func (p *Sprite) NextCostume() {
	p.nextCostume()
}

// -----------------------------------------------------------------------------

func (p *Sprite) Say(sth string, secs ...float64) {
	panic("todo")
}

func (p *Sprite) Think(sth string, secs ...float64) {
	panic("todo")
}

// -----------------------------------------------------------------------------

func (p *Sprite) getXY() (x, y float64) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.x, p.y
}

// DistanceTo func:
//   DistanceTo(sprite)
//   DistanceTo(gox.Mouse)
//   DistanceTo(gox.Edge)
func (p *Sprite) DistanceTo(obj Object) float64 {
	p.mutex.Lock()
	x, y := p.x, p.y
	p.mutex.Unlock()

	_, _ = x, y
	panic("todo")
	// return p.g.distanceTo(x, y, name)
}

func (p *Sprite) doMoveTo(x, y float64) {
	if p.isPenDown {
		p.g.movePen(p, x, y)
	}
	p.x, p.y = x, y
}

func (p *Sprite) Step(step float64) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	sin, cos := math.Sincos(toRadian(p.direction))
	p.doMoveTo(p.x+step*sin, p.y+step*cos)
}

// Goto func:
//   Goto(sprite)
//   Goto(gox.Mouse)
func (p *Sprite) Goto(obj Object) {
	panic("todo")
	// x, y := p.g.mouseOrSpritePos(where)
	// p.setXY(x, y)
}

const (
	glideTick = 1e8
)

func (p *Sprite) Glide(x, y float64, secs float64) {
	inDur := int64(secs * 1e9)
	n := int(inDur / glideTick)
	if n > 0 {
		x0, y0 := p.getXY()
		dx := (x - x0) / float64(n)
		dy := (y - y0) / float64(n)
		for i := 1; i < n; i++ {
			p.g.sleep(glideTick)
			inDur -= glideTick
			x0 += dx
			y0 += dy
			p.SetXY(x0, y0)
		}
	}
	p.g.sleep(inDur)
	p.SetXY(x, y)
}

func (p *Sprite) SetXY(x, y float64) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.doMoveTo(x, y)
}

func (p *Sprite) Xpos() float64 {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	return p.x
}

func (p *Sprite) SetXpos(x float64) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.doMoveTo(x, p.y)
}

func (p *Sprite) ChangeXpos(dx float64) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.doMoveTo(p.x+dx, p.y)
}

func (p *Sprite) Ypos() float64 {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	return p.y
}

func (p *Sprite) SetYpos(y float64) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.doMoveTo(p.x, y)
}

func (p *Sprite) ChangeYpos(dy float64) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.doMoveTo(p.x, p.y+dy)
}

// -----------------------------------------------------------------------------

type RotationStyle int

func (p *Sprite) SetRotationStyle(style RotationStyle) {
	panic("todo")
}

func (p *Sprite) BounceOffEdge() {
	panic("todo")
}

func (p *Sprite) Heading() float64 {
	panic("todo")
}

// Turn func:
//   Turn(degree)
//   Turn(gox.Left)
//   Turn(gox.Right)
//   Turn(gox.Up)
//   Turn(gox.Down)
func (p *Sprite) Turn(heading float64) {
	panic("todo")
}

func (p *Sprite) TurnLeft() {
	panic("todo")
}

func (p *Sprite) TurnRight() {
	panic("todo")
}

// TurnTo func:
//   TurnTo(sprite)
//   TurnTo(gox.Mouse)
func (p *Sprite) TurnTo(obj interface{}) {
	panic("todo")
}

// -----------------------------------------------------------------------------

func (p *Sprite) Size() float64 {
	panic("todo")
}

func (p *Sprite) SetSize(size float64) {
	panic("todo")
}

func (p *Sprite) ChangeSize(delta float64) {
	panic("todo")
}

// -----------------------------------------------------------------------------

type Color = color.RGBA

// Touching func:
//   Touching(sprite)
//   Touching(spx.Mouse)
//   Touching(spx.Edge)
func (p *Sprite) Touching(obj Object) bool {
	panic("todo")
}

func (p *Sprite) TouchingColor(color Color) bool {
	panic("todo")
}

// -----------------------------------------------------------------------------

func (p *Sprite) GoBackLayers(n int) {
	panic("todo")
}

func (p *Sprite) GotoFront(n int) {
	panic("todo")
}

// -----------------------------------------------------------------------------

func (p *Sprite) Clear() {
	panic("todo")
}

func (p *Sprite) Stamp() {
	panic("todo")
}

func (p *Sprite) PenUp() {
	panic("todo")
}

func (p *Sprite) PenDown() {
	panic("todo")
}

func (p *Sprite) SetPenColor(color Color) {
	panic("todo")
}

func (p *Sprite) ChangePenColor(delta float64) {
	panic("todo")
}

func (p *Sprite) SetPenShade(shade float64) {
	panic("todo")
}

func (p *Sprite) ChangePenShade(delta float64) {
	panic("todo")
}

func (p *Sprite) SetPenSize(shade float64) {
	panic("todo")
}

func (p *Sprite) ChangePenSize(delta float64) {
	panic("todo")
}

func (p *Sprite) SetPenHue(shade float64) {
	panic("todo")
}

func (p *Sprite) ChangePenHue(delta float64) {
	panic("todo")
}

// -----------------------------------------------------------------------------
