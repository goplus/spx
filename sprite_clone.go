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

	coreproject "github.com/goplus/spx/v4/internal/core/project"
	spxlog "github.com/goplus/spx/v4/internal/log"
)

type spriteUserFieldSnapshot struct {
	field reflect.Value
	value reflect.Value
}

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
	src.g.shapeMgr.addClonedShape(src, dest)
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
	restoreSpriteUserFields(userState)
	return dest
}

func instantiateStageSprite(out reflect.Value, source Sprite, properties spriteProperties) (*SpriteImpl, Sprite) {
	dest, outPtr := copySprite(out, source)
	applySpriteProperties(dest, properties)
	dest.initRuntimeProxy()
	return dest, outPtr
}

func dispatchCloneLifecycle(dest *SpriteImpl, data any) {
	defer dest.finishCloneInitialization()
	if dest.spriteState.hasOnCloned {
		dest.doWhenCloned(dest, data)
	}
}

func copySprite(out reflect.Value, source Sprite) (*SpriteImpl, Sprite) {
	in := reflect.ValueOf(source).Elem()
	layout, err := inspectCloneLayout(in.Type())
	if err != nil {
		panic(err)
	}
	outPtr := out.Addr().Interface().(Sprite)
	dest := layout.base(out)
	out.Set(in)
	for _, field := range layout.initFields {
		dstField := coreproject.WritableField(out.Field(field.field))
		srcField := coreproject.WritableField(in.Field(field.field))
		dstField.Addr().Method(field.method).Call([]reflect.Value{srcField.Addr()})
	}
	dest.sprite = outPtr
	dest.runtimeState.isCostumeDirty = true
	// The clone gets a fresh engine proxy, so its copied layer must be pushed
	// even when it is numerically unchanged from the source layer.
	dest.runtimeState.isLayerDirty = true

	src := layout.base(in)
	dest.components = cloneComponents(src, dest)
	return dest, outPtr
}

func snapshotSpriteUserFields(v reflect.Value) []spriteUserFieldSnapshot {
	if !v.CanAddr() {
		return nil
	}
	layout, err := inspectSpriteLayout(v.Type())
	if err != nil {
		panic(err)
	}
	out := make([]spriteUserFieldSnapshot, 0, len(layout.userFields))
	for _, i := range layout.userFields {
		field := coreproject.WritableField(v.Field(i))
		saved := reflect.New(field.Type()).Elem()
		saved.Set(field)
		out = append(out, spriteUserFieldSnapshot{field: field, value: saved})
	}
	return out
}

func restoreSpriteUserFields(state []spriteUserFieldSnapshot) {
	for _, saved := range state {
		saved.field.Set(saved.value)
	}
}
