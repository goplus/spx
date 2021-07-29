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

	spxfs "github.com/goplus/spx/fs"
)

const (
	defaultFilterMode = ebiten.FilterLinear
)

func toRadian(dir float64) float64 {
	return math.Pi * dir / 180
}

func normalizeDirection(dir float64) float64 {
	if dir <= -180 {
		dir += 360
	} else if dir > 180 {
		dir -= 360
	}
	return dir
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

type SwitchAction int

const (
	Prev SwitchAction = -1
	Next SwitchAction = 1
)

// -------------------------------------------------------------------------------------

// costume class.
type costume struct {
	name string
	path string

	bitmapResolution int

	x, y  float64
	cache *ebiten.Image
	mutex sync.Mutex
}

func (p *costume) needImage(fs spxfs.Dir) (*ebiten.Image, float64, float64) {
	if p.cache == nil {
		p.doNeedImage(fs)
	}
	return p.cache, p.x, p.y
}

func (p *costume) doNeedImage(fs spxfs.Dir) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.cache == nil {
		f, err := fs.Open(p.path)
		if err != nil {
			panic(errors.Wrapf(err, "costume open file `%s` failed", p.path))
		}
		defer f.Close()

		img, _, err := image.Decode(f)
		if err != nil {
			panic(errors.Wrapf(err, "costume file `%s` is not an image", p.path))
		}

		p.cache, err = ebiten.NewImageFromImage(img, defaultFilterMode)
		if err != nil {
			panic(errors.Wrapf(err, "costume file `%s`: image is too big (or too small)", p.path))
		}
	}
}

// -------------------------------------------------------------------------------------

type baseObj struct {
	costumes []*costume

	mutex               sync.Mutex
	currentCostumeIndex int
}

func (p *baseObj) init(base string, costumes []costumeConfig, currentCostumeIndex int) {
	p.costumes = make([]*costume, len(costumes))
	for i, c := range costumes {
		p.costumes[i] = &costume{
			name: c.Name, path: base + c.Path, x: c.X, y: c.Y,
			bitmapResolution: c.BitmapResolution,
		}
	}
	if currentCostumeIndex >= len(costumes) || currentCostumeIndex < 0 {
		currentCostumeIndex = 0
	}
	p.currentCostumeIndex = currentCostumeIndex
}

func (p *baseObj) findCostume(name string) int {
	for i, c := range p.costumes {
		if c.name == name {
			return i
		}
	}
	return -1
}

func (p *baseObj) goSetCostume(val interface{}) bool {
	switch v := val.(type) {
	case string:
		return p.setCostumeByName(v)
	case int:
		return p.setCostumeByIndex(v)
	case SwitchAction:
		if v == Prev {
			p.goPrevCostume()
		} else {
			p.goNextCostume()
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

func (p *baseObj) goPrevCostume() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.currentCostumeIndex = (len(p.costumes) + p.currentCostumeIndex - 1) % len(p.costumes)
}

func (p *baseObj) goNextCostume() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.currentCostumeIndex = (p.currentCostumeIndex + 1) % len(p.costumes)
}

func (p *baseObj) getCostumeIndex() int {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.currentCostumeIndex
}

func (p *baseObj) getCostumeName() string {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	return p.costumes[p.currentCostumeIndex].name
}

// -------------------------------------------------------------------------------------
