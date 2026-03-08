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

import "slices"

func (p *SpriteImpl) OnCloned__0(onCloned func(data any)) {
	p.hasOnCloned = true
	p.eventSinkMgr.addWhenCloned(eventSink{
		pthis: p,
		sink:  onCloned,
		cond: func(data any) bool {
			return data == p
		},
	})
}

func (p *SpriteImpl) OnCloned__1(onCloned func()) {
	p.OnCloned__0(func(any) {
		onCloned()
	})
}

func (p *SpriteImpl) fireTouchStart(obj *SpriteImpl) {
	if p.hasOnTouchStart {
		p.doWhenTouchStart(p, obj)
	}
}

func (p *SpriteImpl) addTouchStartHandler(onTouchStart func(Sprite)) {
	p.hasOnTouchStart = true
	p.eventSinkMgr.addWhenTouchStart(eventSink{
		pthis: p,
		sink:  onTouchStart,
		cond: func(data any) bool {
			return data == p
		},
	})
}

func (p *SpriteImpl) OnTouchStart__0(sprite SpriteName, onTouchStart func(Sprite)) {
	p.physics().addCollisionTarget(sprite)
	p.addTouchStartHandler(func(s Sprite) {
		impl := spriteOf(s)
		if impl != nil && impl.name == sprite {
			onTouchStart(s)
		}
	})
}

func (p *SpriteImpl) OnTouchStart__1(sprite SpriteName, onTouchStart func()) {
	p.OnTouchStart__0(sprite, func(Sprite) {
		onTouchStart()
	})
}

func (p *SpriteImpl) OnTouchStart__2(sprites []SpriteName, onTouchStart func(Sprite)) {
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

func (p *SpriteImpl) OnTouchStart__3(sprites []SpriteName, onTouchStart func()) {
	p.OnTouchStart__2(sprites, func(Sprite) {
		onTouchStart()
	})
}
