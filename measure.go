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
	"fmt"
	"strconv"
	"strings"

	"github.com/goplus/spx/internal/ui"
	"github.com/realdream-ai/mathf"
)

const (
	measureEdgeLen    = 6
	measureLineWidth  = 2
	measureTextMargin = 8
)

type measure struct {
	size    float64
	pos     mathf.Vec2
	heading float64

	// computed properties
	text         string
	color        mathf.Color
	svgLineStyle string
	svgRotate    string
	svgSize      int // size*scale + 0.5 + measureLineWidth
	panel        *ui.UiMeasure
}

func newMeasure(v specsp) *measure {
	size := v["size"].(float64)
	scale := getSpcspVal(v, "scale", 1.0).(float64)
	text := strconv.FormatFloat(size, 'f', 1, 64)
	text = strings.TrimSuffix(text, ".0")
	heading := getSpcspVal(v, "heading", 0.0).(float64)
	svgSize := int(size*scale + 0.5 + measureLineWidth)
	c, err := mathf.NewColorAny(getSpcspVal(v, "color", 0.0))
	if err != nil {
		panic(err)
	}
	pos := mathf.NewVec2(v["x"].(float64), v["y"].(float64))
	panel := ui.NewUiMeasure()
	meansureObj := &measure{
		heading:      heading,
		size:         size,
		text:         text,
		color:        c,
		pos:          pos,
		svgLineStyle: fmt.Sprintf("stroke-width:%d;stroke:rgb(%d, %d, %d);", measureLineWidth, c.R, c.G, c.B),
		svgRotate:    fmt.Sprintf("rotate(%.1f %d %d)", heading, svgSize>>1, svgSize>>1),
		svgSize:      svgSize,
		panel:        panel,
	}
	panel.UpdateInfo(meansureObj.pos, size*scale, heading, text, c)
	return meansureObj
}

func getSpcspVal(ss specsp, key string, defaultVal ...any) any {
	v, ok := ss[key]
	if ok {
		return v
	}
	if len(defaultVal) > 0 {
		return defaultVal[0]
	}
	return v
}
