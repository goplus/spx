/*
 * Copyright (c) 2021 The XGo Authors (xgo.dev). All rights reserved.
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

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v2/internal/engine"
)

// -------------------------------------------------------------------------------------
// Constants and enums
// -------------------------------------------------------------------------------------

const (
	defaultBitmapResolution = 1
	shaderPath              = "res://engine/shader/spx_sprite_shader.gdshader"
	fullUVRange             = 1.0
)

// switchAction represents the direction for switching costumes.
type switchAction int

const (
	Prev switchAction = -1
	Next switchAction = 1
)

// layerAction represents the direction for layer changes.
type layerAction int

const (
	Front layerAction = -1
	Back  layerAction = 1
)

// dirAction represents forward or backward direction.
type dirAction int

const (
	Forward  dirAction = -1
	Backward dirAction = 1
)

// -------------------------------------------------------------------------------------
// Utility functions
// -------------------------------------------------------------------------------------

// toRadian converts degrees to radians.
func toRadian(dir float64) float64 {
	return math.Pi * dir / 180
}

// normalizeDirection normalizes a direction angle to the range (-180, 180].
func normalizeDirection(dir float64) float64 {
	if dir <= -180 {
		dir += 360
	} else if dir > 180 {
		dir -= 360
	}
	return dir
}

// toBitmapResolution returns a valid bitmap resolution value.
func toBitmapResolution(v int) int {
	if v == 0 {
		return defaultBitmapResolution
	}
	return v
}

// -------------------------------------------------------------------------------------
// Costume set image structure
// -------------------------------------------------------------------------------------

// costumeSetImage represents metadata for a costume set image.
type costumeSetImage struct {
	path   string
	rc     costumeSetRect
	width  float64
	height float64
	nx     int // number of frames in the image
}

// -------------------------------------------------------------------------------------
// Costume structure and methods
// -------------------------------------------------------------------------------------

// costume represents a single costume (image frame) for a sprite or backdrop.
type costume struct {
	name          SpriteCostumeName
	width, height int
	center        mathf.Vec2 // center point
	imageSize     mathf.Vec2 // actual image dimensions

	faceRight        float64
	bitmapResolution int
	path             string

	setIndex   int // costume index in set (-1 if not part of a set)
	posX, posY int // position in atlas (left top corner)

	atlasUVRect mathf.Vec4 // UV coordinates for atlas texture
}

// newCostumeWithSize creates a costume with specified dimensions (no image file).
func newCostumeWithSize(width, height int) *costume {
	c := &costume{
		setIndex:         -1,
		width:            width,
		height:           height,
		bitmapResolution: defaultBitmapResolution,
		posX:             0,
		posY:             0,
	}
	c.imageSize = mathf.NewVec2(float64(width), float64(height))
	c.center = mathf.NewVec2(float64(width)/2, float64(height)/2)
	c.atlasUVRect = mathf.NewVec4(0, 0, fullUVRange, fullUVRange)
	return c
}

// newCostumeWith creates a costume from a costume set image.
func newCostumeWith(name string, img *costumeSetImage, faceRight float64, frameIndex, bitmapResolution int) *costume {
	c := &costume{
		path:             img.path,
		name:             name,
		setIndex:         frameIndex,
		faceRight:        faceRight,
		bitmapResolution: bitmapResolution,
		posY:             0,
	}

	imageSize := resolveImageSize(img.width, img.height, img.path)
	c.imageSize = imageSize
	c.width = int(imageSize.X) / img.nx
	c.height = int(imageSize.Y)
	c.posX = frameIndex * c.width

	// Handle custom rect if specified
	if img.rc.H != 0 {
		c.width = int(img.rc.W) / img.nx
		c.height = int(img.rc.H)
		c.posX = int(img.rc.X) + frameIndex*c.width
		c.posY = int(img.rc.Y)
	}

	// Calculate atlas UV coordinates
	c.atlasUVRect = calculateAtlasUV(c.posX, c.posY, c.width, c.height, imageSize)
	c.center = mathf.NewVec2(float64(c.width)/2, float64(c.height)/2)

	return c
}

// newCostume creates a costume from a costume configuration.
func newCostume(base string, config *costumeConfig) *costume {
	fullPath := path.Join(base, config.Path)
	imageSize := resolveImageSize(config.ImageWidth, config.ImageHeight, fullPath)
	c := &costume{
		name:             config.Name,
		setIndex:         -1,
		center:           mathf.Vec2{X: config.X, Y: config.Y},
		faceRight:        config.FaceRight,
		bitmapResolution: toBitmapResolution(config.BitmapResolution),
		path:             fullPath,
		imageSize:        imageSize,
		width:            int(imageSize.X),
		height:           int(imageSize.Y),
		posX:             0,
		posY:             0,
		atlasUVRect:      mathf.NewVec4(0, 0, fullUVRange, fullUVRange),
	}
	return c
}

// calculateAtlasUV computes UV coordinates for an atlas region.
func calculateAtlasUV(posX, posY, width, height int, imageSize mathf.Vec2) mathf.Vec4 {
	uStart := float64(posX) / imageSize.X
	vStart := float64(posY) / imageSize.Y
	uSize := float64(width) / imageSize.X
	vSize := float64(height) / imageSize.Y
	return mathf.NewVec4(uStart, vStart, uSize, vSize)
}

// resolveImageSize returns the image size from configuration or loads it from file.
func resolveImageSize(cfgWidth, cfgHeight float64, imagePath string) mathf.Vec2 {
	if cfgWidth > 0 && cfgHeight > 0 {
		return mathf.Vec2{X: cfgWidth, Y: cfgHeight}
	}
	return getImageSizeCached(imagePath)
}

// getImageSizeCached retrieves image size from cache or loads it.
func getImageSizeCached(imagePath string) mathf.Vec2 {
	cache := imageSizeCacheRef()
	if v, ok := cache.Load(imagePath); ok {
		return v.(mathf.Vec2)
	}
	size := getCostumeAssetSize(imagePath)
	cache.Store(imagePath, size)
	return size
}

// getCostumeAssetSize loads the actual image size from the asset.
func getCostumeAssetSize(imagePath string) mathf.Vec2 {
	assetPath := engine.ToAssetPath(imagePath)
	if game, ok := engine.GetGame().(*Game); ok && game != nil {
		return game.engine().resMgr.GetImageSize(assetPath)
	}
	var engineMgr engineManagers
	return engineMgr.resMgr.GetImageSize(assetPath)
}

// getSize returns the size of the costume accounting for bitmap resolution.
func (c *costume) getSize() (int, int) {
	return c.width / c.bitmapResolution, c.height / c.bitmapResolution
}

// isAtlas returns true if this costume is part of an atlas/set.
func (c *costume) isAtlas() bool {
	return c.setIndex >= 0
}

// -------------------------------------------------------------------------------------
// Base object structure and core methods
// -------------------------------------------------------------------------------------

// baseObj provides common functionality for sprites and backdrops.
type baseObj struct {
	costumes     []*costume
	costumeIndex int

	// Rendering state
	syncSprite     *engine.Sprite // !!!All methods (except GetId()) can only be called on main thread
	scale          float64
	HasDestroyed   bool
	isCostumeSet   bool
	isCostumeDirty bool

	// Layer management
	layer        int
	isLayerDirty bool

	// Effects
	greffUniforms map[EffectKind]float64 // graphic effects uniforms
	hasShader     bool

	// Animation state
	isAnimating bool
}

// getSpriteId returns the unique identifier for this sprite.
func (p *baseObj) getSpriteId() engine.Object {
	return p.syncSprite.GetId()
}

// getProxy returns the underlying engine sprite.
func (p *baseObj) getProxy() *engine.Sprite {
	return p.syncSprite
}

// setLayer sets the layer/z-order of the object.
func (p *baseObj) setLayer(layer int) {
	if p.layer != layer {
		p.layer = layer
		p.isLayerDirty = true
	}
}

// setCostumeIndex sets the current costume by index.
func (p *baseObj) setCostumeIndex(value int) {
	p.costumeIndex = value
	p.isCostumeDirty = true
	p.isAnimating = false
}

// -------------------------------------------------------------------------------------
// Costume initialization methods
// -------------------------------------------------------------------------------------

// initWith initializes the base object with sprite configuration.
func (p *baseObj) initWith(base string, sprite *spriteConfig) {
	if sprite.CostumeSet != nil {
		initWithCS(p, base, sprite.CostumeSet)
	} else if sprite.CostumeMPSet != nil {
		initWithCMPS(p, base, sprite.CostumeMPSet)
	} else {
		panic("initWith: sprite configuration must have either CostumeSet or CostumeMPSet defined")
	}

	costumeIndex := sprite.getCostumeIndex()
	if costumeIndex >= len(p.costumes) || costumeIndex < 0 {
		costumeIndex = 0
	}
	p.setCostumeIndex(costumeIndex)
}

// initWithCMPS initializes with a multi-part costume set.
func initWithCMPS(p *baseObj, base string, cmps *costumeMPSet) {
	faceRight := cmps.FaceRight
	bitmapResolution := toBitmapResolution(cmps.BitmapResolution)
	imgPath := path.Join(base, cmps.Path)

	for _, cs := range cmps.Parts {
		img := &costumeSetImage{
			path: imgPath,
			rc:   cs.Rect,
			nx:   cs.Nx,
		}
		initCSPart(p, img, faceRight, bitmapResolution, cs.Nx, cs.Items)
	}
}

// initWithCS initializes with a costume set.
func initWithCS(p *baseObj, base string, cs *costumeSet) {
	nx := cs.Nx
	imgPath := path.Join(base, cs.Path)

	img := &costumeSetImage{
		path:   imgPath,
		width:  cs.ImageWidth,
		height: cs.ImageHeight,
		nx:     nx,
	}
	if cs.Rect != nil {
		img.rc = *cs.Rect
	}

	p.costumes = make([]*costume, 0, nx)
	initCSPart(p, img, cs.FaceRight, toBitmapResolution(cs.BitmapResolution), nx, cs.Items)
}

// initCSPart initializes a costume set part.
func initCSPart(p *baseObj, img *costumeSetImage, faceRight float64, bitmapResolution, nx int, items []costumeSetItem) {
	p.isCostumeSet = true
	if nx == 1 {
		name := strconv.Itoa(len(p.costumes))
		addCostumeWith(p, name, img, faceRight, 0, bitmapResolution)
		return
	}
	if items == nil {
		// Generate default names for each frame
		for index := range nx {
			name := strconv.Itoa(len(p.costumes))
			addCostumeWith(p, name, img, faceRight, index, bitmapResolution)
		}
		return
	}
	// Use provided item configurations
	frameIndex := 0
	for _, item := range items {
		for i := 0; i < item.N; i++ {
			name := item.NamePrefix + strconv.Itoa(i)
			addCostumeWith(p, name, img, faceRight, frameIndex, bitmapResolution)
			frameIndex++
		}
	}
	if frameIndex != nx {
		panic("initCostumeSetPart: incomplete costume set loading")
	}
}

// addCostumeWith adds a costume to the base object.
func addCostumeWith(p *baseObj, name SpriteCostumeName, img *costumeSetImage, faceRight float64, frameIndex, bitmapResolution int) {
	c := newCostumeWith(name, img, faceRight, frameIndex, bitmapResolution)
	p.costumes = append(p.costumes, c)
}

// initBackdrops initializes backdrops from configuration.
func (p *baseObj) initBackdrops(base string, configs []*backdropConfig, costumeIndex int) {
	p.costumes = make([]*costume, len(configs))
	for i, cfg := range configs {
		p.costumes[i] = newCostume(base, &cfg.costumeConfig)
	}
	if costumeIndex >= len(configs) || costumeIndex < 0 {
		costumeIndex = 0
	}
	p.setCostumeIndex(costumeIndex)
}

// init initializes costumes from configuration.
func (p *baseObj) init(base string, configs []*costumeConfig, costumeIndex int) {
	p.costumes = make([]*costume, len(configs))
	for i, cfg := range configs {
		p.costumes[i] = newCostume(base, cfg)
	}
	if costumeIndex >= len(configs) || costumeIndex < 0 {
		costumeIndex = 0
	}
	p.isLayerDirty = true
	p.setCostumeIndex(costumeIndex)
}

// initWithSize initializes with a single costume of the specified size.
func (p *baseObj) initWithSize(width, height int) {
	p.costumes = make([]*costume, 1)
	p.costumes[0] = newCostumeWithSize(width, height)
	p.setCostumeIndex(0)
}

// initFrom initializes from another base object (cloning).
func (p *baseObj) initFrom(src *baseObj) {
	p.costumes = src.costumes
	p.hasShader = false
	p.setCostumeIndex(src.costumeIndex)
}

// -------------------------------------------------------------------------------------
// Costume query and manipulation methods
// -------------------------------------------------------------------------------------

// findCostume finds a costume by name and returns its index.
func (p *baseObj) findCostume(name SpriteCostumeName) int {
	for i, c := range p.costumes {
		if c.name == name {
			return i
		}
	}
	return -1
}

// goSetCostume sets the costume from various input types.
func (p *baseObj) goSetCostume(val any) bool {
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

// setCostumeByIndex sets the costume by its index.
func (p *baseObj) setCostumeByIndex(idx int) bool {
	if idx < 0 || idx >= len(p.costumes) {
		panic("invalid costume index")
	}
	p.setCostumeIndex(idx)
	return true
}

// setCostumeByName sets the costume by its name.
func (p *baseObj) setCostumeByName(name SpriteCostumeName) bool {
	if idx := p.findCostume(name); idx >= 0 {
		return p.setCostumeByIndex(idx)
	}
	return false
}

// goPrevCostume switches to the previous costume (wraps around).
func (p *baseObj) goPrevCostume() {
	index := (len(p.costumes) + p.costumeIndex - 1) % len(p.costumes)
	p.setCostumeIndex(index)
}

// goNextCostume switches to the next costume (wraps around).
func (p *baseObj) goNextCostume() {
	index := (p.costumeIndex + 1) % len(p.costumes)
	p.setCostumeIndex(index)
}

// getCostumeIndex returns the current costume index.
func (p *baseObj) getCostumeIndex() int {
	return p.costumeIndex
}

// getCostumeName returns the name of the current costume.
func (p *baseObj) getCostumeName() SpriteCostumeName {
	return p.costumes[p.costumeIndex].name
}

// getCostumePath returns the file path of the current costume.
func (p *baseObj) getCostumePath() string {
	return p.costumes[p.costumeIndex].path
}

// getCostumeRenderScale returns the render scale for the current costume.
func (p *baseObj) getCostumeRenderScale() float64 {
	return p.scale / float64(p.getCurrentBitmapResolution())
}

// getAnimRenderScale returns the render scale for animation with given bitmap resolution.
func (p *baseObj) getAnimRenderScale(bitmapResolution int) float64 {
	return p.scale / float64(bitmapResolution)
}

// getCurrentBitmapResolution returns the bitmap resolution of the current costume.
func (p *baseObj) getCurrentBitmapResolution() int {
	return p.costumes[p.costumeIndex].bitmapResolution
}

// getCostumeSize returns the size of the current costume.
func (p *baseObj) getCostumeSize() (float64, float64) {
	x, y := p.costumes[p.costumeIndex].getSize()
	return float64(x), float64(y)
}

// isCostumeAtlas returns true if the current costume is part of an atlas.
func (p *baseObj) isCostumeAtlas() bool {
	return p.costumes[p.costumeIndex].isAtlas()
}

// getCostumeAtlasUvRemap returns the UV remap coordinates for the current costume.
func (p *baseObj) getCostumeAtlasUvRemap() mathf.Rect2 {
	costume := p.costumes[p.costumeIndex]
	return mathf.NewRect2(
		costume.atlasUVRect.X,
		costume.atlasUVRect.Y,
		costume.atlasUVRect.Z,
		costume.atlasUVRect.W,
	)
}

// getCostumeAtlasRegion returns the pixel region of the current costume in the atlas.
func (p *baseObj) getCostumeAtlasRegion() mathf.Rect2 {
	costume := p.costumes[p.costumeIndex]
	return mathf.NewRect2(
		float64(costume.posX),
		float64(costume.posY),
		float64(costume.width),
		float64(costume.height),
	)
}

// -------------------------------------------------------------------------------------
// Graphic effects methods
// -------------------------------------------------------------------------------------

// requireGreffUniforms ensures the graphic effects map is initialized.
func (p *baseObj) requireGreffUniforms() map[EffectKind]float64 {
	if p.greffUniforms == nil {
		p.greffUniforms = make(map[EffectKind]float64)
	}
	return p.greffUniforms
}

// setGraphicEffect sets a graphic effect to a specific value.
func (p *baseObj) setGraphicEffect(kind EffectKind, val float64) {
	effs := p.requireGreffUniforms()
	effs[kind] = val
	p.doSetGraphicEffect(kind, false)
}

// changeGraphicEffect changes a graphic effect by a delta value.
func (p *baseObj) changeGraphicEffect(kind EffectKind, delta float64) {
	effs := p.requireGreffUniforms()
	newVal := delta
	if oldVal, ok := effs[kind]; ok {
		newVal += oldVal
	}
	effs[kind] = newVal
	p.doSetGraphicEffect(kind, false)
}

// clearGraphicEffects resets all graphic effects to default values.
func (p *baseObj) clearGraphicEffects() {
	p.greffUniforms = nil
	effs := p.requireGreffUniforms()
	for i := range int(enumNumOfEffect) {
		effs[EffectKind(i)] = 0
	}
	p.applyGraphicEffects(false)
}

// applyGraphicEffects applies all graphic effects.
func (p *baseObj) applyGraphicEffects(isSync bool) {
	for i := range int(enumNumOfEffect) {
		p.doSetGraphicEffect(EffectKind(i), isSync)
	}
}

// doSetGraphicEffect applies a single graphic effect.
func (p *baseObj) doSetGraphicEffect(kind EffectKind, isSync bool) {
	if p.syncSprite == nil {
		return
	}

	effs := p.requireGreffUniforms()
	val, ok := effs[kind]
	if !ok {
		return
	}

	// Normalize effect values based on effect type
	normalizedVal := normalizeEffectValue(kind, val)
	p.setMaterialParams(kind.String(), normalizedVal, isSync)
}

// normalizeEffectValue normalizes an effect value based on its type.
func normalizeEffectValue(kind EffectKind, val float64) float64 {
	switch kind {
	case ColorEffect:
		normalized := math.Mod(val/200, 1)
		if normalized < 0 {
			normalized += 1
		}
		return normalized
	case BrightnessEffect:
		return mathf.Clamp(val/100, -1, 1)
	case GhostEffect:
		return mathf.Clamp01f(val / 100)
	case MosaicEffect:
		return math.Max(math.Floor((val+5)/10), 0)
	case WhirlEffect:
		return mathf.Clamp(val/50, -20, 20)
	case FishEyeEffect:
		return mathf.Clamp(val/100, -1, 100)
	case PixelateEffect:
		return mathf.Absf(val / 10)
	default:
		return val
	}
}

// -------------------------------------------------------------------------------------
// Material and shader methods
// -------------------------------------------------------------------------------------

// setMaterialParams sets a material parameter (scalar).
func (p *baseObj) setMaterialParams(effect string, amount float64, isSync bool) {
	if isSync {
		p.applyMaterialParams(effect, amount)
	} else {
		engine.WaitMainThread(func() {
			p.applyMaterialParams(effect, amount)
		})
	}
}

// setMaterialParamsVec4 sets a material parameter (vector).
func (p *baseObj) setMaterialParamsVec4(effect string, amount mathf.Vec4, isSync bool) {
	if isSync {
		p.applyMaterialParamsVec4(effect, amount)
	} else {
		engine.WaitMainThread(func() {
			p.applyMaterialParamsVec4(effect, amount)
		})
	}
}

// applyMaterialParams is the internal implementation for setting scalar material params.
func (p *baseObj) applyMaterialParams(effect string, amount float64) {
	if p.syncSprite == nil {
		return
	}
	if !p.hasShader {
		p.syncSprite.SetMaterialShader(shaderPath)
		p.hasShader = true
	}
	p.syncSprite.SetMaterialParams(effect, amount)
}

// applyMaterialParamsVec4 is the internal implementation for setting vector material params.
func (p *baseObj) applyMaterialParamsVec4(effect string, val mathf.Vec4) {
	if p.syncSprite == nil {
		return
	}
	if !p.hasShader {
		p.syncSprite.SetMaterialShader(shaderPath)
		p.hasShader = true
	}
	p.syncSprite.SetMaterialParamsVec(effect, val.X, val.Y, val.Z, val.W)
}
