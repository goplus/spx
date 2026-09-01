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
	"github.com/goplus/spx/v3/internal/base/collision"
	coreproject "github.com/goplus/spx/v3/internal/core/project"
	"github.com/goplus/spx/v3/internal/engine"
	spxlog "github.com/goplus/spx/v3/internal/log"
)

// -----------------------------------------------------------------------------
// Detection
// -----------------------------------------------------------------------------
func (p *Game) IntersectRect(posX, posY, width, height float64) []Sprite {
	ary := p.engine().PhysicsMgr.CheckCollisionRect(mathf.NewVec2(posX, posY), mathf.NewVec2(width, height), -1)
	return p.checkCollision(ary)
}

func (p *Game) IntersectCircle(posX, posY, radius float64) []Sprite {
	ary := p.engine().PhysicsMgr.CheckCollisionCircle(mathf.NewVec2(posX, posY), radius, -1)
	return p.checkCollision(ary)
}

// -----------------------------------------------------------------------------
// Raycast
// -----------------------------------------------------------------------------
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
	ignoredSpriteIds := make(map[int64]struct{}, len(ignoreSprites))
	if len(ignoreSprites) > 0 {
		ignoreSpritesIds = make([]int64, 0, len(ignoreSprites))
		for _, item := range ignoreSprites {
			if item == nil {
				continue
			}
			impl := spriteOf(item)
			if impl != nil {
				id := impl.getSpriteId()
				if _, exists := ignoredSpriteIds[id]; !exists {
					ignoreSpritesIds = append(ignoreSpritesIds, id)
					ignoredSpriteIds[id] = struct{}{}
				}
			}
		}
	}
	for {
		result := p.raycast(from, to, ignoreSpritesIds, -1)
		if result == nil {
			return false, nil, 0, 0
		}
		if !result.Hited {
			return false, nil, result.PosX, result.PosY
		}

		proxy := engine.GetSprite(result.SpriteId)
		if proxy == nil {
			return true, nil, result.PosX, result.PosY
		}
		impl, ok := proxy.Target.(*SpriteImpl)
		if !ok || impl == nil {
			spxlog.Warn("Raycast object is not a Sprite: %v", result.SpriteId)
			return true, nil, result.PosX, result.PosY
		}
		if !impl.spriteState.IsProxyPublicationPending {
			return true, impl.sprite, result.PosX, result.PosY
		}

		// A clone under initialization already has an engine proxy, but it is not
		// part of the published world yet. Ignore that proxy and continue the same
		// ray so a published sprite behind it can still be selected.
		if _, exists := ignoredSpriteIds[result.SpriteId]; exists {
			spxlog.Warn("Raycast returned ignored pending Sprite: %v", result.SpriteId)
			return false, nil, 0, 0
		}
		ignoredSpriteIds[result.SpriteId] = struct{}{}
		ignoreSpritesIds = append(ignoreSpritesIds, result.SpriteId)
	}
}

// -----------------------------------------------------------------------------
// Debug Draw
// -----------------------------------------------------------------------------
func (p *Game) DebugDrawRect(posX, posY, width, height float64, color Color) {
	p.engine().DebugMgr.DebugDrawRect(mathf.NewVec2(posX, posY), mathf.NewVec2(width, height), toMathfColor(color))
}

func (p *Game) DebugDrawCircle(posX, posY, radius float64, color Color) {
	p.engine().DebugMgr.DebugDrawCircle(mathf.NewVec2(posX, posY), radius, toMathfColor(color))
}

func (p *Game) DebugDrawLine(fromX, fromY, toX, toY float64, color Color) {
	p.engine().DebugMgr.DebugDrawLine(mathf.NewVec2(fromX, fromY), mathf.NewVec2(toX, toY), toMathfColor(color))
}

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

// -----------------------------------------------------------------------------
// Types
// -----------------------------------------------------------------------------
const (
	physicsColliderNone    = collision.ColliderNone
	physicsColliderAuto    = collision.ColliderAuto
	physicsColliderCircle  = collision.ColliderCircle
	physicsColliderRect    = collision.ColliderRect
	physicsColliderCapsule = collision.ColliderCapsule
	physicsColliderPolygon = collision.ColliderPolygon
)

const maxCollisionLayerIdx = 32 // Engine limit: max 32 collision layers

type rayCastResult struct {
	Hited    bool
	SpriteId int64
	PosX     float64
	PosY     float64
	NormalX  float64
	NormalY  float64
}

type spriteCollisionInfo struct {
	Index int
	Layer int64
	Mask  int64
}

