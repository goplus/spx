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
	"sync/atomic"

	"github.com/goplus/spbase/mathf"
	spxlog "github.com/goplus/spx/v4/internal/base/log"
	coreproject "github.com/goplus/spx/v4/internal/core/project"
	"github.com/goplus/spx/v4/internal/engine"
)

// Visual Effects.
type EffectKind int

const (
	ColorEffect EffectKind = iota
	FishEyeEffect
	WhirlEffect
	PixelateEffect
	MosaicEffect
	BrightnessEffect
	GhostEffect

	enumNumOfEffect
)

var graphicEffectNames = []string{
	ColorEffect:      "color_amount",
	FishEyeEffect:    "fisheye_amount",
	WhirlEffect:      "whirl_amount",
	MosaicEffect:     "uv_amount",
	PixelateEffect:   "pixleate_amount",
	BrightnessEffect: "brightness_amount",
	GhostEffect:      "alpha_amount",
}

// visualObject provides common functionality for sprites and backdrops.
type visualObject struct {
	runtimeState visualState

	costumes     []*costume
	costumeIndex int

	graphicEffects map[EffectKind]float64 // Values before shader normalization.
}

type visualState struct {
	proxy          *engine.Sprite
	scale          float64
	isCostumeSet   bool
	isCostumeDirty bool
	layer          int
	isLayerDirty   bool
	hasShader      bool
	isAnimating    bool
	destroyed      atomic.Bool
}

const (
	shaderPath = "res://engine/shader/spx_sprite_shader.gdshader"
)

func (k EffectKind) String() string {
	return graphicEffectNames[k]
}

// init initializes costumes from configuration.
func (p *visualObject) init(configs []*coreproject.CostumeConfig, costumeIndex int) {
	p.costumes = make([]*costume, len(configs))
	for i, cfg := range configs {
		p.costumes[i] = newCostume(cfg)
	}
	if costumeIndex >= len(configs) || costumeIndex < 0 {
		costumeIndex = 0
	}
	p.runtimeState.isLayerDirty = true
	p.setCostumeIndex(costumeIndex)
}

// proxyID returns the unique identifier for this sprite.
func (p *visualObject) proxyID() engine.Object {
	return p.runtimeState.proxy.GetId()
}

// getProxy returns the underlying engine sprite.
func (p *visualObject) getProxy() *engine.Sprite {
	return p.runtimeState.proxy
}

// setLayer sets the layer/z-order of the object.
func (p *visualObject) setLayer(layer int) {
	if p.runtimeState.layer != layer {
		p.runtimeState.layer = layer
		p.runtimeState.isLayerDirty = true
	}
}

// setCostumeIndex sets the current costume by index.
func (p *visualObject) setCostumeIndex(value int) {
	p.costumeIndex = value
	p.runtimeState.isCostumeDirty = true
	p.runtimeState.isAnimating = false
}

// isDestroyed reports whether the object has been permanently destroyed.
// It is safe to call from the engine thread while scripts are still tearing down.
func (p *visualObject) isDestroyed() bool {
	return p.runtimeState.destroyed.Load()
}

// markDestroyed records that the object has been permanently destroyed.
// It is safe to call from script goroutines and read from the engine thread.
func (p *visualObject) markDestroyed() {
	p.runtimeState.destroyed.Store(true)
}

// initSpriteCostumes creates costumes from a layout prepared before any engine
// state is mutated.
func (p *visualObject) initSpriteCostumes(sprite *coreproject.SpriteConfig, layout *coreproject.CostumeLayout) {
	if sprite.Costumes != nil {
		p.init(sprite.Costumes, sprite.GetCostumeIndex())
		return
	}

	p.runtimeState.isCostumeSet = true
	p.costumes = make([]*costume, 0, len(layout.Frames))
	if sprite.CostumeSet != nil {
		cs := sprite.CostumeSet
		image := &costumeSetImage{
			path:       cs.Path,
			width:      cs.ImageWidth,
			height:     cs.ImageHeight,
			frameCount: cs.Nx,
		}
		if cs.Rect != nil {
			image.rect = *cs.Rect
		}
		p.addCostumeFrames(layout.Frames, []*costumeSetImage{image}, cs.FaceRight, cs.BitmapResolution)
	} else {
		cmps := sprite.CostumeMPSet
		images := make([]*costumeSetImage, len(cmps.Parts))
		for i, part := range cmps.Parts {
			images[i] = &costumeSetImage{path: cmps.Path, rect: part.Rect, frameCount: part.Nx}
		}
		p.addCostumeFrames(layout.Frames, images, cmps.FaceRight, cmps.BitmapResolution)
	}

	costumeIndex := sprite.GetCostumeIndex()
	if costumeIndex < 0 || costumeIndex >= len(p.costumes) {
		costumeIndex = 0
	}
	p.setCostumeIndex(costumeIndex)
}

