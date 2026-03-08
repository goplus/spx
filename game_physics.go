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
	"github.com/goplus/spx/v2/internal/base/collisionutil"
	coreproject "github.com/goplus/spx/v2/internal/core/project"
	"github.com/goplus/spx/v2/internal/engine"
	spxlog "github.com/goplus/spx/v2/internal/log"
)

// =============================================================================
// Public API - Physics Detection and Collision
// =============================================================================

// IntersectRect detects sprites intersecting with a rectangular area
func (p *Game) IntersectRect(posX, posY, width, height float64) []Sprite {
	ary := p.engine().PhysicsMgr.CheckCollisionRect(mathf.NewVec2(posX, posY), mathf.NewVec2(width, height), -1)
	return p.checkCollision(ary)
}

// IntersectCircle detects sprites intersecting with a circular area
func (p *Game) IntersectCircle(posX, posY, radius float64) []Sprite {
	ary := p.engine().PhysicsMgr.CheckCollisionCircle(mathf.NewVec2(posX, posY), radius, -1)
	return p.checkCollision(ary)
}

// =============================================================================
// Public API - Raycast Methods
// =============================================================================

// Raycast__0 performs a raycast, ignoring specified sprites
func (p *Game) Raycast__0(fromX, fromY, toX, toY float64, ignoreSprites []Sprite) (hit bool, sprite Sprite, hitX, hitY float64) {
	from := mathf.NewVec2(fromX, fromY)
	to := mathf.NewVec2(toX, toY)
	ignoreSpritesIds := make([]int64, 0)
	for _, item := range ignoreSprites {
		if item == nil {
			continue
		}
		impl := spriteOf(item)
		if impl != nil {
			ignoreSpritesIds = append(ignoreSpritesIds, impl.getSpriteId())
		}
	}
	result := p.raycast(from, to, ignoreSpritesIds, -1)
	if result == nil {
		return false, nil, 0, 0
	}
	var target Sprite = nil
	if result.Hited {
		sprite := engine.GetSprite(result.SpriteId)
		if sprite != nil {
			impl := sprite.Target.(*SpriteImpl)
			if impl != nil {
				target = impl.sprite
			}
		}
	}
	return result.Hited, target, result.PosX, result.PosY
}

// Raycast__1 performs a raycast, ignoring a single sprite
func (p *Game) Raycast__1(fromX, fromY, toX, toY float64, ignoreSprite Sprite) (hit bool, sprite Sprite, hitX, hitY float64) {
	return p.Raycast__0(fromX, fromY, toX, toY, []Sprite{ignoreSprite})
}

// Raycast__2 performs a raycast without ignoring any sprites
func (p *Game) Raycast__2(fromX, fromY, toX, toY float64) (hit bool, sprite Sprite, hitX, hitY float64) {
	return p.Raycast__0(fromX, fromY, toX, toY, []Sprite{})
}

// =============================================================================
// Public API - Debug Drawing
// =============================================================================

// DebugDrawRect draws a debug rectangle
func (p *Game) DebugDrawRect(posX, posY, width, height float64, color Color) {
	p.engine().DebugMgr.DebugDrawRect(mathf.NewVec2(posX, posY), mathf.NewVec2(width, height), toMathfColor(color))
}

// DebugDrawCircle draws a debug circle
func (p *Game) DebugDrawCircle(posX, posY, radius float64, color Color) {
	p.engine().DebugMgr.DebugDrawCircle(mathf.NewVec2(posX, posY), radius, toMathfColor(color))
}

// DebugDrawLine draws a debug line
func (p *Game) DebugDrawLine(fromX, fromY, toX, toY float64, color Color) {
	p.engine().DebugMgr.DebugDrawLine(mathf.NewVec2(fromX, fromY), mathf.NewVec2(toX, toY), toMathfColor(color))
}

// DebugDrawLines draws multiple debug lines
func (p *Game) DebugDrawLines(points []float64, color Color) {
	if len(points) < 4 || len(points)%2 != 0 {
		return
	}

	for i := 0; i < len(points)-2; i += 2 {
		from := mathf.NewVec2(points[i], points[i+1])
		to := mathf.NewVec2(points[i+2], points[i+3])
		p.engine().DebugMgr.DebugDrawLine(from, to, toMathfColor(color))
	}
}

// =============================================================================
// Private - Constants and Types
// =============================================================================

const (
	physicsColliderNone    = collisionutil.ColliderNone
	physicsColliderAuto    = collisionutil.ColliderAuto
	physicsColliderCircle  = collisionutil.ColliderCircle
	physicsColliderRect    = collisionutil.ColliderRect
	physicsColliderCapsule = collisionutil.ColliderCapsule
	physicsColliderPolygon = collisionutil.ColliderPolygon
)

const maxCollisionLayerIdx = 32 // Engine limit: max 32 collision layers

// rayCastResult represents the result of a raycast query.
type rayCastResult struct {
	Hited    bool
	SpriteId int64
	PosX     float64
	PosY     float64
	NormalX  float64
	NormalY  float64
}

// spriteCollisionInfo contains collision information for a sprite.
type spriteCollisionInfo struct {
	Index int
	Layer int64
	Mask  int64
}

