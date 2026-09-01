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
	"sync/atomic"
	"unsafe"

	coreproject "github.com/goplus/spx/v3/internal/core/project"
	"github.com/goplus/spx/v3/internal/coroutine"
	spxlog "github.com/goplus/spx/v3/internal/log"
)

var spriteImplType = reflect.TypeOf(SpriteImpl{})

func (p *SpriteImpl) Clone__0() {
	p.CloneWith(nil)
}

func (p *SpriteImpl) Clone__1(data any) {
	p.CloneWith(data)
}

func (p *SpriteImpl) CloneWith(__xgo_optional_data any) {
	doClone(p.sprite, __xgo_optional_data, false, nil)
}

// TODO(xsw): use classfile clone mechanism instead of reflection.
func doClone(sprite Sprite, data any, isAsync bool, onCloned func(sprite *SpriteImpl)) {
	if sprite == nil {
		spxlog.Panicf("DoClone: sprite is nil")
	}
	creator := currentCloneCreator()
	if cloneCreatorCanceled(creator) {
		// A canceled script must not create a fresh lifecycle that escapes the
		// AbortAll snapshot it already belongs to.
		gco.Abort()
	}
	src := spriteOf(sprite)
	if isDebugInstrEnabled() {
		spxlog.Debug("Clone: %s", src.name)
	}
	in := reflect.ValueOf(sprite).Elem()
	v := reflect.New(in.Type())
	out, outPtr := v.Elem(), v.Interface().(Sprite)
	dest := copySprite(out, outPtr, in, nil)
	creation := &cloneCreation{
		dest:         dest,
		generation:   src.g.currentBootstrapGeneration(),
		stopAllEpoch: src.scriptEventRegistry.stopAllEpoch.Load(),
	}
	if gco != nil {
		creation.abortEpoch = gco.AbortEpoch()
	}
	handedOff := false
	defer func() {
		if !handedOff {
			creation.rollback()
		}
	}()

	dest.initCloneRuntimeProxy()
	dest.awake()
	// Main installs the clone's event handlers. Keep the clone outside the
	// active shape list until that internal initialization has completed.
	initializeClone(out, outPtr)
	if dest.isDestroyed() {
		abortCanceledCloneHandoff(creation, creator)
		return
	}
	abortCanceledCloneHandoff(creation, creator)
	if !creation.isCurrent() {
		return
	}
	if !src.g.shapeMgr.tryAddClonedShape(src, dest) {
		spxlog.Debug("AddClonedShape: cloning a deleted sprite")
		if gco != nil {
			gco.Abort()
		}
		return
	}
	creation.inserted = true
	if onCloned != nil {
		onCloned(dest)
	}
	if dest.isDestroyed() {
		creation.completeWithoutPublication()
		handedOff = true
		abortCanceledCloneHandoff(creation, creator)
		return
	}
	abortCanceledCloneHandoff(creation, creator)
	if !creation.isCurrent() {
		return
	}

	if !dest.spriteState.HasOnCloned {
		// Without a clone hat there is no user-code slice to wait for. Finalize
		// inline so a nested clone block does not introduce a scheduler yield.
		if gco != nil && !gco.AdmitChildHandoff(creator, creation.abortEpoch) {
			abortCanceledCloneHandoff(creation, creator)
			return
		}
		handedOff = true
		func() {
			defer creation.rollback()
			runCloneLifecycle(creation, data, creator)
		}()
		abortCanceledCloneHandoff(creation, creator)
		return
	}
	lifecycle := gco.CreateChildWithFinalizer(
		creator,
		creation.abortEpoch,
		cloneLifecycleOwner{sprite: dest},
		func(me coroutine.Thread) int {
			runCloneLifecycle(creation, data, me)
			return 0
		},
		creation.rollbackFromFinalizer,
	)
	if lifecycle == nil {
		abortCanceledCloneHandoff(creation, creator)
		return
	}
	handedOff = true
	if !isAsync {
		gco.Join(lifecycle)
	}
}

func currentCloneCreator() coroutine.Thread {
	if gco == nil || !gco.IsInCoroutine() {
		return nil
	}
	return gco.Current()
}

func cloneCreatorCanceled(creator coroutine.Thread) bool {
	if creator == nil {
		return false
	}
	if creator.Stopped() {
		return true
	}
	select {
	case <-creator.Context().Done():
		return true
	default:
		return false
	}
}

func abortCanceledCloneHandoff(creation *cloneCreation, creator coroutine.Thread) {
	if creator == nil {
		return
	}
	if cloneCreatorCanceled(creator) ||
		(gco != nil && gco.AbortEpoch() != creation.abortEpoch) {
		gco.Abort()
	}
}

func cloneSprite(out reflect.Value, outPtr Sprite, in reflect.Value, v coreproject.StageShape) *SpriteImpl {
	dest := copySprite(out, outPtr, in, v)
	if v == nil {
		dest.initCloneRuntimeProxy()
		dest.awake()
		initializeClone(out, outPtr)
	} else {
		dest.initRuntimeProxy()
	}
	return dest
}

