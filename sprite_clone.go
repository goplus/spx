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
	"context"
	"reflect"

	coreproject "github.com/goplus/spx/v2/internal/core/project"
	"github.com/goplus/spx/v2/internal/engine"
	spxlog "github.com/goplus/spx/v2/internal/log"
)

func XGot_SpriteImpl_Clone__0(sprite Sprite) {
	XGot_SpriteImpl_Clone__1(sprite, nil)
}

func XGot_SpriteImpl_Clone__1(sprite Sprite, data any) {
	doClone(sprite, data, false, nil)
}

func doClone(sprite Sprite, data any, isAsync bool, onCloned func(sprite *SpriteImpl)) {
	if sprite == nil {
		spxlog.Panicf("doClone: sprite is nil")
	}
	src := spriteOf(sprite)
	if isDebugInstrEnabled() {
		spxlog.Debug("Clone: %s", src.name)
	}
	in := reflect.ValueOf(sprite).Elem()
	v := reflect.New(in.Type())
	out, outPtr := v.Elem(), v.Interface().(Sprite)
	dest := cloneSprite(out, outPtr, in, nil)
	src.g.addClonedShape(src, dest)
	if onCloned != nil {
		onCloned(dest)
	}
	if dest.spriteState.HasOnCloned {
		if isAsync {
			engine.Go(dest.pthis, func(ctx context.Context) {
				dest.doWhenAwake(dest)
				dest.doWhenCloned(dest, data)
			})
		} else {
			dest.doWhenAwake(dest)
			dest.doWhenCloned(dest, data)
		}
	}
}

func cloneSprite(out reflect.Value, outPtr Sprite, in reflect.Value, v coreproject.StageShape) *SpriteImpl {
	dest := spriteOf(outPtr)
	func() {
		out.Set(in)
		for i, n := 0, out.NumField(); i < n; i++ {
			fld := out.Field(i).Addr()
			if ini := fld.MethodByName("InitFrom"); ini.IsValid() {
				ini.Call([]reflect.Value{in.Field(i).Addr()})
			}
		}
	}()
	dest.sprite = outPtr
	dest.runtimeState.IsCostumeDirty = true

	src := spriteOf(in.Addr().Interface().(Sprite))
	dest.components.cloneFrom(&src.components, dest)

	if v != nil {
		applySpriteProps(dest, v)
	} else {
		dest.onAwake(func() {
			dest.awake()
		})
		runMain(outPtr.Main)
	}
	dest.resetRuntimeProxy(true)
	return dest
}

func applySpriteProps(dest *SpriteImpl, v coreproject.StageShape) {
	transform := dest.transform()
	if x, ok := v["x"]; ok {
		transform.x = x.(float64)
	}
	if y, ok := v["y"]; ok {
		transform.y = y.(float64)
	}
	if heading, ok := v["heading"]; ok {
		transform.direction = heading.(float64)
	}
	if style, ok := v["rotationStyle"]; ok {
		transform.rotationStyle = toRotationStyle(style.(string))
	}
	if visible, ok := v["visible"]; ok {
		dest.spriteState.IsVisible = visible.(bool)
	}
	if size, ok := v["size"]; ok {
		dest.runtimeState.Scale = size.(float64)
	}
	if idx, ok := v["costumeIndex"]; ok {
		dest.setCostumeIndex(int(idx.(float64)))
	}
	dest.spriteState.Cloned = false
}

func applySprite(out reflect.Value, sprite Sprite, v coreproject.StageShape) (*SpriteImpl, Sprite) {
	in := reflect.ValueOf(sprite).Elem()
	outPtr := out.Addr().Interface().(Sprite)
	return cloneSprite(out, outPtr, in, v), outPtr
}
