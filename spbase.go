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
	"strconv"
	"sync"

	_ "image/jpeg" // for image decode
	_ "image/png"  // for image decode

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

type switchAction int

const (
	Prev switchAction = -1
	Next switchAction = 1
)

// -------------------------------------------------------------------------------------

type imagePoint struct {
	x, y float64
}

type imageLoaderByPath string

func (path imageLoaderByPath) load(fs spxfs.Dir, pt *imagePoint) (*ebiten.Image, error) {
	f, err := fs.Open(string(path))
	if err != nil {
		return nil, errors.Wrapf(err, "imageLoader: open file `%s` failed", path)
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, errors.Wrapf(err, "imageLoader: file `%s` is not an image", path)
	}

	ret, err := ebiten.NewImageFromImage(img, defaultFilterMode)
	if err != nil {
		return nil, errors.Wrapf(err, "imageLoader open `%s`: image is too big (or too small)", path)
	}
	return ret, nil
}

// -------------------------------------------------------------------------------------

type delayloadImage struct {
	mutex  sync.Mutex
	cache  *ebiten.Image
	pt     imagePoint
	loader func(fs spxfs.Dir, pt *imagePoint) (*ebiten.Image, error)
}

func (p *delayloadImage) ensure(fs spxfs.Dir) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.cache == nil {
		var err error
		if p.cache, err = p.loader(fs, &p.pt); err != nil {
			panic(err)
		}
	}
}

type costumeSetImage struct {
	mutex  sync.Mutex
	cache  *ebiten.Image
	loader func(fs spxfs.Dir, pt *imagePoint) (*ebiten.Image, error)
	width  int
	nx     int
}

func (p *costumeSetImage) ensure(fs spxfs.Dir) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.cache == nil {
		var err error
		if p.cache, err = p.loader(fs, nil); err != nil {
			panic(err)
		}
		p.width = p.cache.Bounds().Dx() / p.nx
	}
}

type imageLoaderByCostumeSet struct {
	costumeSet *costumeSetImage
	index      int
}

func (p imageLoaderByCostumeSet) load(fs spxfs.Dir, pt *imagePoint) (*ebiten.Image, error) {
	costumeSet := p.costumeSet
	if costumeSet.cache == nil {
		p.costumeSet.ensure(fs)
	}
	cache, width := costumeSet.cache, costumeSet.width
	bounds := cache.Bounds()
	min := image.Point{X: bounds.Min.X + width*p.index, Y: bounds.Min.Y}
	max := image.Point{X: min.X + width, Y: bounds.Max.Y}
	pt.x, pt.y = float64(width>>1), float64(bounds.Dy()>>1)
	if img := cache.SubImage(image.Rectangle{Min: min, Max: max}); img != nil {
		return img.(*ebiten.Image), nil
	}
	panic("disposed image")
}

// -------------------------------------------------------------------------------------

type costume struct {
	name string
	img  delayloadImage

	bitmapResolution int
}

func newCostumeWith(name string, img *costumeSetImage, i int, bitmapResolution int) *costume {
	loader := imageLoaderByCostumeSet{costumeSet: img, index: i}.load
	return &costume{
		name: name, img: delayloadImage{loader: loader},
		bitmapResolution: bitmapResolution,
	}
}

func newCostume(base string, c *costumeConfig) *costume {
	loader := imageLoaderByPath(base + c.Path).load
	return &costume{
		name: c.Name, img: delayloadImage{loader: loader, pt: imagePoint{c.X, c.Y}},
		bitmapResolution: c.BitmapResolution,
	}
}

func (p *costume) needImage(fs spxfs.Dir) (*ebiten.Image, float64, float64) {
	if p.img.cache == nil {
		p.img.ensure(fs)
	}
	return p.img.cache, p.img.pt.x, p.img.pt.y
}

// -------------------------------------------------------------------------------------

type baseObj struct {
	costumes []*costume

	mutex               sync.Mutex
	currentCostumeIndex int
}

func (p *baseObj) initWith(base string, cs *costumeSet, currentCostumeIndex int) {
	nx, bitmapResolution := cs.Nx, cs.BitmapResolution
	costumeSetLoader := imageLoaderByPath(base + cs.Path).load
	img := &costumeSetImage{loader: costumeSetLoader, nx: nx}
	p.costumes = make([]*costume, nx)
	if cs.Items == nil {
		for index := 0; index < nx; index++ {
			p.costumes[index] = newCostumeWith(strconv.Itoa(index), img, index, bitmapResolution)
		}
	} else {
		index := 0
		for _, item := range cs.Items {
			for i := 0; i < item.N; i++ {
				name := item.NamePrefix + strconv.Itoa(i)
				p.costumes[i] = newCostumeWith(name, img, index, bitmapResolution)
				index++
			}
		}
		if index != nx {
			panic("costumeSet load uncompleted")
		}
	}
	if currentCostumeIndex >= nx || currentCostumeIndex < 0 {
		currentCostumeIndex = 0
	}
	p.currentCostumeIndex = currentCostumeIndex
}

func (p *baseObj) init(base string, costumes []*costumeConfig, currentCostumeIndex int) {
	p.costumes = make([]*costume, len(costumes))
	for i, c := range costumes {
		p.costumes[i] = newCostume(base, c)
	}
	if currentCostumeIndex >= len(costumes) || currentCostumeIndex < 0 {
		currentCostumeIndex = 0
	}
	p.currentCostumeIndex = currentCostumeIndex
}

func (p *baseObj) initFrom(src *baseObj) {
	p.costumes = src.costumes
	p.mutex = sync.Mutex{}
	p.currentCostumeIndex = src.currentCostumeIndex
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
	case switchAction:
		if v == Prev {
			p.goPrevCostume()
		} else {
			p.goNextCostume()
		}
		return true
	case float64:
		return p.setCostumeByIndex(int(v))
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

func (p *baseObj) goCostume(step int) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.currentCostumeIndex = (len(p.costumes) + p.currentCostumeIndex + step) % len(p.costumes)
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
