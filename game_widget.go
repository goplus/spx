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

type WidgetName = string

type Widget interface {
	GetName() WidgetName
	Visible() bool
	Show()
	Hide()

	Xpos() float64
	Ypos() float64
	SetXpos(x float64)
	SetYpos(y float64)
	SetXYpos(x float64, y float64)
	ChangeXpos(dx float64)
	ChangeYpos(dy float64)
	ChangeXYpos(dx float64, dy float64)

	Size() float64
	SetSize(size float64)
	ChangeSize(delta float64)
}

type ShapeGetter interface {
	getAllShapes() []Shape
}

// GetWidget returns the widget instance with given name. It panics if not found.
// Instead of being used directly, it is meant to be called by `XGot_Game_XGox_GetWidget` only.
// We extract `GetWidget` to keep `XGot_Game_XGox_GetWidget` simple, which simplifies work in ispx,
// see details in https://github.com/goplus/builder/issues/765#issuecomment-2313915805.
func GetWidget(sg ShapeGetter, name WidgetName) Widget {
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

// XGot_Game_XGox_GetWidget returns a pointer to the widget of type T with the given name.
// It panics if the widget is not found or if the type does not match.
func XGot_Game_XGox_GetWidget[T any](sg ShapeGetter, name WidgetName) *T {
	widget, ok := GetWidget(sg, name).(any).(*T)
	if !ok {
		panic("GetWidget: type mismatch - " + name)
	}
	return widget
}
