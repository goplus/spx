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

import (
	"image/color"
)

// -------------------------------------------------------------------------------------

type penLine struct {
	x1, y1 int
	x2, y2 int
	clr    color.RGBA
	width  int
}

// -------------------------------------------------------------------------------------

type turtleCanvas struct {
	objs []interface{}
}

func (p *turtleCanvas) eraseAll() {
	p.objs = nil
}

func (p *turtleCanvas) penLine(obj *penLine) {
	p.objs = append(p.objs, obj)
}

func (p *turtleCanvas) stampCostume(obj *SpriteImpl) {
	p.objs = append(p.objs, obj)
}

// -------------------------------------------------------------------------------------
