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
	"io"
)

type FileSystem interface {
	Open(file string) (io.ReadCloser, error)
	Close() error
}

type Game struct {
	baseObj
	fs     FileSystem
	turtle turtleCanvas

	width  int
	height int
}

type SwitchAction int

const (
	Prev SwitchAction = -1
	Next SwitchAction = 1
)

// -----------------------------------------------------------------------------

func (p *Game) sleep(tick int64) {
	panic("todo")
}

// -----------------------------------------------------------------------------

func (p *Game) getWidth() int {
	if p.width == 0 {
		p.doSize()
	}
	return p.width
}

func (p *Game) size() (int, int) {
	if p.width == 0 {
		p.doSize()
	}
	return p.width, p.height
}

func (p *Game) doSize() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.width == 0 {
		c := p.costumes[p.currentCostumeIndex]
		img, _, _ := c.needImage(p.fs)
		w, h := img.Size()
		p.width, p.height = w/c.bitmapResolution, h/c.bitmapResolution
	}
}

// -----------------------------------------------------------------------------

func (p *Game) getTurtle() turtleCanvas {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	return p.turtle
}

func (p *Sprite) Clear() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.turtle.clear()
}

func (p *Game) stampCostume(di *spriteDrawInfo) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.turtle.stampCostume(di)
}

func (p *Game) movePen(sp *Sprite, x, y float64) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	screenW, screenH := p.size()
	p.turtle.penLine(&penLine{
		x1:    (screenW >> 1) + int(sp.x),
		y1:    (screenH >> 1) - int(sp.y),
		x2:    (screenW >> 1) + int(x),
		y2:    (screenH >> 1) - int(y),
		clr:   sp.penColor,
		width: int(sp.penWidth),
	})
}

// -----------------------------------------------------------------------------

func (p *Game) SceneName() string {
	return p.costumeName()
}

func (p *Game) SceneIndex() int {
	return p.costumeIndex()
}

// StartScene func:
//   StartScene(sceneName) or
//   StartScene(sceneIndex) or
//   StartScene(spx.Next)
//   StartScene(spx.Prev)
func (p *Game) StartScene(scene interface{}, wait ...bool) {
	if p.setCostume(scene) {
		// TODO: send event & wait
	}
}

func (p *Game) NextScene(wait ...bool) {
	p.StartScene(Next, wait...)
}

// -----------------------------------------------------------------------------

type Key int

func (p *Game) KeyPressed(key Key) bool {
	panic("todo")
}

func (p *Game) MouseX() float64 {
	panic("todo")
}

func (p *Game) MouseY() float64 {
	panic("todo")
}

func (p *Game) MousePressed() bool {
	panic("todo")
}

func (p *Game) Username() string {
	panic("todo")
}

// -----------------------------------------------------------------------------

func (p *Game) Wait(secs float64) {
	panic("todo")
}

func (p *Game) Timer() float64 {
	panic("todo")
}

func (p *Game) ResetTimer() {
	panic("todo")
}

// -----------------------------------------------------------------------------

func (p *Game) Ask(ask string) {
	panic("todo")
}

func (p *Game) Answer() Value {
	panic("todo")
}

// -----------------------------------------------------------------------------

type EffectKind int

func (p *Game) SetEffect(kind EffectKind, val float64) {
	panic("todo")
}

func (p *Game) ChangeEffect(kind EffectKind, delta float64) {
	panic("todo")
}

func (p *Game) ClearEffects() {
	panic("todo")
}

// -----------------------------------------------------------------------------

// Play func:
//   Play(sound)
//   Play(video) -- maybe
func (p *Game) Play(media interface{}, secs ...float64) {
	panic("todo")
}

func (p *Game) StopAllSounds() {
	panic("todo")
}

func (p *Game) Volume() float64 {
	panic("todo")
}

func (p *Game) SetVolume(volume float64) {
	panic("todo")
}

func (p *Game) ChangeVolume(delta float64) {
	panic("todo")
}

// -----------------------------------------------------------------------------

func (p *Game) Broadcast(msg string, data interface{}, wait ...bool) {
	panic("todo")
}

// -----------------------------------------------------------------------------
