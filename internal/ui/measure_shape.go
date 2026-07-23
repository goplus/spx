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

package ui

import (
	"strconv"
	"strings"

	"github.com/goplus/spbase/mathf"
	coreproject "github.com/goplus/spx/v3/internal/core/project"
)

type measureShape struct {
	panel *UiMeasure
}

func NewMeasureShape(v coreproject.StageShape) any {
	size := v["size"].(float64)
	scale := coreproject.ShapeValue(v, "scale", 1.0).(float64)
	text := strconv.FormatFloat(size, 'f', 1, 64)
	text = strings.TrimSuffix(text, ".0")
	heading := coreproject.ShapeValue(v, "heading", 0.0).(float64)
	color, err := mathf.NewColorAny(coreproject.ShapeValue(v, "color", 0.0))
	if err != nil {
		panic(err)
	}

	pos := mathf.NewVec2(v["x"].(float64), v["y"].(float64))
	panel := NewUiMeasure()
	panel.UpdateInfo(pos, size*scale, heading, text, color)
	return &measureShape{panel: panel}
}
