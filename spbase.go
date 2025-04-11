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
	"math"
	"path"
	"strconv"

	_ "image/jpeg" // for image decode
	_ "image/png"  // for image decode

	"github.com/realdream-ai/mathf"

	"github.com/goplus/spx/internal/engine"
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

type costumeSetImage struct {
	path  string
	rc    costumeSetRect
	width int
	nx    int
}

// -------------------------------------------------------------------------------------

type costume struct {
	name          SpriteCostumeName
	width, height int
	center        mathf.Vec2 // center point

	faceRight        float64
	bitmapResolution int
	path             string

	setIndex   int // costume index
	posX, posY int // left top
}

func newCostumeWithSize(width, height int) *costume {
	value := &costume{
		setIndex: -1,
		width:    width, height: height,
		bitmapResolution: 1,
	}
	value.posX = 0
	value.posY = 0
	value.center.X = float64(value.width) / 2
	value.center.Y = float64(value.height) / 2
	return value
}

func newCostumeWith(name string, img *costumeSetImage, faceRight float64, i, bitmapResolution int) *costume {
	value := &costume{
		path: img.path,
		name: name, setIndex: i,
		faceRight: faceRight, bitmapResolution: bitmapResolution,
	}
	imageSize := getCustomeAssetSize(img.path)
	value.width = int(imageSize.X) / img.nx
	value.height = int(imageSize.Y)
	value.posX = i * value.width
	value.posY = 0
	if img.rc.H != 0 {
		value.width = int(img.rc.W) / img.nx
		value.height = int(img.rc.H)
		value.posX = int(img.rc.X) + i*value.width
		value.posY = int(img.rc.Y)
	}
	value.center.X = float64(value.width) / 2
	value.center.Y = float64(value.height) / 2
	return value
}

func newCostume(base string, c *costumeConfig) *costume {
	path := path.Join(base, c.Path)
	value := &costume{
		name:             c.Name,
		setIndex:         -1,
		center:           mathf.Vec2{X: c.X, Y: c.Y},
		faceRight:        c.FaceRight,
		bitmapResolution: toBitmapResolution(c.BitmapResolution),
		path:             path,
	}
	imageSize := getCustomeAssetSize(path)
	value.width = int(imageSize.X)
	value.height = int(imageSize.Y)
	value.posX = 0
	value.posY = 0
	return value
}

func getCustomeAssetSize(path string) mathf.Vec2 {
	assetPath := engine.ToAssetPath(path)
	return resMgr.GetImageSize(assetPath)
}

func toBitmapResolution(v int) int {
	if v == 0 {
		return 1
	}
	return v
}

func (p *costume) getSize() (int, int) {
	return p.width / p.bitmapResolution, p.height / p.bitmapResolution
}
func (p *costume) isAltas() bool {
	return p.setIndex >= 0
}

// -------------------------------------------------------------------------------------

type baseObj struct {
	costumes      []*costume
	costumeIndex_ int
	// !!!All methods of this object (except GetId()) can only be called on the main thread
	syncSprite     *engine.Sprite
	scale          float64
	HasDestroyed   bool
	isCostumeSet   bool
	isCostumeDirty bool

	layer        int
	isLayerDirty bool
}

func (p *baseObj) setLayer(layer int) { // dying: visible but can't be touched
	if p.layer != layer {
		p.layer = layer
		p.isLayerDirty = true
	}
}

func (p *baseObj) setCustumeIndex(value int) {
	p.costumeIndex_ = value
	p.isCostumeDirty = true
}

func (p *baseObj) getProxy() *engine.Sprite {
	return p.syncSprite
}

func (p *baseObj) initWith(base string, sprite *spriteConfig) {
	if sprite.CostumeSet != nil {
		initWithCS(p, base, sprite.CostumeSet)
	} else if sprite.CostumeMPSet != nil {
		initWithCMPS(p, base, sprite.CostumeMPSet)
	} else {
		panic("sprite.init should have one of costumes, costumeSet and costumeMPSet")
	}
	nx := len(p.costumes)
	costumeIndex := sprite.getCostumeIndex()
	if costumeIndex >= nx || costumeIndex < 0 {
		costumeIndex = 0
	}
	p.setCustumeIndex(costumeIndex)
}

