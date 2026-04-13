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
)

type SpriteInitConfig[T any] struct {
	Items        []T
	Setup        func(T)
	CameraTarget string
	FollowCamera func(string)
	OnLoaded     func()
}

func WalkZOrder(
	zorder []any,
	onSprite func(layer int, name string) error,
	onSpecial func(layer int, shape StageShape) error,
) error {
	for layer, entry := range zorder {
		if name, ok := entry.(string); ok {
			if err := onSprite(layer, name); err != nil {
				return err
			}
			continue
		}
		shape, ok := entry.(StageShape)
		if !ok {
			return fmt.Errorf("invalid zorder entry type %T", entry)
		}
		if err := onSpecial(layer, shape); err != nil {
			return err
		}
	}
	return nil
}

func RunSpriteInitializers[T any](cfg SpriteInitConfig[T]) {
	for _, item := range cfg.Items {
		if cfg.Setup != nil {
			cfg.Setup(item)
		}
	}

	if cfg.CameraTarget != "" && cfg.FollowCamera != nil {
		cfg.FollowCamera(cfg.CameraTarget)
	}
	if cfg.OnLoaded != nil {
		cfg.OnLoaded()
	}
}

func WalkFields(
	v reflect.Value,
	resolve func(fieldIndex int) (string, any),
	visit func(fieldName string, fieldValue any) error,
) error {
	for i, n := 0, v.NumField(); i < n; i++ {
		name, value := resolve(i)
		if err := visit(name, value); err != nil {
			return err
		}
	}
	return nil
}

func BindStageSprite(
	v reflect.Value,
	target string,
	findObject func(reflect.Value, string, int) any,
	bind func(any) error,
) error {
	val := findObject(v, target, 0)
	if val == nil {
		return fmt.Errorf("unexpected - %s", target)
	}
	return bind(val)
}

func BindStageSprites(
	v reflect.Value,
	target string,
	items []any,
	findField func(reflect.Value, string, int) any,
	isSpriteType func(reflect.Type) bool,
	bind func(itemValue reflect.Value, shape StageShape) error,
) error {
	val := findField(v, target, 0)
	if val == nil {
		return fmt.Errorf("unexpected - %s", target)
	}

	fldSlice := reflect.ValueOf(val).Elem()
	if fldSlice.Kind() != reflect.Slice {
		return fmt.Errorf("unexpected - %s", target)
	}

	typSlice := fldSlice.Type()
	typItem := typSlice.Elem()
	isPtr := typItem.Kind() == reflect.Pointer
	typItemPtr := typItem
	if isPtr {
		typItem = typItem.Elem()
	} else {
		typItemPtr = reflect.PointerTo(typItem)
	}
	if !isSpriteType(typItemPtr) {
		return fmt.Errorf("unexpected - %s", target)
	}

	n := len(items)
	newSlice := reflect.MakeSlice(typSlice, n, n)
	for i := range n {
		newItem := newSlice.Index(i)
		if isPtr {
			newItem.Set(reflect.New(typItem))
			newItem = newItem.Elem()
		}
		shape, ok := items[i].(StageShape)
		if !ok {
			return fmt.Errorf("unexpected stage sprite item type %T", items[i])
		}
		if err := bind(newItem, shape); err != nil {
			return err
		}
	}
	fldSlice.Set(newSlice)
	return nil
}
