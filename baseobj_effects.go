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

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v2/internal/engine"
)

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
	if p.SyncSprite == nil {
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
	if p.SyncSprite == nil {
		return
	}
	if !p.HasShader {
		p.SyncSprite.SetMaterialShader(shaderPath)
		p.HasShader = true
	}
	p.SyncSprite.SetMaterialParams(effect, amount)
}

// applyMaterialParamsVec4 is the internal implementation for setting vector material params.
func (p *baseObj) applyMaterialParamsVec4(effect string, val mathf.Vec4) {
	if p.SyncSprite == nil {
		return
	}
	if !p.HasShader {
		p.SyncSprite.SetMaterialShader(shaderPath)
		p.HasShader = true
	}
	p.SyncSprite.SetMaterialParamsVec(effect, val.X, val.Y, val.Z, val.W)
}
