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

package project

import (
	"strings"
	"testing"

	"github.com/goplus/spbase/mathf"
)

func TestParseMonitorShape(t *testing.T) {
	shape := StageShape{
		"target": "", "val": "score", "name": "score", "label": "Score",
		"mode": "slider", "x": 12.0, "y": -5.0, "visible": true,
		"style": "scratch", "size": "2.5",
	}
	got, err := ParseMonitorShape(shape)
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode != MonitorModeSlider || got.Size != 2.5 || got.Style != "scratch" || got.X != 12 || got.Y != -5 || !got.Visible {
		t.Fatalf("parsed monitor = %+v", got)
	}

	shape["size"] = "invalid"
	got, err = ParseMonitorShape(shape)
	if err != nil || got.Size != 0 {
		t.Fatalf("invalid monitor size changed its fallback: %+v, %v", got, err)
	}

	for _, test := range []struct {
		field string
		value any
		want  string
	}{
		{"label", nil, `field "label" has type`},
		{"mode", []any{"slider"}, `field "mode" has type`},
		{"x", "left", `field "x" has type`},
		{"visible", 1.0, `field "visible" has type`},
	} {
		t.Run(test.field, func(t *testing.T) {
			bad := make(StageShape, len(shape))
			for key, value := range shape {
				bad[key] = value
			}
			bad[test.field] = test.value
			if _, err := ParseMonitorShape(bad); err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestParseMeasureShape(t *testing.T) {
	shape := StageShape{"size": 10.0, "x": 2.0, "y": -3.0}
	got, err := ParseMeasureShape(shape)
	if err != nil {
		t.Fatal(err)
	}
	black, _ := mathf.NewColorAny(0.0)
	if got.Size != 10 || got.Scale != 1 || got.Heading != 0 || got.X != 2 || got.Y != -3 || got.Color != black {
		t.Fatalf("parsed measure = %+v", got)
	}

	shape["scale"] = 2.0
	shape["heading"] = 90.0
	shape["color"] = "white"
	got, err = ParseMeasureShape(shape)
	if err != nil || got.Scale != 2 || got.Heading != 90 || got.Color != (mathf.Color{R: 1, G: 1, B: 1, A: 1}) {
		t.Fatalf("parsed measure = %+v, %v", got, err)
	}

	shape["scale"] = "twice"
	if _, err := ParseMeasureShape(shape); err == nil || !strings.Contains(err.Error(), `field "scale" has type`) {
		t.Fatalf("scale error = %v", err)
	}
	shape["scale"] = 2.0
	shape["color"] = []any{1.0}
	if _, err := ParseMeasureShape(shape); err == nil || !strings.Contains(err.Error(), `field "color"`) {
		t.Fatalf("color error = %v", err)
	}
}
