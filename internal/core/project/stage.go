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

import "fmt"

type StageShape = map[string]any

type StageShapeHandlers struct {
	StageMonitor func(StageShape) error
	Measure      func(StageShape) error
	Sprites      func(StageShape) error
	Sprite       func(StageShape) error
}

type StageItemHandlers[T any] struct {
	StageMonitor func(StageShape) error
	Measure      func(StageShape) error
	Sprites      func(StageShape) ([]T, error)
	Sprite       func(StageShape) (T, error)
}

func DispatchStageShape(shape StageShape, handlers StageShapeHandlers) error {
	typ, ok := shape["type"].(string)
	if !ok {
		return fmt.Errorf("invalid stage shape type")
	}

	switch typ {
	case "stageMonitor", "monitor":
		if handlers.StageMonitor == nil {
			return fmt.Errorf("missing stage monitor handler")
		}
		return handlers.StageMonitor(shape)
	case "measure":
		if handlers.Measure == nil {
			return fmt.Errorf("missing measure handler")
		}
		return handlers.Measure(shape)
	case "sprites":
		if handlers.Sprites == nil {
			return fmt.Errorf("missing sprites handler")
		}
		return handlers.Sprites(shape)
	case "sprite":
		if handlers.Sprite == nil {
			return fmt.Errorf("missing sprite handler")
		}
		return handlers.Sprite(shape)
	default:
		return fmt.Errorf("unknown shape - %s", typ)
	}
}

func AppendStageItems[T any](items []T, shape StageShape, handlers StageItemHandlers[T]) ([]T, error) {
	err := DispatchStageShape(shape, StageShapeHandlers{
		StageMonitor: handlers.StageMonitor,
		Measure:      handlers.Measure,
		Sprites: func(shape StageShape) error {
			if handlers.Sprites == nil {
				return fmt.Errorf("missing sprites handler")
			}
			newItems, err := handlers.Sprites(shape)
			if err != nil {
				return err
			}
			items = append(items, newItems...)
			return nil
		},
		Sprite: func(shape StageShape) error {
			if handlers.Sprite == nil {
				return fmt.Errorf("missing sprite handler")
			}
			item, err := handlers.Sprite(shape)
			if err != nil {
				return err
			}
			items = append(items, item)
			return nil
		},
	})
	return items, err
}

func ShapeValue(shape StageShape, key string, defaultVal ...any) any {
	if v, ok := shape[key]; ok {
		return v
	}
	if len(defaultVal) > 0 {
		return defaultVal[0]
	}
	return nil
}