func (p *visualObject) addCostumeFrames(frames []coreproject.CostumeFrame, images []*costumeSetImage, faceRight float64, bitmapResolution int) {
	resolution := normalizeBitmapResolution(bitmapResolution)
	for _, frame := range frames {
		c := newCostumeFromSet(frame.Name, images[frame.Part], faceRight, frame.Index, resolution)
		p.costumes = append(p.costumes, c)
	}
}

// initBackdrops initializes backdrops from configuration.
func (p *visualObject) initBackdrops(configs []*coreproject.BackdropConfig, costumeIndex int) {
	p.costumes = make([]*costume, len(configs))
	for i, cfg := range configs {
		p.costumes[i] = newBackdropCostume(cfg)
	}
	if costumeIndex >= len(configs) || costumeIndex < 0 {
		costumeIndex = 0
	}
	p.setCostumeIndex(costumeIndex)
}

// initWithSize initializes with a single costume of the specified size.
func (p *visualObject) initWithSize(width, height int) {
	p.costumes = make([]*costume, 1)
	p.costumes[0] = newCostumeWithSize(width, height)
	p.setCostumeIndex(0)
}

// initFrom initializes from another base object (cloning).
func (p *visualObject) initFrom(src *visualObject) {
	p.costumes = src.costumes
	p.runtimeState.hasShader = false
	p.setCostumeIndex(src.costumeIndex)
}

// findCostume finds a costume by name and returns its index.
func (p *visualObject) findCostume(name SpriteCostumeName) int {
	for i, c := range p.costumes {
		if c.name == name {
			return i
		}
	}
	return -1
}

// goSetCostume sets the costume from various input types.
func (p *visualObject) goSetCostume(val any) bool {
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
		spxlog.Error("setCostume: invalid argument type: %T", val)
		return false
	}
}

// setCostumeByIndex sets the costume by its index.
func (p *visualObject) setCostumeByIndex(idx int) bool {
	if idx < 0 || idx >= len(p.costumes) {
		spxlog.Error("setCostume: invalid index %d (count: %d)", idx, len(p.costumes))
		return false
	}
	p.setCostumeIndex(idx)
	return true
}

// setCostumeByName sets the costume by its name.
func (p *visualObject) setCostumeByName(name SpriteCostumeName) bool {
	if idx := p.findCostume(name); idx >= 0 {
		return p.setCostumeByIndex(idx)
	}
	spxlog.Error("setCostume: invalid name %s", name)
	return false
}

// goPrevCostume switches to the previous costume (wraps around).
func (p *visualObject) goPrevCostume() {
	index := (len(p.costumes) + p.costumeIndex - 1) % len(p.costumes)
	p.setCostumeIndex(index)
}

// goNextCostume switches to the next costume (wraps around).
func (p *visualObject) goNextCostume() {
	index := (p.costumeIndex + 1) % len(p.costumes)
	p.setCostumeIndex(index)
}

// getCostumeIndex returns the current costume index.
func (p *visualObject) getCostumeIndex() int {
	return p.costumeIndex
}

// currentCostume returns the currently selected costume.
func (p *visualObject) currentCostume() *costume {
	return p.costumes[p.costumeIndex]
}

// getCostumeName returns the name of the current costume.
func (p *visualObject) getCostumeName() SpriteCostumeName {
	return p.currentCostume().name
}

// getCostumePath returns the file path of the current costume.
func (p *visualObject) getCostumePath() string {
	return p.currentCostume().path
}

// getCostumeRenderScale returns the render scale for the current costume.
func (p *visualObject) getCostumeRenderScale() float64 {
	return p.renderScale(p.currentCostume().frame.BitmapResolution)
}

// renderScale adjusts the object scale for bitmap resolution.
func (p *visualObject) renderScale(bitmapResolution int) float64 {
	return p.runtimeState.scale / float64(bitmapResolution)
}

// getCostumeSize returns the size of the current costume.
func (p *visualObject) getCostumeSize() (float64, float64) {
	return p.currentCostume().sizeInSPX()
}

