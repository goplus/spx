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
	"strconv"

	"github.com/goplus/spbase/mathf"
	assetutil "github.com/goplus/spx/v2/internal/assets"
	corestate "github.com/goplus/spx/v2/internal/core/state"
	"github.com/goplus/spx/v2/internal/engine"
)

// baseObj provides common functionality for sprites and backdrops.
type baseObj struct {
	runtimeState corestate.BaseObjRuntimeState

	costumes     []*costume
	costumeIndex int

	// Effects
	greffUniforms map[EffectKind]float64 // graphic effects uniforms
}

// getSpriteId returns the unique identifier for this sprite.
func (p *baseObj) getSpriteId() engine.Object {
	return p.runtimeState.SyncSprite.GetId()
}

// getProxy returns the underlying engine sprite.
func (p *baseObj) getProxy() *engine.Sprite {
	return p.runtimeState.SyncSprite
}

// setLayer sets the layer/z-order of the object.
func (p *baseObj) setLayer(layer int) {
	if p.runtimeState.Layer != layer {
		p.runtimeState.Layer = layer
		p.runtimeState.IsLayerDirty = true
	}
}

// setCostumeIndex sets the current costume by index.
func (p *baseObj) setCostumeIndex(value int) {
	p.costumeIndex = value
	p.runtimeState.IsCostumeDirty = true
	p.runtimeState.IsAnimating = false
}

// isDestroyed reports whether the object has been permanently destroyed.
// It is safe to call from the engine thread while scripts are still tearing down.
func (p *baseObj) isDestroyed() bool {
	return p.runtimeState.IsDestroyed()
}

// markDestroyed records that the object has been permanently destroyed.
// It is safe to call from script goroutines and read from the engine thread.
func (p *baseObj) markDestroyed() {
	p.runtimeState.MarkDestroyed()
}

// initWith initializes the base object with sprite configuration.
func (p *baseObj) initWith(base string, sprite *spriteConfig) {
	if sprite.CostumeSet != nil {
		initWithCS(p, base, sprite.CostumeSet)
	} else if sprite.CostumeMPSet != nil {
		initWithCMPS(p, base, sprite.CostumeMPSet)
	} else {
		engine.Panic("initWith: sprite configuration must have either CostumeSet or CostumeMPSet defined")
		return
	}

	costumeIndex := sprite.GetCostumeIndex()
	if costumeIndex >= len(p.costumes) || costumeIndex < 0 {
		costumeIndex = 0
	}
	p.setCostumeIndex(costumeIndex)
}

// initWithCMPS initializes with a multi-part costume set.
func initWithCMPS(p *baseObj, base string, cmps *costumeMPSet) {
	faceRight := cmps.FaceRight
	bitmapResolution := assetutil.ToBitmapResolution(cmps.BitmapResolution)
	imgPath := cmps.Path

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
	imgPath := cs.Path

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
	initCSPart(p, img, cs.FaceRight, assetutil.ToBitmapResolution(cs.BitmapResolution), nx, cs.Items)
}

// initCSPart initializes a costume set part.
func initCSPart(p *baseObj, img *costumeSetImage, faceRight float64, bitmapResolution, nx int, items []costumeSetItem) {
	p.runtimeState.IsCostumeSet = true
	if nx <= 0 {
		engine.Panicf("initCSPart: invalid costume set frame count %d", nx)
		return
	}
	if nx == 1 {
		name := strconv.Itoa(len(p.costumes))
		addCostumeWith(p, name, img, faceRight, 0, bitmapResolution)
		return
	}
	if items == nil {
		for index := range nx {
			name := strconv.Itoa(len(p.costumes))
			addCostumeWith(p, name, img, faceRight, index, bitmapResolution)
		}
		return
	}
	frameIndex := 0
	for _, item := range items {
		for i := 0; i < item.N; i++ {
			name := item.NamePrefix + strconv.Itoa(i)
			addCostumeWith(p, name, img, faceRight, frameIndex, bitmapResolution)
			frameIndex++
		}
	}
	if frameIndex != nx {
		engine.Panicf("initCSPart: incomplete costume set loading (loaded=%d, expected=%d)", frameIndex, nx)
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
		p.costumes[i] = newCostume(base, &cfg.CostumeConfig)
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
	p.runtimeState.IsLayerDirty = true
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
	p.runtimeState.HasShader = false
	p.setCostumeIndex(src.costumeIndex)
}

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
		engine.Panic("setCostume: invalid argument type")
		return false
	}
}

// setCostumeByIndex sets the costume by its index.
func (p *baseObj) setCostumeByIndex(idx int) bool {
	if idx < 0 || idx >= len(p.costumes) {
		engine.Panicf("invalid costume index: %d (count: %d)", idx, len(p.costumes))
		return false
	}
	p.setCostumeIndex(idx)
	return true
}

// setCostumeByName sets the costume by its name.
func (p *baseObj) setCostumeByName(name SpriteCostumeName) bool {
	if idx := p.findCostume(name); idx >= 0 {
		return p.setCostumeByIndex(idx)
	}
	engine.Panicf("invalid costume name: %s", name)
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
	return p.runtimeState.Scale / float64(p.getCurrentBitmapResolution())
}

// getAnimRenderScale returns the render scale for animation with given bitmap resolution.
func (p *baseObj) getAnimRenderScale(bitmapResolution int) float64 {
	return p.runtimeState.Scale / float64(bitmapResolution)
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

const (
	shaderPath = "res://engine/shader/spx_sprite_shader.gdshader"
)

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
	if p.runtimeState.SyncSprite == nil {
		return
	}

	effs := p.requireGreffUniforms()
	val, ok := effs[kind]
	if !ok {
		return
	}

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
		return mathf.Clamp(val/100, -1, 1)
	case PixelateEffect:
		return mathf.Absf(val / 10)
	default:
		return val
	}
}

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
	if p.runtimeState.SyncSprite == nil {
		return
	}
	if !p.runtimeState.HasShader {
		p.runtimeState.SyncSprite.SetMaterialShader(shaderPath)
		p.runtimeState.HasShader = true
	}
	p.runtimeState.SyncSprite.SetMaterialParams(effect, amount)
}

// applyMaterialParamsVec4 is the internal implementation for setting vector material params.
func (p *baseObj) applyMaterialParamsVec4(effect string, val mathf.Vec4) {
	if p.runtimeState.SyncSprite == nil {
		return
	}
	if !p.runtimeState.HasShader {
		p.runtimeState.SyncSprite.SetMaterialShader(shaderPath)
		p.runtimeState.HasShader = true
	}
	p.runtimeState.SyncSprite.SetMaterialParamsVec(effect, val.X, val.Y, val.Z, val.W)
}