func copySprite(out reflect.Value, outPtr Sprite, in reflect.Value, v coreproject.StageShape) *SpriteImpl {
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

	src := spriteOf(in.Addr().Interface().(Sprite))
	dest.components.cloneFrom(&src.components, dest)

	if v != nil {
		applySpriteProps(dest, v)
	}
	return dest
}

func initializeClone(out reflect.Value, outPtr Sprite) {
	// Re-running Main re-registers clone events but also replays XGo_Init.
	// Save top-level user fields first, then restore them without changing
	// the existing out.Set(in) reference semantics.
	userState := snapshotSpriteUserFields(out)
	runMain(outPtr.Main)
	restoreSpriteUserFields(out, userState)
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

type cloneLifecycleOwner struct {
	sprite *SpriteImpl
}

func (p cloneLifecycleOwner) Name() string {
	return "clone lifecycle: " + p.sprite.name
}

type cloneCreation struct {
	dest         *SpriteImpl
	generation   uint64
	stopAllEpoch uint64
	abortEpoch   uint64
	inserted     bool
	state        atomic.Uint32
}

type cloneCreationState uint32

const (
	cloneCreationPending cloneCreationState = iota
	cloneCreationPublished
	cloneCreationCompletedWithoutPublication
	cloneCreationRolledBack
)

func (p *cloneCreation) isCurrent() bool {
	dest := p.dest
	return p.canCommit() &&
		dest.scriptEventRegistry.stopAllEpoch.Load() == p.stopAllEpoch &&
		(gco == nil || gco.AbortEpoch() == p.abortEpoch)
}

// canCommit checks structural state that remains relevant after a queued
// main-thread callback has won admission against lifecycle cancellation.
func (p *cloneCreation) canCommit() bool {
	dest := p.dest
	return cloneCreationState(p.state.Load()) == cloneCreationPending &&
		dest.g.isBootstrapGenerationCurrent(p.generation) &&
		!dest.isDestroyed()
}

func (p *cloneCreation) rollback() {
	if gco == nil {
		p.rollbackSynchronized()
		return
	}
	gco.RunSynchronizedCleanup(p.rollbackSynchronized)
}

func (p *cloneCreation) rollbackFromFinalizer() {
	// Coroutine finalizers already run inside an uncancelable execution barrier.
	p.rollbackSynchronized()
}

func (p *cloneCreation) rollbackSynchronized() {
	if !p.state.CompareAndSwap(
		uint32(cloneCreationPending), uint32(cloneCreationRolledBack),
	) {
		return
	}
	rollbackCloneCreation(p.dest, p.inserted, p.generation)
}

func (p *cloneCreation) completeWithoutPublication() {
	p.state.CompareAndSwap(
		uint32(cloneCreationPending), uint32(cloneCreationCompletedWithoutPublication),
	)
}

func (p *cloneCreation) tryPublish() bool {
	return p.state.CompareAndSwap(
		uint32(cloneCreationPending), uint32(cloneCreationPublished),
	)
}

func runCloneLifecycle(creation *cloneCreation, data any, lifecycle coroutine.Thread) {
	dest := creation.dest

	if lifecycle != nil {
		select {
		case <-lifecycle.Context().Done():
			return
		default:
		}
	}
	// Stop All may run after the clone was inserted but before this detached
	// lifecycle gets the scheduler. Never start handlers from a stale epoch:
	// they would be absent from StopIf's snapshot and could escape the stop.
	if !creation.isCurrent() || dest.g.shapeMgr.findShapeIndex(dest) < 0 ||
		dest.runtimeState.SyncSprite == nil || !dest.spriteState.IsProxyPublicationPending {
		return
	}
	if dest.spriteState.HasOnCloned {
		if !dest.doWhenCloned(dest, data, func() bool {
			if lifecycle != nil {
				select {
				case <-lifecycle.Context().Done():
					return false
				default:
				}
			}
			return creation.isCurrent() &&
				dest.g.shapeMgr.findShapeIndex(dest) >= 0 &&
				dest.runtimeState.SyncSprite != nil &&
				dest.spriteState.IsProxyPublicationPending
		}) {
			return
		}
	}
	if dest.isDestroyed() {
		creation.completeWithoutPublication()
		return
	}
	if !creation.isCurrent() ||
		!dest.publishCloneRuntimeProxy(creation) {
		return
	}
}

func rollbackCloneCreation(dest *SpriteImpl, inserted bool, generation uint64) {
	cleanup := func() {
		dest.setDying()
		if !dest.g.isBootstrapGenerationCurrent(generation) {
			// Reset may clear the shared registry before a still-running clone
			// Main registers its handlers. Remove that stale owner explicitly;
			// engine/component teardown belongs to the discarded generation.
			dest.doDeleteClone()
			dest.runtimeState.SyncSprite = nil
			dest.markDestroyed()
			return
		}

		if !dest.isDestroyed() {
			dest.destroyWithoutCurrentAbort()
		} else {
			// Main may delete itself and then continue registering handlers.
			dest.doDeleteClone()
		}
		if !inserted && dest.runtimeState.SyncSprite != nil {
			// Normal teardown only queues proxies belonging to active shapes.
			dest.g.shapeMgr.remove(dest)
		}
	}
	cleanup()
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
