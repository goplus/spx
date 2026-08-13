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

import "math"

func (p *SpriteImpl) getAllShapes() []Shape {
	return p.g.getAllShapes()
}

func (p *Game) addShape(child Shape) {
	p.shapeMgr.addShape(child)
}

func (p *Game) addClonedShape(src, clone Shape) {
	p.shapeMgr.addClonedShape(src, clone)
}

func (p *Game) removeShape(child Shape) {
	p.shapeMgr.removeShape(child)
}

func (p *Game) activateShape(child Shape) {
	p.shapeMgr.activateShape(child)
}

func (p *Game) findSprite(name SpriteName) *SpriteImpl {
	return p.shapeMgr.findSprite(name)
}

func (p *Game) getAllShapes() []Shape {
	return p.shapeMgr.all()
}

func (p *Game) getTempShapes() []Shape {
	return p.shapeMgr.getTempShapes()
}

func (p *Game) gotoFront(spr *SpriteImpl) {
	p.goBackLayers(spr, math.MinInt32)
}

func (p *Game) gotoBack(spr *SpriteImpl) {
	p.goBackLayers(spr, math.MaxInt32)
}

func (p *Game) goBackLayers(spr *SpriteImpl, n int) {
	p.shapeMgr.goBackLayers(spr, n)
}
