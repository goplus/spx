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
	"errors"
	"fmt"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v2/internal/engine"
	spxlog "github.com/goplus/spx/v2/internal/log"
)

const (
	physicsColliderNone    = 0x00
	physicsColliderAuto    = 0x01
	physicsColliderCircle  = 0x02
	physicsColliderRect    = 0x03
	physicsColliderCapsule = 0x04
	physicsColliderPolygon = 0x05
)

type rayCastResult struct {
	Hited    bool
	SpriteId int64
	PosX     float64
	PosY     float64
	NormalX  float64
	NormalY  float64
}

func (p *rayCastResult) ToArray() engine.Array {
	ary := make([]int64, 6)
	if p.Hited {
		ary[0] = 1
	}
	ary[1] = p.SpriteId
	ary[2] = engine.ConvertToInt64(p.PosX)
	ary[3] = engine.ConvertToInt64(p.PosY)
	ary[4] = engine.ConvertToInt64(p.NormalX)
	ary[5] = engine.ConvertToInt64(p.NormalY)
	return ary
}

func tryRaycastResult(ary engine.Array) (*rayCastResult, error) {
	dataAry, succ := ary.([]int64)
	if !succ {
		return nil, errors.New("array type error" + fmt.Sprintf("%v", ary))
	}
	p := &rayCastResult{}
	p.Hited = false
	if len(dataAry) != 6 {
		return nil, errors.New("array len error")
	}
	p.Hited = dataAry[0] != 0
	p.SpriteId = dataAry[1]
	p.PosX = engine.ConvertToFloat64(dataAry[2])
	p.PosY = engine.ConvertToFloat64(dataAry[3])
	p.NormalX = engine.ConvertToFloat64(dataAry[4])
	p.NormalY = engine.ConvertToFloat64(dataAry[5])
	return p, nil
}

func raycast(from, to mathf.Vec2, ignoreSprites []int64, mask int64) *rayCastResult {
	ary := physicsMgr.RaycastWithDetails(from, to, ignoreSprites, -1, true, true)
	result, err := tryRaycastResult(ary)
	if err != nil {
		spxlog.Warn("Raycast error: %v", err)
	}
	return result
}
