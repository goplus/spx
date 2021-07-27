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

type Base struct {
}

type Value struct {
}

func (p Value) String() string {
	panic("todo")
}

func (p Value) Int() int {
	panic("todo")
}

func (p Value) Float() float64 {
	panic("todo")
}

// -----------------------------------------------------------------------------

type Key int

func (p *Base) KeyPressed(key Key) bool {
	panic("todo")
}

func (p *Base) MouseX() float64 {
	panic("todo")
}

func (p *Base) MouseY() float64 {
	panic("todo")
}

func (p *Base) MousePressed() bool {
	panic("todo")
}

func (p *Base) Username() string {
	panic("todo")
}

// -----------------------------------------------------------------------------

func (p *Base) Wait(secs float64) {
	panic("todo")
}

func (p *Base) Timer() float64 {
	panic("todo")
}

func (p *Base) ResetTimer() {
	panic("todo")
}

// -----------------------------------------------------------------------------

func (p *Base) Ask(ask string) {
	panic("todo")
}

func (p *Base) Answer() Value {
	panic("todo")
}

// -----------------------------------------------------------------------------

type EffectKind int

func (p *Base) SetEffect(kind EffectKind, val float64) {
	panic("todo")
}

func (p *Base) ChangeEffect(kind EffectKind, delta float64) {
	panic("todo")
}

func (p *Base) ClearEffects() {
	panic("todo")
}

// -----------------------------------------------------------------------------

// Play func:
//   Play(sound)
//   Play(video) -- maybe
func (p *Base) Play(media interface{}, secs ...float64) {
	panic("todo")
}

func (p *Base) StopAllSounds() {
	panic("todo")
}

func (p *Base) Volume() float64 {
	panic("todo")
}

func (p *Base) SetVolume(volume float64) {
	panic("todo")
}

func (p *Base) ChangeVolume(delta float64) {
	panic("todo")
}

// -----------------------------------------------------------------------------

func (p *Base) Broadcast(msg string, data interface{}, wait ...bool) {
	panic("todo")
}

// -----------------------------------------------------------------------------
