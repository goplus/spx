/*
 * Copyright (c) 2021 The GoPlus Authors (goplus.org). All rights reserved.
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

import (
	"image"
	"math"
	"path"
	"strconv"

	_ "image/jpeg" // for image decode
	_ "image/png"  // for image decode

	"github.com/pkg/errors"

	spxfs "github.com/goplus/spx/fs"
	"github.com/goplus/spx/internal/gdi"
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

type imageLoader interface {
	load(fs spxfs.Dir, pt *imagePoint) (gdi.Image, error)
}

// -------------------------------------------------------------------------------------

type imageLoaderByPath string

func (path imageLoaderByPath) load(fs spxfs.Dir, pt *imagePoint) (ret gdi.Image, err error) {
	f, err := fs.Open(string(path))
	if err != nil {
		err = errors.Wrapf(err, "imageLoader: open file `%s` failed", path)
		return
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		err = errors.Wrapf(err, "imageLoader: file `%s` is not an image", path)
		return
	}
	return gdi.NewImageFrom(img), nil
}

// -------------------------------------------------------------------------------------

type delayloadImage struct {
	cache  gdi.Image
	pt     imagePoint
	loader imageLoader
}

func (p *delayloadImage) ensure(fs spxfs.Dir) {
	if !p.cache.IsValid() {
		var err error
		if p.cache, err = p.loader.load(fs, &p.pt); err != nil {
			panic(err)
		}
	}
}

type costumeSetImage struct {
	cache  gdi.Image
	loader imageLoader
	width  int
	nx     int
}

func (p *costumeSetImage) ensure(fs spxfs.Dir) {
	if !p.cache.IsValid() {
		var err error
		if p.cache, err = p.loader.load(fs, nil); err != nil {
			panic(err)
		}
		p.width = p.cache.Bounds().Dx() / p.nx
	}
}

type sharedImages struct {
	imgs map[string]gdi.Image
}

type sharedImage struct {
	shared *sharedImages
	path   string
	rc     costumeSetRect
}

func (p *sharedImage) load(fs spxfs.Dir, pt *imagePoint) (ret gdi.Image, err error) {
	path := p.path
	shared, ok := p.shared.imgs[path]
	if !ok {
		var tmp imagePoint
		if shared, err = imageLoaderByPath(path).load(fs, &tmp); err != nil {
			return
		}
		p.shared.imgs[path] = shared
	}
	rc := p.rc
	min := image.Point{X: int(rc.X), Y: int(rc.Y)}
	max := image.Point{X: int(rc.X + rc.W), Y: int(rc.Y + rc.H)}
	if pt != nil {
		pt.x, pt.y = rc.W/2, rc.H/2
	}

	if sub := shared.SubImage(image.Rectangle{Min: min, Max: max}); sub.IsValid() {
		return sub, nil
	}
	panic("disposed image")
}

// -------------------------------------------------------------------------------------

type imageLoaderByCostumeSet struct {
	costumeSet *costumeSetImage
	index      int
}

func (p *imageLoaderByCostumeSet) load(fs spxfs.Dir, pt *imagePoint) (gdi.Image, error) {
	costumeSet := p.costumeSet
	if !costumeSet.cache.IsValid() {
		p.costumeSet.ensure(fs)
	}
	cache, width := costumeSet.cache, costumeSet.width
	bounds := cache.Bounds()
	min := image.Point{X: bounds.Min.X + width*p.index, Y: bounds.Min.Y}
	max := image.Point{X: min.X + width, Y: bounds.Max.Y}
	pt.x, pt.y = float64(width>>1), float64(bounds.Dy()>>1)
	if img := cache.SubImage(image.Rectangle{Min: min, Max: max}); img.IsValid() {
		return img, nil
	}
	panic("disposed image")
}

// -------------------------------------------------------------------------------------

type costume struct {
	name             SpriteCostumeName
	img              delayloadImage
	faceRight        float64
	bitmapResolution int
}

func newCostumeWithSize(width, height int) *costume {
	return &costume{
		img:              delayloadImage{cache: gdi.NewImageSize(width, height)},
		bitmapResolution: 1,
	}
}

func newCostumeWith(name string, img *costumeSetImage, faceRight float64, i, bitmapResolution int) *costume {
	var loader imageLoader
	if i < 0 {
		loader = img.loader
	} else {
		loader = &imageLoaderByCostumeSet{costumeSet: img, index: i}
	}
	return &costume{
		name: name, img: delayloadImage{loader: loader},
		faceRight: faceRight, bitmapResolution: bitmapResolution,
	}
}

func newCostume(base string, c *costumeConfig) *costume {
	loader := imageLoaderByPath(path.Join(base, c.Path))
	return &costume{
		name:             c.Name,
		img:              delayloadImage{loader: loader, pt: imagePoint{c.X, c.Y}},
		faceRight:        c.FaceRight,
		bitmapResolution: toBitmapResolution(c.BitmapResolution),
	}
}

func toBitmapResolution(v int) int {
	if v == 0 {
		return 1
	}
	return v
}

func (p *costume) needImage(fs spxfs.Dir) (gdi.Image, float64, float64) {
	if !p.img.cache.IsValid() {
		p.img.ensure(fs)
	}
	return p.img.cache, p.img.pt.x, p.img.pt.y
}

// -------------------------------------------------------------------------------------

type baseObj struct {
	costumes      []*costume
	costumeIndex_ int
}

func (p *baseObj) initWith(base string, sprite *spriteConfig, shared *sharedImages) {
	if sprite.CostumeSet != nil {
		initWithCS(p, base, sprite.CostumeSet, shared)
	} else if sprite.CostumeMPSet != nil {
		initWithCMPS(p, base, sprite.CostumeMPSet, shared)
	} else {
		panic("sprite.init should have one of costumes, costumeSet and costumeMPSet")
	}
	nx := len(p.costumes)
	costumeIndex := sprite.getCostumeIndex()
	if costumeIndex >= nx || costumeIndex < 0 {
		costumeIndex = 0
	}
	p.costumeIndex_ = costumeIndex
}

func initWithCMPS(p *baseObj, base string, cmps *costumeMPSet, shared *sharedImages) {
	faceRight, bitmapResolution := cmps.FaceRight, toBitmapResolution(cmps.BitmapResolution)
	imgPath := path.Join(base, cmps.Path)
	for _, cs := range cmps.Parts {
		simg := &sharedImage{shared: shared, path: imgPath, rc: cs.Rect}
		img := &costumeSetImage{loader: simg, nx: cs.Nx}
		initCSPart(p, img, faceRight, bitmapResolution, cs.Nx, cs.Items)
	}
}

func initWithCS(p *baseObj, base string, cs *costumeSet, shared *sharedImages) {
	nx := cs.Nx
	imgPath := path.Join(base, cs.Path)
	var img *costumeSetImage
	if cs.Rect == nil {
		costumeSetLoader := imageLoaderByPath(imgPath)
		img = &costumeSetImage{loader: costumeSetLoader, nx: nx}
	} else {
		simg := &sharedImage{shared: shared, path: imgPath, rc: *cs.Rect}
		img = &costumeSetImage{loader: simg, nx: nx}
	}
	p.costumes = make([]*costume, 0, nx)
	initCSPart(p, img, cs.FaceRight, toBitmapResolution(cs.BitmapResolution), nx, cs.Items)
}

func initCSPart(p *baseObj, img *costumeSetImage, faceRight float64, bitmapResolution, nx int, items []costumeSetItem) {
	if nx == 1 {
		name := strconv.Itoa(len(p.costumes))
		addCostumeWith(p, name, img, faceRight, -1, bitmapResolution)
	} else if items == nil {
		for index := 0; index < nx; index++ {
			name := strconv.Itoa(len(p.costumes))
			addCostumeWith(p, name, img, faceRight, index, bitmapResolution)
		}
	} else {
		index := 0
		for _, item := range items {
			for i := 0; i < item.N; i++ {
				name := item.NamePrefix + strconv.Itoa(i)
				addCostumeWith(p, name, img, faceRight, index, bitmapResolution)
				index++
			}
		}
		if index != nx {
			panic("initCostumeSetPart: load uncompleted")
		}
	}
}

func addCostumeWith(p *baseObj, name SpriteCostumeName, img *costumeSetImage, faceRight float64, i, bitmapResolution int) {
	c := newCostumeWith(name, img, faceRight, i, bitmapResolution)
	p.costumes = append(p.costumes, c)
}

func (p *baseObj) initBackdrops(base string, costumes []*backdropConfig, costumeIndex int) {
	p.costumes = make([]*costume, len(costumes))
	for i, c := range costumes {
		p.costumes[i] = newCostume(base, &c.costumeConfig) // has error how to fixed it
	}
	if costumeIndex >= len(costumes) || costumeIndex < 0 {
		costumeIndex = 0
	}
	p.costumeIndex_ = costumeIndex
}

func (p *baseObj) init(base string, costumes []*costumeConfig, costumeIndex int) {
	p.costumes = make([]*costume, len(costumes))
	for i, c := range costumes {
		p.costumes[i] = newCostume(base, c)
	}
	if costumeIndex >= len(costumes) || costumeIndex < 0 {
		costumeIndex = 0
	}
	p.costumeIndex_ = costumeIndex
}

func (p *baseObj) initWithSize(width, height int) {
	p.costumes = make([]*costume, 1)
	p.costumes[0] = newCostumeWithSize(width, height)
	p.costumeIndex_ = 0
}

func (p *baseObj) initFrom(src *baseObj) {
	p.costumes = src.costumes
	p.costumeIndex_ = src.costumeIndex_
}

func (p *baseObj) findCostume(name SpriteCostumeName) int {
	for i, c := range p.costumes {
		if c.name == name {
			return i
		}
	}
	return -1
}

func (p *baseObj) goSetCostume(val interface{}) bool {
	switch v := val.(type) {
	case SpriteCostumeName:
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
	if idx >= len(p.costumes) {
		panic("invalid costume index")
	}
	if p.costumeIndex_ != idx {
		p.costumeIndex_ = idx
		return true
	}
	return false
}
func (p *baseObj) setCostumeByName(name SpriteCostumeName) bool {
	if idx := p.findCostume(name); idx >= 0 {
		return p.setCostumeByIndex(idx)
	}
	return false
}

func (p *baseObj) goPrevCostume() {
	p.costumeIndex_ = (len(p.costumes) + p.costumeIndex_ - 1) % len(p.costumes)
}

func (p *baseObj) goNextCostume() {
	p.costumeIndex_ = (p.costumeIndex_ + 1) % len(p.costumes)
}

func (p *baseObj) getCostumeIndex() int {
	return p.costumeIndex_
}

func (p *baseObj) getCostumeName() SpriteCostumeName {
	return p.costumes[p.costumeIndex_].name
}

// -------------------------------------------------------------------------------------
