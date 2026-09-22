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
	"reflect"
	"unsafe"

	spxlog "github.com/goplus/spx/v3/internal/log"
)

var spriteImplType = reflect.TypeFor[SpriteImpl]()

func (p *SpriteImpl) Clone__0() {
	p.CloneWith(nil)
}

func (p *SpriteImpl) Clone__1(data any) {
	p.CloneWith(data)
}

func (p *SpriteImpl) CloneWith(__xgo_optional_data any) {
	doClone(p.sprite, __xgo_optional_data, nil)
}

// TODO(xsw): use classfile clone mechanism instead of reflection.
func doClone(sprite Sprite, data any, onCloned func(sprite *SpriteImpl)) {
	if sprite == nil {
		spxlog.Panicf("DoClone: sprite is nil")
	}
	src := spriteOf(sprite)
	dest := createRuntimeClone(src)
	if dest == nil {
		return
	}
	dest.requestRedrawIfVisible()
	if onCloned != nil {
		onCloned(dest)
	}
	dispatchCloneLifecycle(dest, data)
}

func createRuntimeClone(src *SpriteImpl) *SpriteImpl {
	shapes := &src.g.shapeMgr
	if !shapes.reserveClone() {
		return nil
	}
	defer func() { shapes.pendingClones-- }()

	if isDebugInstrEnabled() {
		spxlog.Debug("Clone: %s", src.name)
	}
	out := reflect.New(reflect.TypeOf(src.sprite).Elem()).Elem()
	dest := instantiateRuntimeClone(out, src.sprite)
	src.g.addClonedShape(src, dest)
	return dest
}

func instantiateRuntimeClone(out reflect.Value, source Sprite) *SpriteImpl {
	dest, outPtr := copySprite(out, source)
	// The native proxy must stay hidden until the clone's initialization
	// handlers have completed their first execution slice.
	dest.beginCloneProxyPublication()
	dest.initRuntimeProxy()
	dest.awake()

	// Re-running Main re-registers clone events but also replays XGo_Init.
	// Save top-level user fields first, then restore them without changing
	// the existing out.Set(in) reference semantics.
	userState := snapshotSpriteUserFields(out)
	runMain(outPtr.Main)
	restoreSpriteUserFields(out, userState)
	return dest
}

func instantiateStageSprite(out reflect.Value, source Sprite, properties spriteProperties) (*SpriteImpl, Sprite) {
	dest, outPtr := copySprite(out, source)
	applySpriteProperties(dest, properties)
	dest.initRuntimeProxy()
	return dest, outPtr
}

func copySprite(out reflect.Value, source Sprite) (*SpriteImpl, Sprite) {
	in := reflect.ValueOf(source).Elem()
	outPtr := out.Addr().Interface().(Sprite)
	dest := spriteOf(outPtr)
	func() {
		out.Set(in)
		for i, n := 0, out.NumField(); i < n; i++ {
			dstField := settableSpriteField(out.Field(i))
			srcField := settableSpriteField(in.Field(i))
			if !dstField.IsValid() || !srcField.IsValid() {
				continue
			}
			if ini := dstField.Addr().MethodByName("InitFrom"); ini.IsValid() {
				ini.Call([]reflect.Value{srcField.Addr()})
			}
		}
	}()
	dest.sprite = outPtr
	dest.runtimeState.IsCostumeDirty = true
	// The clone gets a fresh engine proxy, so its copied layer must be pushed
	// even when it is numerically unchanged from the source layer.
	dest.runtimeState.IsLayerDirty = true

	src := spriteOf(source)
	dest.components = cloneComponents(src, dest)
	return dest, outPtr
}

func snapshotSpriteUserFields(v reflect.Value) map[int]reflect.Value {
	out := make(map[int]reflect.Value, v.NumField())
	for i := 0; i < v.NumField(); i++ {
		fieldType := v.Type().Field(i).Type
		if fieldType == spriteImplType {
			continue
		}
		field := settableSpriteField(v.Field(i))
		if !field.IsValid() {
			continue
		}
		saved := reflect.New(field.Type()).Elem()
		saved.Set(field)
		out[i] = saved
	}
	return out
}

func restoreSpriteUserFields(v reflect.Value, state map[int]reflect.Value) {
	for i, saved := range state {
		field := settableSpriteField(v.Field(i))
		if !field.IsValid() {
			continue
		}
		field.Set(saved)
	}
}

func settableSpriteField(field reflect.Value) reflect.Value {
	if field.CanSet() {
		return field
	}
	if !field.CanAddr() {
		return reflect.Value{}
	}
	return reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
}

func dispatchCloneLifecycle(dest *SpriteImpl, data any) {
	defer dest.finishCloneInitialization()
	if dest.spriteState.HasOnCloned {
		dest.doWhenCloned(dest, data)
	}
}