// isCostumeAtlas returns true if the current costume is part of an atlas.
func (p *visualObject) isCostumeAtlas() bool {
	return p.currentCostume().isAtlas()
}

// getCostumeAtlasRegion returns the pixel region of the current costume in the atlas.
func (p *visualObject) getCostumeAtlasRegion() mathf.Rect2 {
	frame := p.currentCostume().frame
	return mathf.NewRect2(
		float64(frame.PosX),
		float64(frame.PosY),
		float64(frame.Width),
		float64(frame.Height),
	)
}

// ensureGraphicEffects ensures the graphic effects map is initialized.
func (p *visualObject) ensureGraphicEffects() map[EffectKind]float64 {
	if p.graphicEffects == nil {
		p.graphicEffects = make(map[EffectKind]float64)
	}
	return p.graphicEffects
}

// setGraphicEffect sets a graphic effect to a specific value.
func (p *visualObject) setGraphicEffect(kind EffectKind, val float64) {
	effs := p.ensureGraphicEffects()
	effs[kind] = clampEffectValue(kind, val)
	p.doSetGraphicEffect(kind, false)
}

// changeGraphicEffect changes a graphic effect by a delta value.
func (p *visualObject) changeGraphicEffect(kind EffectKind, delta float64) {
	effs := p.ensureGraphicEffects()
	newVal := delta
	if oldVal, ok := effs[kind]; ok {
		newVal += oldVal
	}
	effs[kind] = clampEffectValue(kind, newVal)
	p.doSetGraphicEffect(kind, false)
}

// clearGraphicEffects resets all graphic effects to default values.
func (p *visualObject) clearGraphicEffects() {
	p.graphicEffects = nil
	effs := p.ensureGraphicEffects()
	for i := range int(enumNumOfEffect) {
		effs[EffectKind(i)] = 0
	}
	p.applyGraphicEffects(false)
}

// applyGraphicEffects applies all graphic effects.
func (p *visualObject) applyGraphicEffects(isSync bool) {
	for i := range int(enumNumOfEffect) {
		p.doSetGraphicEffect(EffectKind(i), isSync)
	}
}

// doSetGraphicEffect applies a single graphic effect.
func (p *visualObject) doSetGraphicEffect(kind EffectKind, isSync bool) {
	if p.runtimeState.proxy == nil {
		return
	}

	effs := p.ensureGraphicEffects()
	val, ok := effs[kind]
	if !ok {
		return
	}

	normalizedVal := normalizeEffectValue(kind, val)
	p.setMaterialParams(kind.String(), normalizedVal, isSync)
}

// setMaterialParams sets a material parameter (scalar).
func (p *visualObject) setMaterialParams(effect string, amount float64, isSync bool) {
	if isSync {
		p.applyMaterialParams(effect, amount)
	} else {
		engine.WaitMainThread(func() {
			p.applyMaterialParams(effect, amount)
		})
	}
}

// setMaterialParamsVec4 sets a material parameter (vector).
func (p *visualObject) setMaterialParamsVec4(effect string, amount mathf.Vec4, isSync bool) {
	if isSync {
		p.applyMaterialParamsVec4(effect, amount)
	} else {
		engine.WaitMainThread(func() {
			p.applyMaterialParamsVec4(effect, amount)
		})
	}
}

// applyMaterialParams is the internal implementation for setting scalar material params.
func (p *visualObject) applyMaterialParams(effect string, amount float64) {
	if !p.ensureMaterialShader() {
		return
	}
	p.runtimeState.proxy.SetMaterialParams(effect, amount)
}

// applyMaterialParamsVec4 is the internal implementation for setting vector material params.
func (p *visualObject) applyMaterialParamsVec4(effect string, val mathf.Vec4) {
	if !p.ensureMaterialShader() {
		return
	}
	p.runtimeState.proxy.SetMaterialParamsVec(effect, val.X, val.Y, val.Z, val.W)
}

func (p *visualObject) ensureMaterialShader() bool {
	if p.runtimeState.proxy == nil {
		return false
	}
	if !p.runtimeState.hasShader {
		p.runtimeState.proxy.SetMaterialShader(shaderPath)
		p.runtimeState.hasShader = true
	}
	return true
}

// clampEffectValue limits stored effects so later changes start at Scratch's bounds.
func clampEffectValue(kind EffectKind, val float64) float64 {
	switch kind {
	case BrightnessEffect:
		return mathf.Clamp(val, -100, 100)
	case GhostEffect:
		return mathf.Clamp(val, 0, 100)
	default:
		return val
	}
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
