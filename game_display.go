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

import "github.com/goplus/spbase/mathf"

// SetWindowSize sets the window size to the specified width and height.
func (p *Game) SetWindowSize(width int64, height int64) {
	p.engine().PlatformMgr.SetWindowSize(width, height, false)
}

// EraseAll erases all pen drawings.
func (p *Game) EraseAll() {
	p.engine().PenMgr.DestroyAllPens()
}

func (p *Game) getWindowSize() mathf.Vec2 {
	x, y := p.windowSize()
	return mathf.NewVec2(float64(x), float64(y))
}

func (p *Game) windowSize() (int, int) {
	if p.windowWidth == 0 {
		p.doWindowSize()
	}
	return p.windowWidth, p.windowHeight
}

func (p *Game) doWindowSize() {
	if p.windowWidth == 0 {
		c := p.costumes[p.costumeIndex]
		p.windowWidth, p.windowHeight = c.getSize()
	}
}

func (p *Game) worldSize() (int, int) {
	if p.worldWidth == 0 {
		p.doWorldSize()
	}
	return p.worldWidth, p.worldHeight
}

func (p *Game) doWorldSize() {
	if p.worldWidth == 0 {
		c := p.costumes[p.costumeIndex]
		p.worldWidth, p.worldHeight = c.getSize()
	}
}
