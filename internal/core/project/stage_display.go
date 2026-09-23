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
	"fmt"
	"reflect"

	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v3/internal/tools"
)

const (
	MonitorModeDefault = 1
	MonitorModeLarge   = 2
	MonitorModeSlider  = 3
	MonitorModeList    = 4
)

type MonitorShape struct {
	Target, Val, Name, Label string
	Mode                     float64
	Style                    string
	Size, X, Y               float64
	Visible                  bool
}

func ParseMonitorShape(shape StageShape) (config MonitorShape, err error) {
	if config.Target, err = shapeField[string](shape, "target"); err != nil {
		return
	}
	if config.Val, err = shapeField[string](shape, "val"); err != nil {
		return
	}
	if config.Name, err = shapeField[string](shape, "name"); err != nil {
		return
	}
	if config.Label, err = shapeField[string](shape, "label"); err != nil {
		return
	}
	mode, _ := shape["mode"].(string)
	switch mode {
	case "slider":
		config.Mode = MonitorModeSlider
	case "list":
		config.Mode = MonitorModeList
	default:
		if config.Mode, err = shapeField[float64](shape, "mode"); err != nil {
			return
		}
	}
	if config.X, err = shapeField[float64](shape, "x"); err != nil {
		return
	}
	if config.Y, err = shapeField[float64](shape, "y"); err != nil {
		return
	}
	if config.Visible, err = shapeField[bool](shape, "visible"); err != nil {
		return
	}
	config.Style, _ = ShapeValue(shape, "style", "default").(string)
	config.Size = 1
	if shape["size"] != nil {
		config.Size, _ = tools.GetFloat(shape["size"])
	}
	return
}

type MeasureShape struct {
	Size, Scale, Heading, X, Y float64
	Color                      mathf.Color
}

func ParseMeasureShape(shape StageShape) (config MeasureShape, err error) {
	if config.Size, err = shapeField[float64](shape, "size"); err != nil {
		return
	}
	if config.X, err = shapeField[float64](shape, "x"); err != nil {
		return
	}
	if config.Y, err = shapeField[float64](shape, "y"); err != nil {
		return
	}
	if config.Scale, err = shapeField(shape, "scale", 1.0); err != nil {
		return
	}
	if config.Heading, err = shapeField(shape, "heading", 0.0); err != nil {
		return
	}
	color := ShapeValue(shape, "color", 0.0)
	if config.Color, err = mathf.NewColorAny(color); err != nil {
		err = fmt.Errorf("stage shape field %q: %w", "color", err)
	}
	return
}

func shapeField[T any](shape StageShape, key string, fallback ...T) (T, error) {
	value, ok := shape[key]
	if !ok {
		if len(fallback) != 0 {
			return fallback[0], nil
		}
		var zero T
		return zero, fmt.Errorf("stage shape field %q is required", key)
	}
	if typed, ok := value.(T); ok {
		return typed, nil
	}
	var zero T
	return zero, fmt.Errorf("stage shape field %q has type %T, want %s", key, value, reflect.TypeFor[T]())
}