// spriteCollisionData caches sprite collision information.
type spriteCollisionData struct {
	sprite *SpriteImpl
	info   *spriteCollisionInfo
	modIdx int
}

// =============================================================================
// Private - Physics Configuration
// =============================================================================

// applyPhysicsSettings initializes physics system configuration.
func (p *Game) applyPhysicsSettings(settings coreproject.SystemSettings) {
	p.isCollisionByPixel = settings.CollisionByPixel
	p.isAutoSetCollisionLayer = settings.AutoSetCollisionLayer
	spxlog.Debug("==> isCollisionByPixel: %v", p.isCollisionByPixel)
	spxlog.Debug("==> isAutoSetCollisionLayer: %v", p.isAutoSetCollisionLayer)

	// Set pixel collision sampling step based on configuration
	p.engine().SpriteMgr.SetPixelCollisionSamplingStep(settings.PixelCollisionPrecision)

	// Set global physics parameters
	p.engine().PhysicsMgr.SetGlobalGravity(settings.GlobalGravity)
	p.engine().PhysicsMgr.SetGlobalAirDrag(settings.GlobalAirDrag)
	p.engine().PhysicsMgr.SetGlobalFriction(settings.GlobalFriction)
	p.engine().PhysicsMgr.SetCollisionSystemType(p.isCollisionByPixel)
	if p.isAutoSetCollisionLayer {
		p.sprCollisionInfos = make(map[string]*spriteCollisionInfo)
		idx := 0
		for name := range p.typs {
			modIdx := idx % maxCollisionLayerIdx
			info := &spriteCollisionInfo{Index: idx, Layer: 1 << modIdx}
			p.sprCollisionInfos[name] = info
			idx++
		}
	}
}

// setupCollisionLayers configures collision layers for sprites.
func (p *Game) setupCollisionLayers(inits []Sprite) {
	if !p.isAutoSetCollisionLayer {
		return
	}

	spriteData := p.buildSpriteCollisionData(inits)
	maskMap := make([]int64, maxCollisionLayerIdx)

	// Gather collision masks
	for _, data := range spriteData {
		data.info.Mask = 0
		for target := range data.sprite.physics().getCollisionTargets() {
			targetInfo := p.getSpriteCollisionInfo(target)
			maskMap[data.modIdx] |= targetInfo.Layer
		}
	}

	// Apply collision masks
	for _, data := range spriteData {
		data.info.Mask = maskMap[data.modIdx]
		spxlog.Debug("init sprite collision info: name=%s, layer=%d, mask=%d", data.sprite.name, data.info.Layer, data.info.Mask)
	}

	// Recalculate physics info
	engine.WaitMainThread(func() {
		for _, data := range spriteData {
			syncInitSpritePhysicInfo(data.sprite, data.sprite.SyncSprite)
		}
	})
}

// =============================================================================
// Private - Helper Functions
// =============================================================================

// getSpriteCollisionInfo retrieves collision info for a sprite by name.
func (p *Game) getSpriteCollisionInfo(name string) *spriteCollisionInfo {
	if info, ok := p.sprCollisionInfos[name]; ok {
		return info
	}
	engine.Panic("Unknown sprite " + name)
	return &spriteCollisionInfo{}
}

// getCollisionLayerIndex calculates the collision layer index.
func getCollisionLayerIndex(info *spriteCollisionInfo) int {
	return info.Index % maxCollisionLayerIdx
}

// buildSpriteCollisionData builds collision data for sprites.
func (p *Game) buildSpriteCollisionData(inits []Sprite) []*spriteCollisionData {
	spriteData := make([]*spriteCollisionData, 0, len(inits))
	for _, ini := range inits {
		spr := spriteOf(ini)
		info := p.getSpriteCollisionInfo(spr.name)
		spriteData = append(spriteData, &spriteCollisionData{
			sprite: spr,
			info:   info,
			modIdx: getCollisionLayerIndex(info),
		})
	}
	return spriteData
}

// checkCollision checks collision and returns a list of sprites.
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

// tryRaycastResult attempts to parse raycast result from array.
func tryRaycastResult(ary engine.Array) (*rayCastResult, error) {
	dataAry, succ := ary.([]int64)
	if !succ {
		return nil, fmt.Errorf("array type error: expected []int64 but got %T", ary)
	}
	p := &rayCastResult{}
	if len(dataAry) != 6 {
		return nil, fmt.Errorf("array len error: expected 6 but got %d", len(dataAry))
	}
	p.Hited = dataAry[0] != 0
	p.SpriteId = dataAry[1]
	p.PosX = engine.ConvertToFloat64(dataAry[2])
	p.PosY = engine.ConvertToFloat64(dataAry[3])
	p.NormalX = engine.ConvertToFloat64(dataAry[4])
	p.NormalY = engine.ConvertToFloat64(dataAry[5])
	return p, nil
}

// raycast performs a raycast query.
func (p *Game) raycast(from, to mathf.Vec2, ignoreSprites []int64, mask int64) *rayCastResult {
	ary := p.engine().PhysicsMgr.RaycastWithDetails(from, to, ignoreSprites, mask, true, true)
	result, err := tryRaycastResult(ary)
	if err != nil {
		spxlog.Warn("Raycast warn: %v", err)
	}
	return result
}