type spriteCollisionData struct {
	sprite *SpriteImpl
	info   *spriteCollisionInfo
	modIdx int
}

// -----------------------------------------------------------------------------
// Setup
// -----------------------------------------------------------------------------
func (p *Game) applyPhysicsSettings(settings coreproject.SystemSettings) {
	p.isCollisionByPixel = settings.CollisionByPixel
	p.isAutoSetCollisionLayer = settings.AutoSetCollisionLayer
	spxlog.Debug("IsCollisionByPixel: %v", p.isCollisionByPixel)
	spxlog.Debug("IsAutoSetCollisionLayer: %v", p.isAutoSetCollisionLayer)

	p.engine().SpriteMgr.SetPixelCollisionSamplingStep(settings.PixelCollisionPrecision)

	p.engine().PhysicsMgr.SetGlobalGravity(settings.GlobalGravity)
	p.engine().PhysicsMgr.SetGlobalAirDrag(settings.GlobalAirDrag)
	p.engine().PhysicsMgr.SetGlobalFriction(settings.GlobalFriction)
	p.engine().PhysicsMgr.SetCollisionSystemType(p.isCollisionByPixel)

	p.resetCollisionLayerState()
	if p.isAutoSetCollisionLayer {
		p.sprCollisionInfos = p.buildSpriteCollisionInfos()
	}
}

func (p *Game) resetCollisionLayerState() {
	p.sprCollisionInfos = nil
	p.sprCollisionData = nil
}

func (p *Game) buildSpriteCollisionInfos() map[string]*spriteCollisionInfo {
	infos := make(map[string]*spriteCollisionInfo, len(p.typs))
	idx := 0
	for name := range p.typs {
		modIdx := idx % maxCollisionLayerIdx
		infos[name] = &spriteCollisionInfo{Index: idx, Layer: 1 << modIdx}
		idx++
	}
	return infos
}

func (p *Game) setupCollisionData(inits []Sprite) {
	if !p.isAutoSetCollisionLayer {
		p.resetCollisionLayerState()
		return
	}

	p.sprCollisionData = p.buildSpriteCollisionData(inits)
}

func (p *Game) refreshCollisionLayers() {
	if !p.isAutoSetCollisionLayer {
		return
	}

	p.applyCollisionLayers(p.sprCollisionData)
}

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------
func (p *Game) getSpriteCollisionInfo(name string) *spriteCollisionInfo {
	if info, ok := p.sprCollisionInfos[name]; ok {
		return info
	}
	engine.Panic("Unknown sprite " + name)
	return &spriteCollisionInfo{}
}

func getCollisionLayerIndex(info *spriteCollisionInfo) int {
	return info.Index % maxCollisionLayerIdx
}

func (p *Game) buildSpriteCollisionData(inits []Sprite) []*spriteCollisionData {
	spriteData := make([]*spriteCollisionData, 0, len(inits))
	for _, ini := range inits {
		spr := spriteOf(ini)
		if spr == nil {
			continue
		}
		info := p.getSpriteCollisionInfo(spr.name)
		spriteData = append(spriteData, &spriteCollisionData{
			sprite: spr,
			info:   info,
			modIdx: getCollisionLayerIndex(info),
		})
	}
	return spriteData
}

func (p *Game) applyCollisionLayers(spriteData []*spriteCollisionData) {
	if len(spriteData) == 0 {
		return
	}

	maskMap := make([]int64, maxCollisionLayerIdx)

	for _, data := range spriteData {
		data.info.Mask = 0
		for target := range data.sprite.physics().getCollisionTargets() {
			targetInfo := p.getSpriteCollisionInfo(target)
			maskMap[data.modIdx] |= targetInfo.Layer
		}
	}

	for _, data := range spriteData {
		data.info.Mask = maskMap[data.modIdx]
		spxlog.Debug("Init sprite collision info: name=%s, layer=%d, mask=%d", data.sprite.name, data.info.Layer, data.info.Mask)
	}

	engine.WaitMainThread(func() {
		for _, data := range spriteData {
			data.sprite.applyPhysicsProxyConfig()
		}
	})
}

func (p *Game) checkCollision(ary any) []Sprite {
	spriteIdAry := ary.([]engine.Object)
	sprites := make([]Sprite, 0, len(spriteIdAry))
	slices.Sort(spriteIdAry)
	for _, item := range spriteIdAry {
		sprite := engine.GetSprite(item)
		if sprite != nil {
			impl, ok := sprite.Target.(*SpriteImpl)
			if ok && impl != nil {
				if impl.spriteState.IsProxyPublicationPending {
					continue
				}
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
