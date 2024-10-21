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

import "github.com/goplus/spx/internal/engine"

type Shape interface {
}

// -------------------------------------------------------------------------------------

func (p *SpriteImpl) touchPoint(x, y float64) bool {
	return engine.SyncSpriteCheckCollisionWithPoint(p.proxy.GetId(), engine.NewVec2(x, y), true)
}

func (p *SpriteImpl) touchingSprite(dst *SpriteImpl) bool {
	return engine.SyncSpriteCheckCollision(p.proxy.GetId(), dst.proxy.GetId(), true, true)
}
