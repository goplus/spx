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

func NewMeasureShape(v coreproject.MeasureShape) any {
	text := strconv.FormatFloat(v.Size, 'f', 1, 64)
	text = strings.TrimSuffix(text, ".0")
	panel := NewUiMeasure()
	panel.UpdateInfo(mathf.NewVec2(v.X, v.Y), v.Size*v.Scale, v.Heading, text, v.Color)
	return &measureShape{panel: panel}
}
