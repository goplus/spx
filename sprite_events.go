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

	coreevent "github.com/goplus/spx/v2/internal/core/event"
)

func (p *SpriteImpl) OnCloned__0(onCloned func()) {
	p.OnCloned__1(coreevent.Ignore1[any](onCloned))
}

func (p *SpriteImpl) OnCloned__1(onCloned func(data any)) {
	p.spriteState.HasOnCloned = true
	p.scriptEventRegistry.manager.AddCloned(coreevent.NewSink(p, onCloned, coreevent.MatchOwner(p)))
}

func (p *SpriteImpl) fireTouchStart(obj *SpriteImpl) {
	if p.spriteState.HasOnTouchStart {
		p.doWhenTouchStart(p, obj)
	}
}

func (p *SpriteImpl) addTouchStartHandler(onTouchStart func(Sprite)) {
	p.spriteState.HasOnTouchStart = true
	p.scriptEventRegistry.manager.AddTouchStart(coreevent.NewSink(p, onTouchStart, coreevent.MatchOwner(p)))
}

func (p *SpriteImpl) OnTouchStart__0(sprite SpriteName, onTouchStart func()) {
	p.OnTouchStart__1(sprite, coreevent.Ignore1[Sprite](onTouchStart))
}

func (p *SpriteImpl) OnTouchStart__1(sprite SpriteName, onTouchStart func(Sprite)) {
	p.physics().addCollisionTarget(sprite)
	p.addTouchStartHandler(func(s Sprite) {
		impl := spriteOf(s)
		if impl != nil && impl.name == sprite {
			onTouchStart(s)
		}
	})
}

func (p *SpriteImpl) OnTouchStart__2(sprites []SpriteName, onTouchStart func()) {
	p.OnTouchStart__3(sprites, coreevent.Ignore1[Sprite](onTouchStart))
}

func (p *SpriteImpl) OnTouchStart__3(sprites []SpriteName, onTouchStart func(Sprite)) {
	for _, sprite := range sprites {
		p.physics().addCollisionTarget(sprite)
	}
	p.addTouchStartHandler(func(s Sprite) {
		impl := spriteOf(s)
		if impl != nil && slices.Contains(sprites, impl.name) {
			onTouchStart(s)
		}
	})
}