func initWithCMPS(p *baseObj, base string, cmps *costumeMPSet) {
	faceRight, bitmapResolution := cmps.FaceRight, toBitmapResolution(cmps.BitmapResolution)
	imgPath := path.Join(base, cmps.Path)

	for _, cs := range cmps.Parts {
		img := &costumeSetImage{path: imgPath, rc: cs.Rect, nx: cs.Nx}
		initCSPart(p, img, faceRight, bitmapResolution, cs.Nx, cs.Items)
	}
}

func initWithCS(p *baseObj, base string, cs *costumeSet) {
	nx := cs.Nx
	imgPath := path.Join(base, cs.Path)
	var img *costumeSetImage
	if cs.Rect == nil {
		img = &costumeSetImage{path: imgPath, nx: nx}
	} else {
		img = &costumeSetImage{path: imgPath, rc: *cs.Rect, nx: nx}
	}
	p.costumes = make([]*costume, 0, nx)
	initCSPart(p, img, cs.FaceRight, toBitmapResolution(cs.BitmapResolution), nx, cs.Items)
}

func initCSPart(p *baseObj, img *costumeSetImage, faceRight float64, bitmapResolution, nx int, items []costumeSetItem) {
	p.isCostumeSet = true
	if nx == 1 {
		name := strconv.Itoa(len(p.costumes))
		addCostumeWith(p, name, img, faceRight, 0, bitmapResolution)
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
	p.setCustumeIndex(costumeIndex)
}

func (p *baseObj) init(base string, costumes []*costumeConfig, costumeIndex int) {
	p.costumes = make([]*costume, len(costumes))
	for i, c := range costumes {
		p.costumes[i] = newCostume(base, c)
	}
	if costumeIndex >= len(costumes) || costumeIndex < 0 {
		costumeIndex = 0
	}
	p.isLayerDirty = true
	p.setCustumeIndex(costumeIndex)
}

func (p *baseObj) initWithSize(width, height int) {
	p.costumes = make([]*costume, 1)
	p.costumes[0] = newCostumeWithSize(width, height)
	p.setCustumeIndex(0)

}

func (p *baseObj) initFrom(src *baseObj) {
	p.costumes = src.costumes
	p.setCustumeIndex(src.costumeIndex_)
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
	case BackdropName:
		return p.setCostumeByName(SpriteCostumeName(v))
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
	isDirty := p.costumeIndex_ != idx
	p.setCustumeIndex(idx)
	return isDirty
}
func (p *baseObj) setCostumeByName(name SpriteCostumeName) bool {
	if idx := p.findCostume(name); idx >= 0 {
		return p.setCostumeByIndex(idx)
	}
	return false
}

func (p *baseObj) goPrevCostume() {
	index := (len(p.costumes) + p.costumeIndex_ - 1) % len(p.costumes)
	p.setCustumeIndex(index)
}

func (p *baseObj) goNextCostume() {
	index := (p.costumeIndex_ + 1) % len(p.costumes)
	p.setCustumeIndex(index)
}

func (p *baseObj) getCostumeIndex() int {
	return p.costumeIndex_
}

func (p *baseObj) getCostumeName() SpriteCostumeName {
	return p.costumes[p.costumeIndex_].name
}
func (p *baseObj) getCostumePath() string {
	return p.costumes[p.costumeIndex_].path
}
func (p *baseObj) getCostumeRenderScale() float64 {
	return 1.0 / float64(p.costumes[p.costumeIndex_].bitmapResolution) * p.scale
}
func (p *baseObj) getCostumeSize() (float64, float64) {
	x, y := p.costumes[p.costumeIndex_].getSize()
	return float64(x), float64(y)
}
func (p *baseObj) isCostumeAltas() bool {
	//println("p.costumeIndex_ ", p.costumeIndex_, " len ", len(p.costumes), " isAltas ", p.costumes[p.costumeIndex_].isAltas())
	return p.costumes[p.costumeIndex_].isAltas()
}

func (p *baseObj) getCostumeAltasRegion() mathf.Rect2 {
	costume := p.costumes[p.costumeIndex_]
	rect := mathf.NewRect2(float64(costume.posX), float64(costume.posY),
		float64(costume.width), float64(costume.height))
	return rect
}

// -------------------------------------------------------------------------------------
