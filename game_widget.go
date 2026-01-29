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
	"math"
)

// -----------------------------------------------------------------------------
// Widget Management

type ShapeGetter interface {
	getAllShapes() []Shape
}

// GetWidget_ returns the widget instance with given name. It panics if not found.
// Instead of being used directly, it is meant to be called by `XGot_Game_XGox_GetWidget` only.
// We extract `GetWidget_` to keep `XGot_Game_XGox_GetWidget` simple, which simplifies work in ispx,
// see details in https://github.com/goplus/builder/issues/765#issuecomment-2313915805.
func GetWidget_(sg ShapeGetter, name WidgetName) Widget {
	items := sg.getAllShapes()
	for _, item := range items {
		widget, ok := item.(Widget)
		if !ok {
			continue
		}
		if widget.GetName() == name {
			return widget
		}
	}
	panic("GetWidget: widget not found - " + name)
}

// GetWidget returns the widget instance (in given type) with given name. It panics if not found.
func XGot_Game_XGox_GetWidget[T any](sg ShapeGetter, name WidgetName) *T {
	widget, ok := GetWidget_(sg, name).(any).(*T)
	if !ok {
		panic("GetWidget: type mismatch - " + name)
	}
	return widget
}

// -----------------------------------------------------------------------------
// Layer Management

func (p *Game) gotoFront(spr *SpriteImpl) {
	p.goBackLayers(spr, math.MinInt32)
}

func (p *Game) gotoBack(spr *SpriteImpl) {
	p.goBackLayers(spr, math.MaxInt32)
}

func (p *Game) goBackLayers(spr *SpriteImpl, n int) {
	p.spriteMgr.goBackLayers(spr, n)
}
