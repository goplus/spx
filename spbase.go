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
	"image"
	"math"
	"sync"

	"github.com/hajimehoshi/ebiten"
	"github.com/pkg/errors"
)

const (
	defaultFilterMode = ebiten.FilterLinear
)

func toRadian(dir float64) float64 {
	return math.Pi * dir / 180
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

// -------------------------------------------------------------------------------------

type drawContext struct {
	*ebiten.Image
}

type hitContext struct {
	Pos image.Point
}

type hitResult struct {
	Target interface{}
}

type shape interface {
	draw(dc drawContext)
	hit(hc hitContext) (hr hitResult, ok bool)
}

// -------------------------------------------------------------------------------------

// costume class.
type costume struct {
	name string
	file string

	bitmapResolution int

	cx    float64
	cy    float64
	cache *ebiten.Image
	mutex sync.Mutex
}

func (p *costume) needImage(fs FileSystem) (*ebiten.Image, float64, float64) {
	if p.cache == nil {
		p.doNeedImage(fs)
	}
	return p.cache, p.cx, p.cy
}

func (p *costume) doNeedImage(fs FileSystem) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.cache == nil {
		f, err := fs.Open(p.file)
		if err != nil {
			panic(errors.Wrapf(err, "costume open file `%s` failed", p.file))
		}
		defer f.Close()

		img, _, err := image.Decode(f)
		if err != nil {
			panic(errors.Wrapf(err, "costume file `%s` is not an image", p.file))
		}

		p.cache, err = ebiten.NewImageFromImage(img, defaultFilterMode)
		if err != nil {
			panic(errors.Wrapf(err, "costume file `%s`: image is too big (or too small)", p.file))
		}
	}
}

// -------------------------------------------------------------------------------------

type baseObj struct {
	costumes []*costume

	mutex               sync.Mutex
	currentCostumeIndex int
}

func (p *baseObj) findCostume(name string) int {
	for i, c := range p.costumes {
		if c.name == name {
			return i
		}
	}
	return -1
}

func (p *baseObj) setCostume(val interface{}) bool {
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

func (p *baseObj) setCostumeByIndex(idx int) bool {
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

func (p *baseObj) setCostumeByName(name string) bool {
	if idx := p.findCostume(name); idx >= 0 {
		return p.setCostumeByIndex(idx)
	}
	return false
}

func (p *baseObj) prevCostume() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.currentCostumeIndex = (len(p.costumes) + p.currentCostumeIndex - 1) % len(p.costumes)
}

func (p *baseObj) nextCostume() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.currentCostumeIndex = (p.currentCostumeIndex + 1) % len(p.costumes)
}

func (p *baseObj) costumeIndex() int {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.currentCostumeIndex
}

func (p *baseObj) costumeName() string {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.costumes[p.currentCostumeIndex].name
}

// -------------------------------------------------------------------------------------
