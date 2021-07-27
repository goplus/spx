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

type Sprite struct {
}

const (
	Mouse = iota // TODO: type?
	Edge
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
	panic("todo")
}

func (p *Sprite) CostumeIndex() int {
	panic("todo")
}

func (p *Sprite) SetCostume(costume interface{}) {
	panic("todo")
}

func (p *Sprite) NextCostume() {
	panic("todo")
}

// -----------------------------------------------------------------------------

func (p *Sprite) Say(sth string, secs ...float64) {
	panic("todo")
}

func (p *Sprite) Think(sth string, secs ...float64) {
	panic("todo")
}

// -----------------------------------------------------------------------------

// DistanceTo func:
//   DistanceTo(sprite)
//   DistanceTo(gox.Mouse)
//   DistanceTo(gox.Edge)
func (p *Sprite) DistanceTo(obj interface{}) float64 {
	panic("todo")
}

func (p *Sprite) Step(n float64) {
	panic("todo")
}

// Goto func:
//   Goto(sprite)
//   Goto(gox.Mouse)
func (p *Sprite) Goto(obj interface{}) float64 {
	panic("todo")
}

func (p *Sprite) Glide(x, y float64, secs float64) {
	panic("todo")
}

func (p *Sprite) GotoXY(x, y float64) {
	panic("todo")
}

func (p *Sprite) Xpos() float64 {
	panic("todo")
}

func (p *Sprite) SetXpos(x float64) {
	panic("todo")
}

func (p *Sprite) ChangeXpos(dx float64) {
	panic("todo")
}

func (p *Sprite) Ypos() float64 {
	panic("todo")
}

func (p *Sprite) SetYpos(y float64) {
	panic("todo")
}

func (p *Sprite) ChangeYpos(dy float64) {
	panic("todo")
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

type Color = float64

// Touching func:
//   Touching(sprite)
//   Touching(spx.Mouse)
//   Touching(spx.Edge)
func (p *Sprite) Touching(obj interface{}) bool {
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
