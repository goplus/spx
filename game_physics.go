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
	"fmt"
	"slices"

	"github.com/goplus/spbase/mathf"
	spxlog "github.com/goplus/spx/v4/internal/base/log"
	coreproject "github.com/goplus/spx/v4/internal/core/project"
	"github.com/goplus/spx/v4/internal/engine"
)

const touchLayerCount = 32 // Godot trigger layer bit count.

type raycastResult struct {
	Hit      bool
	SpriteID int64
	PosX     float64
	PosY     float64
}

type touchFilter struct {
	layerIndex int
	Layer      int64
	Mask       int64
}

type touchEntry struct {
	sprite *SpriteImpl
	filter *touchFilter
}

func (p *Game) IntersectRect(posX, posY, width, height float64) []Sprite {
	ary := engine.Managers().PhysicsMgr.CheckCollisionRect(mathf.NewVec2(posX, posY), mathf.NewVec2(width, height), -1)
	return p.checkCollision(ary)
}

func (p *Game) IntersectCircle(posX, posY, radius float64) []Sprite {
	ary := engine.Managers().PhysicsMgr.CheckCollisionCircle(mathf.NewVec2(posX, posY), radius, -1)
	return p.checkCollision(ary)
}

func (p *Game) Raycast__0(fromX, fromY, toX, toY float64) (hit bool, sprite Sprite, hitX, hitY float64) {
	return p.Raycast__2(fromX, fromY, toX, toY, nil)
}

func (p *Game) Raycast__1(fromX, fromY, toX, toY float64, ignoreSprite Sprite) (hit bool, sprite Sprite, hitX, hitY float64) {
	return p.Raycast__2(fromX, fromY, toX, toY, []Sprite{ignoreSprite})
}

func (p *Game) Raycast__2(fromX, fromY, toX, toY float64, ignoreSprites []Sprite) (hit bool, sprite Sprite, hitX, hitY float64) {
	from := mathf.NewVec2(fromX, fromY)
	to := mathf.NewVec2(toX, toY)
	var ignoreSpritesIds []int64
	if len(ignoreSprites) > 0 {
		ignoreSpritesIds = make([]int64, 0, len(ignoreSprites))
		for _, item := range ignoreSprites {
			if item == nil {
				continue
			}
			impl := spriteOf(item)
			if impl != nil {
				ignoreSpritesIds = append(ignoreSpritesIds, impl.getSpriteId())
			}
		}
	}
	result := p.raycast(from, to, ignoreSpritesIds, -1)
	if result == nil {
		return false, nil, 0, 0
	}
	var target Sprite
	if result.Hit {
		sprite := engine.GetSprite(result.SpriteID)
		if sprite != nil {
			impl := sprite.Target.(*SpriteImpl)
			if impl != nil {
				target = impl.sprite
			}
		}
	}
	return result.Hit, target, result.PosX, result.PosY
}

func (p *Game) DebugDrawRect(posX, posY, width, height float64, color Color) {
	engine.Managers().DebugMgr.DebugDrawRect(mathf.NewVec2(posX, posY), mathf.NewVec2(width, height), toMathfColor(color))
}

func (p *Game) DebugDrawCircle(posX, posY, radius float64, color Color) {
	engine.Managers().DebugMgr.DebugDrawCircle(mathf.NewVec2(posX, posY), radius, toMathfColor(color))
}

func (p *Game) DebugDrawLine(fromX, fromY, toX, toY float64, color Color) {
	engine.Managers().DebugMgr.DebugDrawLine(mathf.NewVec2(fromX, fromY), mathf.NewVec2(toX, toY), toMathfColor(color))
}

func (p *Game) DebugDrawLines(points []float64, color Color) {
	if len(points) < 4 || len(points)%2 != 0 {
		return
	}

	for i := 0; i < len(points)-2; i += 2 {
		from := mathf.NewVec2(points[i], points[i+1])
		to := mathf.NewVec2(points[i+2], points[i+3])
		engine.Managers().DebugMgr.DebugDrawLine(from, to, toMathfColor(color))
	}
}

