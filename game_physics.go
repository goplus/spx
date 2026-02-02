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
	"slices"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v2/internal/engine"
	spxlog "github.com/goplus/spx/v2/internal/log"
)

// -----------------------------------------------------------------------------
// Sprite Collision Info

const maxCollisionLayerIdx = 32 // engine limit support 32 layers

type spriteCollisionInfo struct {
	Id    int
	Layer int64
	Mask  int64
}

// spriteCollisionData caches sprite collision information
type spriteCollisionData struct {
	sprite *SpriteImpl
	info   *spriteCollisionInfo
	modIdx int
}

func (p *Game) getSpriteCollisionInfo(name string) *spriteCollisionInfo {
	if info, ok := p.sprCollisionInfos[name]; ok {
		return info
	}
	engine.Panic("Unknown sprite " + name)
	return &spriteCollisionInfo{}
}

func getCollisionLayerIndex(info *spriteCollisionInfo) int {
	return info.Id % maxCollisionLayerIdx
}

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
			syncInitSpritePhysicInfo(data.sprite, data.sprite.syncSprite)
		}
	})
}

// -----------------------------------------------------------------------------
// Physics Configuration

func (p *Game) setupPhysicsConfig(proj *projConfig) {
	p.isCollisionByPixel = !proj.CollisionByShape && !proj.Physics
	p.isAutoSetCollisionLayer = proj.AutoSetCollisionLayer == nil || *proj.AutoSetCollisionLayer
	spxlog.Debug("==> isCollisionByPixel: %v", p.isCollisionByPixel)
	spxlog.Debug("==> isAutoSetCollisionLayer: %v", p.isAutoSetCollisionLayer)

	// Set pixel collision sampling step based on configuration
	precision := parsePixelCollisionPrecision(proj.PixelCollisionPrecision)
	spriteMgr.SetPixelCollisionSamplingStep(int64(precision))

	// Set global physics parameters
	physicsMgr.SetGlobalGravity(parseDefaultValue(proj.GlobalGravity, 1))
	physicsMgr.SetGlobalAirDrag(parseDefaultValue(proj.GlobalAirDrag, 1))
	physicsMgr.SetGlobalFriction(parseDefaultValue(proj.GlobalFriction, 1))
	physicsMgr.SetCollisionSystemType(p.isCollisionByPixel)
	if p.isAutoSetCollisionLayer {
		p.sprCollisionInfos = make(map[string]*spriteCollisionInfo)
		idx := 0
		for name := range p.typs {
			modIdx := idx % maxCollisionLayerIdx
			info := &spriteCollisionInfo{Id: idx, Layer: 1 << modIdx}
			p.sprCollisionInfos[name] = info
			idx++
		}
	}
}

// -----------------------------------------------------------------------------
// Physics Detection and Collision

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

func (p *Game) IntersectRect(posX, posY, width, height float64) []Sprite {
	ary := physicsMgr.CheckCollisionRect(mathf.NewVec2(posX, posY), mathf.NewVec2(width, height), -1)
	return p.checkCollision(ary)
}

func (p *Game) IntersectCircle(posX, posY, radius float64) []Sprite {
	ary := physicsMgr.CheckCollisionCircle(mathf.NewVec2(posX, posY), radius, -1)
	return p.checkCollision(ary)
}

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
	result := raycast(from, to, ignoreSpritesIds, -1)
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

func (p *Game) Raycast__1(fromX, fromY, toX, toY float64, ignoreSprite Sprite) (hit bool, sprite Sprite, hitX, hitY float64) {
	return p.Raycast__0(fromX, fromY, toX, toY, []Sprite{ignoreSprite})
}

func (p *Game) Raycast__2(fromX, fromY, toX, toY float64) (hit bool, sprite Sprite, hitX, hitY float64) {
	return p.Raycast__0(fromX, fromY, toX, toY, []Sprite{})
}

// -----------------------------------------------------------------------------
// Debug Drawing

func (p *Game) DebugDrawRect(posX, posY, width, height float64, color Color) {
	debugMgr.DebugDrawRect(mathf.NewVec2(posX, posY), mathf.NewVec2(width, height), toMathfColor(color))
}

func (p *Game) DebugDrawCircle(posX, posY, radius float64, color Color) {
	debugMgr.DebugDrawCircle(mathf.NewVec2(posX, posY), radius, toMathfColor(color))
}

func (p *Game) DebugDrawLine(fromX, fromY, toX, toY float64, color Color) {
	debugMgr.DebugDrawLine(mathf.NewVec2(fromX, fromY), mathf.NewVec2(toX, toY), toMathfColor(color))
}

func (p *Game) DebugDrawLines(points []float64, color Color) {
	if len(points) < 4 || len(points)%2 != 0 {
		return
	}

	for i := 0; i < len(points)-2; i += 2 {
		from := mathf.NewVec2(points[i], points[i+1])
		to := mathf.NewVec2(points[i+2], points[i+3])
		debugMgr.DebugDrawLine(from, to, toMathfColor(color))
	}
}
