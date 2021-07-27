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
	"sync"
)

type Base struct {
	costumes []*costume

	mutex               sync.Mutex
	currentCostumeIndex int
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

func (p *Base) findCostume(name string) int {
	for i, c := range p.costumes {
		if c.name == name {
			return i
		}
	}
	return -1
}

func (p *Base) setCostume(val interface{}) bool {
	switch v := val.(type) {
	case string:
		return p.setCostumeByName(v)
	case int:
		return p.setCostumeByIndex(v)
	case SwitchAction:
		if v == Prev {
			p.prevCostume()
		} else {
			p.nextCostume()
		}
		return true
	default:
		panic("setCostume: invalid argument type")
	}
}

func (p *Base) setCostumeByIndex(idx int) bool {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	if idx >= len(p.costumes) {
		panic("invalid costume index")
	}
	if p.currentCostumeIndex != idx {
		p.currentCostumeIndex = idx
		return true
	}
	return false
}

func (p *Base) setCostumeByName(name string) bool {
	if idx := p.findCostume(name); idx >= 0 {
		return p.setCostumeByIndex(idx)
	}
	return false
}

func (p *Base) prevCostume() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.currentCostumeIndex = (len(p.costumes) + p.currentCostumeIndex - 1) % len(p.costumes)
}

func (p *Base) nextCostume() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.currentCostumeIndex = (p.currentCostumeIndex + 1) % len(p.costumes)
}

func (p *Base) costumeIndex() int {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.currentCostumeIndex
}

func (p *Base) costumeName() string {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.costumes[p.currentCostumeIndex].name
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