func (p *Game) applyPhysicsSettings(settings coreproject.SystemSettings) {
	p.autoTouchLayers = settings.AutoSetCollisionLayer // Legacy name for automatic touch filtering.
	p.touchByShape = settings.TouchByShape
	spxlog.Debug("TouchByShape: %v", settings.TouchByShape)
	spxlog.Debug("AutoTouchLayers: %v", p.autoTouchLayers)

	engine.Managers().SpriteMgr.SetPixelCollisionSamplingStep(settings.PixelCollisionPrecision)

	engine.Managers().PhysicsMgr.SetGlobalGravity(settings.GlobalGravity)
	engine.Managers().PhysicsMgr.SetGlobalAirDrag(settings.GlobalAirDrag)
	engine.Managers().PhysicsMgr.SetGlobalFriction(settings.GlobalFriction)
	engine.Managers().PhysicsMgr.SetCollisionSystemType(!settings.TouchByShape)

	p.touchFilters = nil
	p.touchEntries = nil
	if p.autoTouchLayers {
		p.touchFilters = p.buildTouchFilters()
	}
}

func (p *Game) setupTouchEntries(inits []Sprite) {
	if !p.autoTouchLayers {
		p.touchFilters = nil
		p.touchEntries = nil
		return
	}

	entries := make([]touchEntry, 0, len(inits))
	for _, ini := range inits {
		spr := spriteOf(ini)
		if spr == nil {
			continue
		}
		entries = append(entries, touchEntry{
			sprite: spr,
			filter: p.getTouchFilter(spr.name),
		})
	}
	p.touchEntries = entries
}

func (p *Game) refreshTouchFilters() {
	if !p.autoTouchLayers || len(p.touchEntries) == 0 {
		return
	}

	entries := p.touchEntries
	var masksByLayer [touchLayerCount]int64

	// Types sharing a layer must share its subscriptions.
	for _, entry := range entries {
		for target := range entry.sprite.physics().touchTargets {
			filter, ok := p.touchFilters[target]
			if !ok {
				engine.Panic("Unknown sprite " + target)
				return
			}
			masksByLayer[entry.filter.layerIndex] |= filter.Layer
		}
	}

	// Commit only after validation, before any proxy writes.
	for _, entry := range entries {
		entry.filter.Mask = masksByLayer[entry.filter.layerIndex]
		spxlog.Debug("Touch filter: name=%s, layer=%d, mask=%d", entry.sprite.name, entry.filter.Layer, entry.filter.Mask)
	}

	engine.WaitMainThread(func() {
		for _, entry := range entries {
			if proxy := entry.sprite.runtimeState.syncSprite; proxy != nil {
				entry.sprite.physics().syncTriggerFilters(proxy)
			}
		}
	})
}

func (p *Game) buildTouchFilters() map[string]*touchFilter {
	filters := make(map[string]*touchFilter, len(p.typs))
	for name := range p.typs {
		layerIndex := len(filters) % touchLayerCount
		filters[name] = &touchFilter{layerIndex: layerIndex, Layer: 1 << layerIndex}
	}
	return filters
}

func (p *Game) getTouchFilter(name string) *touchFilter {
	if filter, ok := p.touchFilters[name]; ok {
		return filter
	}
	engine.Panic("Unknown sprite " + name)
	return &touchFilter{}
}

func (p *Game) checkCollision(ary any) []Sprite {
	spriteIdAry := ary.([]engine.Object)
	sprites := make([]Sprite, 0, len(spriteIdAry))
	slices.Sort(spriteIdAry)
	for _, item := range spriteIdAry {
		sprite := engine.GetSprite(item)
		if sprite != nil {
			impl := sprite.Target.(*SpriteImpl)
			if impl != nil {
				sprites = append(sprites, impl.sprite)
			} else {
				spxlog.Warn("Collision object is not a Sprite: %v", item)
			}
		}
	}
	return sprites
}

func (p *Game) raycast(from, to mathf.Vec2, ignoreSprites []int64, mask int64) *raycastResult {
	ary := engine.Managers().PhysicsMgr.RaycastWithDetails(from, to, ignoreSprites, mask, true, true)
	result, err := tryRaycastResult(ary)
	if err != nil {
		spxlog.Warn("Raycast warn: %v", err)
	}
	return result
}

func tryRaycastResult(ary engine.Array) (*raycastResult, error) {
	dataAry, succ := ary.([]int64)
	if !succ {
		return nil, fmt.Errorf("array type error: expected []int64 but got %T", ary)
	}
	// The engine returns six fields, including the unused surface normal.
	if len(dataAry) != 6 {
		return nil, fmt.Errorf("array len error: expected 6 but got %d", len(dataAry))
	}
	return &raycastResult{
		Hit:      dataAry[0] != 0,
		SpriteID: dataAry[1],
		PosX:     engine.ConvertToFloat64(dataAry[2]),
		PosY:     engine.ConvertToFloat64(dataAry[3]),
	}, nil
}
