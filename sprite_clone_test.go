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
	"fmt"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/goplus/spbase/mathf"
	coreproject "github.com/goplus/spx/v4/internal/core/project"
	"github.com/goplus/spx/v4/internal/coroutine"
	"github.com/goplus/spx/v4/internal/engine"
	"github.com/goplus/spx/v4/internal/enginewrap"
	pkgengine "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

type cloneAwakeOrderSprite struct {
	SpriteImpl
	sawAwakeInMain *bool
	onClonedFired  *bool
	clonedDone     chan struct{}
}

type cloneStatePreservingSprite struct {
	SpriteImpl
	cloneValue  float64
	recordValue *float64
	clonedDone  chan struct{}
}

type cloneFieldPayload struct {
	label string
	count int
}

type cloneAllFieldKindsObserved struct {
	boolValue      bool
	intValue       int
	stringValue    string
	floatValue     float64
	valueString    string
	listString     string
	listLen        int
	sliceValue     []int
	mapValue       map[string]int
	structValue    cloneFieldPayload
	arrayValue     [2]string
	pointerValue   *cloneFieldPayload
	interfaceValue any
}

type cloneAllFieldKindsSprite struct {
	SpriteImpl
	boolValue      bool
	intValue       int
	stringValue    string
	floatValue     float64
	valueValue     Value
	listValue      List
	sliceValue     []int
	mapValue       map[string]int
	structValue    cloneFieldPayload
	arrayValue     [2]string
	pointerValue   *cloneFieldPayload
	interfaceValue any
	observed       *cloneAllFieldKindsObserved
	clonedDone     chan struct{}
}

type cloneProxyInitSprite struct {
	SpriteImpl
	finalCostume     SpriteCostumeName
	hideOnCloned     bool
	deleteOnCloned   bool
	registerCloned   bool
	stopOthers       bool
	waitOnCloned     <-chan struct{}
	onClonedReturned chan<- struct{}
}

type cloneProxyOperation struct {
	kind    string
	object  pkgengine.Object
	path    string
	visible bool
}

type spyCloneSpriteMgr struct {
	enginewrap.SpriteMgrImpl
	mu         sync.Mutex
	nextID     pkgengine.Object
	zIndexes   map[pkgengine.Object]int64
	operations []cloneProxyOperation
	shaderIDs  []pkgengine.Object
}

type cloneLimitSprite struct {
	SpriteImpl
	onInit  func(*cloneLimitSprite)
	onClone func(*cloneLimitSprite)
}

type layoutInvalidInitializer int

type layoutInvalidInitSprite struct {
	SpriteImpl
	owner *Game
	value layoutInvalidInitializer
}

type layoutCloneValue struct{ value int }

type layoutCloneSprite struct {
	value layoutCloneValue
	owner *Game
	SpriteImpl
	variadic layoutVariadicValue
	optional layoutOptionalValue
	list     List
	items    []int
}

type layoutVariadicValue int

type layoutOptionalValue int

func TestInstantiateRuntimeCloneRunsCloneLifecycle(t *testing.T) {
	var game Game
	game.initShapeMgr()
	setupCloneSpriteMgr(t)
	source := newCloneAwakeOrderSprite(&game, "SpriteA")
	out := reflect.New(reflect.TypeOf(source).Elem()).Elem()
	dest := instantiateRuntimeClone(out, source)

	if !dest.IsCloned() {
		t.Fatal("runtime clone is not marked cloned")
	}
	if got := dest.CostumeIndex(); got != 0 {
		t.Fatalf("runtime clone costume = %d, want default costume 0 after awake", got)
	}
	if !dest.spriteState.hasOnCloned || !*source.sawAwakeInMain {
		t.Fatal("runtime clone did not run Main after awake")
	}
	if dest.runtimeState.syncSprite == nil {
		t.Fatal("runtime clone proxy was not initialized")
	}
	if !dest.isCloneProxyPublicationBlocked() {
		t.Fatal("runtime clone publication state was not created")
	}
}

func TestSpriteInitFromCopiesDefaultCostumeIndex(t *testing.T) {
	source := &SpriteImpl{spriteState: spriteRuntimeState{defaultCostumeIndex: 3}}
	dest := &SpriteImpl{spriteState: spriteRuntimeState{defaultCostumeIndex: 1}}

	dest.InitFrom(source)

	if got := dest.spriteState.defaultCostumeIndex; got != source.spriteState.defaultCostumeIndex {
		t.Fatalf("default costume index = %d, want %d", got, source.spriteState.defaultCostumeIndex)
	}
}

func TestFinishCloneInitializationPreservesPublicationState(t *testing.T) {
	for _, tt := range []struct {
		name      string
		initial   uint32
		destroyed bool
		want      uint32
	}{
		{"pending becomes ready", cloneProxyPending, false, cloneProxyReady},
		{"ready stays ready", cloneProxyReady, false, cloneProxyReady},
		{"published stays published", cloneProxyPublished, false, cloneProxyPublished},
		{"destroyed stays pending", cloneProxyPending, true, cloneProxyPending},
	} {
		t.Run(tt.name, func(t *testing.T) {
			sprite := &SpriteImpl{
				g:                &Game{},
				proxyPublication: &cloneProxyPublication{},
			}
			sprite.proxyPublication.state.Store(tt.initial)
			if tt.destroyed {
				sprite.markDestroyed()
			}
			for range 2 {
				sprite.finishCloneInitialization()
				if got := sprite.cloneProxyPublicationState(); got != tt.want {
					t.Fatalf("publication state = %d, want %d", got, tt.want)
				}
			}
		})
	}
}

func TestFinishCloneInitializationLeavesOrdinarySpriteUngated(t *testing.T) {
	sprite := &SpriteImpl{g: &Game{}}
	sprite.finishCloneInitialization()
	if sprite.proxyPublication != nil || sprite.isCloneProxyPublicationBlocked() {
		t.Fatal("ordinary sprite acquired a clone publication gate")
	}
}

func TestCloneProxyStaysHiddenUntilOnClonedFirstSlice(t *testing.T) {
	setupRuntimeScheduler(t)
	var game Game
	game.initShapeMgr()
	originalGame := engine.GetGame()
	engine.SetGame(&game)
	t.Cleanup(func() { engine.SetGame(originalGame) })
	mgr := setupCloneSpriteMgr(t)
	source := newCloneProxyInitSprite(&game, true, false)
	release := make(chan struct{})
	defer close(release)
	handlerReturned := make(chan struct{})
	source.waitOnCloned = release
	source.onClonedReturned = handlerReturned
	game.shapeMgr.add(spriteOf(source))

	var clone *SpriteImpl
	doClone(source, nil, func(sprite *SpriteImpl) {
		clone = sprite
	})

	select {
	case <-handlerReturned:
		t.Fatal("clone publication waited for the entire onCloned handler")
	default:
	}
	if clone == nil || clone.cloneProxyPublicationState() != cloneProxyReady {
		t.Fatal("clone did not reach the first-slice publication barrier")
	}
	mgr.assertNoVisibleWrites(t)

	flushCloneProxyUpdates(&game)
	if clone.isCloneProxyPublicationBlocked() {
		t.Fatal("clone publication gate was not committed by the proxy batch")
	}

	operations := mgr.recordedOperations()
	finalCostume := -1
	firstVisible := -1
	for i, op := range operations {
		if op.kind == "texture" && strings.HasSuffix(op.path, "final.png") {
			finalCostume = i
		}
		if (op.kind == "visible" || op.kind == "transform") && op.visible && firstVisible < 0 {
			firstVisible = i
		}
	}
	if finalCostume < 0 {
		t.Fatalf("final costume was not applied; operations = %#v", operations)
	}
	if firstVisible < 0 {
		t.Fatalf("clone was not published visible; operations = %#v", operations)
	}
	if firstVisible <= finalCostume {
		t.Fatalf("clone became visible before final costume; operations = %#v", operations)
	}
}

func TestCloneDeletedDuringFirstSliceIsNeverPublished(t *testing.T) {
	setupRuntimeScheduler(t)
	var game Game
	game.initShapeMgr()
	originalGame := engine.GetGame()
	engine.SetGame(&game)
	t.Cleanup(func() { engine.SetGame(originalGame) })
	mgr := setupCloneSpriteMgr(t)
	source := newCloneProxyInitSprite(&game, true, false)
	source.deleteOnCloned = true
	game.shapeMgr.add(spriteOf(source))

	var clone *SpriteImpl
	doClone(source, nil, func(sprite *SpriteImpl) {
		clone = sprite
	})

	if clone == nil || !clone.isDestroyed() {
		t.Fatal("clone deleted during onCloned was not destroyed")
	}
	mgr.assertNoVisibleWrites(t)
}

func TestCloneStopDuringFirstSliceStillReachesPublicationBatch(t *testing.T) {
	co := setupRuntimeScheduler(t)
	var game Game
	game.initShapeMgr()
	originalGame := engine.GetGame()
	engine.SetGame(&game)
	t.Cleanup(func() { engine.SetGame(originalGame) })
	mgr := setupCloneSpriteMgr(t)
	source := newCloneProxyInitSprite(&game, true, false)
	source.stopOthers = true
	game.shapeMgr.add(spriteOf(source))

	creatorDone := make(chan struct{})
	var clone *SpriteImpl
	co.Create(source, func(coroutine.Thread) {
		defer close(creatorDone)
		doClone(source, nil, func(sprite *SpriteImpl) {
			clone = sprite
		})

	})
	updateRuntimeEventSchedulerUntil(t, co, func() bool {
		select {
		case <-creatorDone:
			return true
		default:
			return false
		}
	})

	if clone == nil || game.shapeMgr.findShapeIndex(clone) < 0 {
		t.Fatal("canceled creator lost its initialized clone")
	}
	if clone.cloneProxyPublicationState() != cloneProxyReady {
		t.Fatal("canceled creator stranded clone before the publication batch")
	}
	mgr.assertNoVisibleWrites(t)

	flushCloneProxyUpdates(&game)
	if clone.isCloneProxyPublicationBlocked() {
		t.Fatal("publication batch did not finish canceled creator's clone")
	}
}

func TestCloneProxyPublishesFinalVisibility(t *testing.T) {
	for _, test := range []struct {
		name         string
		hideOnCloned bool
		wantVisible  bool
	}{
		{name: "shown", wantVisible: true},
		{name: "hidden", hideOnCloned: true, wantVisible: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			var game Game
			game.initShapeMgr()
			mgr := setupCloneSpriteMgr(t)
			source := newCloneProxyInitSprite(&game, true, test.hideOnCloned)
			game.shapeMgr.add(spriteOf(source))

			doClone(source, nil, nil)
			flushCloneProxyUpdates(&game)

			operations := mgr.recordedOperations()
			var published *cloneProxyOperation
			for i := range operations {
				if operations[i].kind == "transform" {
					published = &operations[i]
				}
			}
			if published == nil {
				t.Fatalf("clone visibility was not published; operations = %#v", operations)
			}
			if published.visible != test.wantVisible {
				t.Fatalf("published visibility = %v, want %v; operations = %#v", published.visible, test.wantVisible, operations)
			}
			if !test.wantVisible {
				mgr.assertNoVisibleWrites(t)
			}
		})
	}
}

func TestCloneProxyWithoutOnClonedIsReadyImmediately(t *testing.T) {
	var game Game
	game.initShapeMgr()
	mgr := setupCloneSpriteMgr(t)
	source := newCloneProxyInitSprite(&game, false, false)
	game.shapeMgr.add(spriteOf(source))

	var clone *SpriteImpl
	doClone(source, nil, func(sprite *SpriteImpl) {
		clone = sprite
	})

	if clone == nil {
		t.Fatal("clone callback did not capture cloned sprite")
	}
	if clone.cloneProxyPublicationState() != cloneProxyReady {
		t.Fatal("clone without onCloned handler was not immediately ready")
	}
	flushCloneProxyUpdates(&game)
	if clone.isCloneProxyPublicationBlocked() {
		t.Fatal("clone without onCloned handler was not published by the proxy batch")
	}
	operations := mgr.recordedOperations()
	if len(operations) == 0 || !operations[len(operations)-1].visible {
		t.Fatalf("clone without onCloned handler was not published; operations = %#v", operations)
	}
}

func TestPendingCloneProxySyncWritesStayHidden(t *testing.T) {
	var game Game
	game.initShapeMgr()
	mgr := setupCloneSpriteMgr(t)
	source := newCloneProxyInitSprite(&game, true, false)
	game.shapeMgr.add(spriteOf(source))

	in := reflect.ValueOf(source).Elem()
	out := reflect.New(in.Type()).Elem()
	clone := instantiateRuntimeClone(out, source)
	game.shapeMgr.addClonedShape(spriteOf(source), clone)

	clone.syncProxyForQuery()
	mgr.assertNoVisibleWrites(t)

	clone.markProxyDirty()
	buffer := engine.NewSpriteSyncBuffer(1)
	clone.prepareProxyUpdate()
	update := clone.collectProxyUpdate(buffer)
	serialized := buffer.Serialize()
	if len(serialized) != 2+engine.SyncFieldsPerSprite {
		t.Fatalf("serialized sync length = %d, want %d", len(serialized), 2+engine.SyncFieldsPerSprite)
	}
	if got := serialized[len(serialized)-1]; got != 0 {
		t.Fatalf("pending clone batch visibility = %v, want 0", got)
	}
	engine.SyncBatchUpdateSprites(serialized)
	commitProxyChange(update)

	// Both query and batch synchronization have already consumed the logical
	// transform. Opening the publication gate must still submit visibility.
	clone.finishCloneInitialization()
	buffer.Clear()
	clone.prepareProxyUpdate()
	update = clone.collectProxyUpdate(buffer)
	if got := buffer.UpdateCount(); got != 1 {
		t.Fatalf("publication batched %d updates, want 1", got)
	}
	serialized = buffer.Serialize()
	if got := serialized[len(serialized)-1]; got != 1 {
		t.Fatalf("published clone batch visibility = %v, want 1", got)
	}
	engine.SyncBatchUpdateSprites(serialized)
	commitProxyChange(update)
	buffer.Clear()
	clone.prepareProxyUpdate()
	clone.collectProxyUpdate(buffer)
	if got := buffer.UpdateCount(); got != 0 {
		t.Fatalf("published clone batched %d repeated updates, want 0", got)
	}
}

func TestCloneProxyPublicationReadyCanRaceProxyCollection(t *testing.T) {
	var game Game
	game.initShapeMgr()
	setupCloneSpriteMgr(t)

	for range 100 {
		clone := newCloneProxyInitSprite(&game, false, false)
		clone.beginCloneProxyPublication()
		clone.initRuntimeProxy()
		buffer := engine.NewSpriteSyncBuffer(1)
		start := make(chan struct{})
		var workers sync.WaitGroup
		workers.Add(2)
		go func() {
			defer workers.Done()
			<-start
			clone.finishCloneInitialization()
		}()
		go func() {
			defer workers.Done()
			<-start
			clone.prepareProxyUpdate()
			clone.collectProxyUpdate(buffer)
		}()
		close(start)
		workers.Wait()

		// Collection is allowed to win before READY. A later ordinary update
		// must observe the atomic transition and complete publication safely.
		clone.prepareProxyUpdate()
		clone.collectProxyUpdate(buffer)
		if clone.isCloneProxyPublicationBlocked() {
			t.Fatal("ready clone remained blocked after proxy collection")
		}
	}
}

func TestCloneSpriteAwakesBeforeMain(t *testing.T) {
	var game Game
	game.initShapeMgr()
	setupCloneSpriteMgr(t)

	source := newCloneAwakeOrderSprite(&game, "SpriteA")
	game.shapeMgr.add(spriteOf(source))

	var cloned *cloneAwakeOrderSprite
	doClone(source, nil, func(sprite *SpriteImpl) {
		var ok bool
		cloned, ok = sprite.sprite.(*cloneAwakeOrderSprite)
		if !ok {
			t.Fatalf("clone sprite type = %T, want *cloneAwakeOrderSprite", sprite.sprite)
		}
	})

	if cloned == nil {
		t.Fatal("clone callback did not capture cloned sprite")
	}

	select {
	case <-cloned.clonedDone:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for clone onCloned handler")
	}

	if cloned.sawAwakeInMain == nil || !*cloned.sawAwakeInMain {
		t.Fatal("clone Main ran before awake")
	}
	if cloned.onClonedFired == nil || !*cloned.onClonedFired {
		t.Fatal("clone onCloned did not fire after Main registration")
	}
}

func TestCloneSpriteInsertsImmediatelyBehindSource(t *testing.T) {
	var game Game
	game.initShapeMgr()
	setupCloneSpriteMgr(t)

	back := newCloneAwakeOrderSprite(&game, "Back")
	source := newCloneAwakeOrderSprite(&game, "Source")
	front := newCloneAwakeOrderSprite(&game, "Front")
	game.shapeMgr.add(spriteOf(back))
	game.shapeMgr.add(spriteOf(source))
	game.shapeMgr.add(spriteOf(front))

	var cloned *cloneAwakeOrderSprite
	doClone(source, nil, func(sprite *SpriteImpl) {
		var ok bool
		cloned, ok = sprite.sprite.(*cloneAwakeOrderSprite)
		if !ok {
			t.Fatalf("clone sprite type = %T, want *cloneAwakeOrderSprite", sprite.sprite)
		}
	})

	if cloned == nil {
		t.Fatal("clone callback did not capture cloned sprite")
	}

	shapes := game.getAllShapes()
	if got := len(shapes); got != 4 {
		t.Fatalf("shape count = %d, want 4", got)
	}
	if shapes[0] != spriteOf(back) {
		t.Fatalf("shape[0] = %v, want back sprite", shapes[0])
	}
	if shapes[1] != spriteOf(cloned) {
		t.Fatalf("shape[1] = %v, want cloned sprite", shapes[1])
	}
	if shapes[2] != spriteOf(source) {
		t.Fatalf("shape[2] = %v, want source sprite", shapes[2])
	}
	if shapes[3] != spriteOf(front) {
		t.Fatalf("shape[3] = %v, want front sprite", shapes[3])
	}

	if got, want := source.runtimeState.layer, 3; got != want {
		t.Fatalf("source layer = %d, want %d", got, want)
	}
	if got, want := cloned.runtimeState.layer, 2; got != want {
		t.Fatalf("clone layer = %d, want %d", got, want)
	}
	if got, want := front.runtimeState.layer, 4; got != want {
		t.Fatalf("front layer = %d, want %d", got, want)
	}
}

func TestCloneSpriteSynchronizesFinalLayerBeforePublication(t *testing.T) {
	var game Game
	game.initShapeMgr()
	mgr := setupCloneSpriteMgr(t)

	source := newCloneAwakeOrderSprite(&game, "Source")
	source.sawAwakeInMain = nil
	source.onClonedFired = nil
	source.clonedDone = nil
	source.runtimeState.layer = firstSpriteLayer + 6
	source.runtimeState.isLayerDirty = true
	source.initRuntimeProxy()
	sourceID := source.runtimeState.syncSprite.GetId()
	game.shapeMgr.add(spriteOf(source))

	var cloned *SpriteImpl
	doClone(source, nil, func(sprite *SpriteImpl) {
		cloned = sprite
	})

	if cloned == nil || cloned.runtimeState.syncSprite == nil {
		t.Fatal("clone proxy was not initialized")
	}
	flushCloneProxyUpdates(&game)

	cloneID := cloned.runtimeState.syncSprite.GetId()
	if got, want := mgr.zIndex(cloneID), int64(firstSpriteLayer); got != want {
		t.Fatalf("published clone proxy z-index = %d, want final layer %d", got, want)
	}
	if got, want := mgr.zIndex(sourceID), int64(firstSpriteLayer+1); got != want {
		t.Fatalf("source proxy z-index = %d, want shifted layer %d", got, want)
	}
}

func TestRepeatedClonesStackNewestBehindSourceAndInFrontOfOlderClone(t *testing.T) {
	var game Game
	game.initShapeMgr()
	setupCloneSpriteMgr(t)

	source := newCloneAwakeOrderSprite(&game, "Source")
	source.sawAwakeInMain = nil
	source.onClonedFired = nil
	source.clonedDone = nil
	game.shapeMgr.add(spriteOf(source))

	clones := make([]*SpriteImpl, 0, 2)
	for range 2 {
		doClone(source, nil, func(sprite *SpriteImpl) {
			clones = append(clones, sprite)
		})
	}

	shapes := game.getAllShapes()
	if got, want := len(shapes), 3; got != want {
		t.Fatalf("shape count = %d, want %d", got, want)
	}
	if shapes[0] != clones[0] {
		t.Fatalf("shape[0] = %v, want oldest clone", shapes[0])
	}
	if shapes[1] != clones[1] {
		t.Fatalf("shape[1] = %v, want newest clone", shapes[1])
	}
	if shapes[2] != spriteOf(source) {
		t.Fatalf("shape[2] = %v, want source sprite", shapes[2])
	}
	if got, want := source.runtimeState.layer, firstSpriteLayer+2; got != want {
		t.Fatalf("source layer = %d, want %d", got, want)
	}
	if got, want := clones[0].runtimeState.layer, firstSpriteLayer; got != want {
		t.Fatalf("oldest clone layer = %d, want %d", got, want)
	}
	if got, want := clones[1].runtimeState.layer, firstSpriteLayer+1; got != want {
		t.Fatalf("newest clone layer = %d, want %d", got, want)
	}
}

func TestCloneOfCloneInsertsImmediatelyBehindItsParent(t *testing.T) {
	var game Game
	game.initShapeMgr()
	setupCloneSpriteMgr(t)

	source := newCloneAwakeOrderSprite(&game, "Source")
	source.sawAwakeInMain = nil
	source.onClonedFired = nil
	source.clonedDone = nil
	game.shapeMgr.add(spriteOf(source))

	var first, second *SpriteImpl
	doClone(source, nil, func(sprite *SpriteImpl) {
		first = sprite
	})
	doClone(first.sprite, nil, func(sprite *SpriteImpl) {
		second = sprite
	})
	if first.proxyPublication == nil || second.proxyPublication == nil {
		t.Fatal("clone publication state was not initialized")
	}
	if first.proxyPublication == second.proxyPublication {
		t.Fatal("clone-of-clone shared publication state with its parent")
	}

	shapes := game.getAllShapes()
	if len(shapes) != 3 || shapes[0] != second || shapes[1] != first || shapes[2] != spriteOf(source) {
		t.Fatalf("clone-of-clone order = %v, want second clone, first clone, source", shapes)
	}
}

func TestCloneInsertionUsesSourceCurrentLayer(t *testing.T) {
	var game Game
	game.initShapeMgr()
	setupCloneSpriteMgr(t)

	source := newCloneAwakeOrderSprite(&game, "Source")
	source.sawAwakeInMain = nil
	source.onClonedFired = nil
	source.clonedDone = nil
	other := newCloneAwakeOrderSprite(&game, "Other")
	game.shapeMgr.add(spriteOf(source))

	clones := make([]*SpriteImpl, 0, 3)
	for range 2 {
		doClone(source, nil, func(sprite *SpriteImpl) {
			clones = append(clones, sprite)
		})
	}
	game.shapeMgr.add(spriteOf(other))
	game.shapeMgr.goBackLayers(spriteOf(other), 1)
	doClone(source, nil, func(sprite *SpriteImpl) {
		clones = append(clones, sprite)
	})

	shapes := game.getAllShapes()
	want := []Shape{clones[0], clones[1], spriteOf(other), clones[2], spriteOf(source)}
	if !reflect.DeepEqual(shapes, want) {
		t.Fatalf("clone order after source layer move = %v, want %v", shapes, want)
	}
}

func TestCloneSpritePreservesUserStateAfterMainRegistration(t *testing.T) {
	var game Game
	game.initShapeMgr()
	setupCloneSpriteMgr(t)

	var recorded float64
	source := newCloneStatePreservingSprite(&game, "SpriteA", &recorded)
	source.cloneValue = 3
	game.shapeMgr.add(spriteOf(source))

	var cloned *cloneStatePreservingSprite
	doClone(source, nil, func(sprite *SpriteImpl) {
		var ok bool
		cloned, ok = sprite.sprite.(*cloneStatePreservingSprite)
		if !ok {
			t.Fatalf("clone sprite type = %T, want *cloneStatePreservingSprite", sprite.sprite)
		}
	})

	if cloned == nil {
		t.Fatal("clone callback did not capture cloned sprite")
	}

	select {
	case <-cloned.clonedDone:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for clone onCloned handler")
	}

	if got := recorded; got != 3 {
		t.Fatalf("clone user state = %v, want 3", got)
	}
}

func TestCloneSpritePreservesAllTopLevelUserFields(t *testing.T) {
	var game Game
	game.initShapeMgr()
	setupCloneSpriteMgr(t)

	var observed cloneAllFieldKindsObserved
	source := newCloneAllFieldKindsSprite(&game, "SpriteA", &observed)
	payload := &cloneFieldPayload{label: "orig-pointer", count: 9}
	source.boolValue = true
	source.intValue = 42
	source.stringValue = "orig"
	source.floatValue = 3.14
	source.valueValue = NewValue("orig-value")
	source.listValue.Init("left", 7)
	source.sliceValue = []int{1, 2, 3}
	source.mapValue = map[string]int{"orig": 9}
	source.structValue = cloneFieldPayload{label: "orig-struct", count: 5}
	source.arrayValue = [2]string{"left", "right"}
	source.pointerValue = payload
	source.interfaceValue = payload
	game.shapeMgr.add(spriteOf(source))

	var cloned *cloneAllFieldKindsSprite
	doClone(source, nil, func(sprite *SpriteImpl) {
		var ok bool
		cloned, ok = sprite.sprite.(*cloneAllFieldKindsSprite)
		if !ok {
			t.Fatalf("clone sprite type = %T, want *cloneAllFieldKindsSprite", sprite.sprite)
		}
	})

	if cloned == nil {
		t.Fatal("clone callback did not capture cloned sprite")
	}

	select {
	case <-cloned.clonedDone:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for clone onCloned handler")
	}

	if observed.boolValue != true {
		t.Fatalf("boolValue = %v, want true", observed.boolValue)
	}
	if observed.intValue != 42 {
		t.Fatalf("intValue = %v, want 42", observed.intValue)
	}
	if observed.stringValue != "orig" {
		t.Fatalf("stringValue = %q, want %q", observed.stringValue, "orig")
	}
	if observed.floatValue != 3.14 {
		t.Fatalf("floatValue = %v, want 3.14", observed.floatValue)
	}
	if observed.valueString != "orig-value" {
		t.Fatalf("valueValue = %q, want %q", observed.valueString, "orig-value")
	}
	if observed.listLen != 2 || observed.listString != "left 7" {
		t.Fatalf("listValue = (%d, %q), want (2, %q)", observed.listLen, observed.listString, "left 7")
	}
	if !reflect.DeepEqual(observed.sliceValue, []int{1, 2, 3}) {
		t.Fatalf("sliceValue = %v, want [1 2 3]", observed.sliceValue)
	}
	if !reflect.DeepEqual(observed.mapValue, map[string]int{"orig": 9}) {
		t.Fatalf("mapValue = %v, want map[orig:9]", observed.mapValue)
	}
	if observed.structValue != (cloneFieldPayload{label: "orig-struct", count: 5}) {
		t.Fatalf("structValue = %+v, want %+v", observed.structValue, cloneFieldPayload{label: "orig-struct", count: 5})
	}
	if observed.arrayValue != [2]string{"left", "right"} {
		t.Fatalf("arrayValue = %v, want [left right]", observed.arrayValue)
	}
	if observed.pointerValue != payload {
		t.Fatalf("pointerValue = %p, want %p", observed.pointerValue, payload)
	}
	if observed.interfaceValue != payload {
		t.Fatalf("interfaceValue = %#v, want %#v", observed.interfaceValue, payload)
	}
}

func TestClonedSpriteCollisionTargetRegistrationDoesNotRefreshCollisionLayers(t *testing.T) {
	game := &collisionLayerOrderGame{}
	game.isAutoSetCollisionLayer = true
	game.sprCollisionInfos = map[string]*spriteCollisionInfo{
		"SpriteA": {Index: 0, Layer: 1 << 0},
		"SpriteB": {Index: 1, Layer: 1 << 1},
	}

	spriteA := newCollisionLayerOrderSprite(&game.Game, "SpriteA", nil)
	spriteB := newCollisionLayerOrderSprite(&game.Game, "SpriteB", nil)
	cloneA := newCollisionLayerOrderSprite(&game.Game, "SpriteA", nil)
	cloneA.spriteState.cloned = true

	game.setupCollisionData([]Sprite{spriteA, spriteB})
	spriteA.physics().collisionTargets["SpriteB"] = true
	cloneA.physics().addCollisionTarget("SpriteB")

	if got := game.sprCollisionInfos["SpriteA"].Mask; got != 0 {
		t.Fatalf("SpriteA collision mask = %d, want 0", got)
	}
	if got := game.sprCollisionInfos["SpriteB"].Mask; got != 0 {
		t.Fatalf("SpriteB collision mask = %d, want 0", got)
	}
}

func TestCloneLimitSharedAcrossSprites(t *testing.T) {
	game := setupCloneLimitGame(t)
	a := newCloneLimitSprite(game, "A")
	b := newCloneLimitSprite(game, "B")
	game.shapeMgr.add(&struct{}{}) // Non-sprite shapes do not use clone slots.
	var initialized, started int
	for _, s := range []*cloneLimitSprite{a, b} {
		s.onInit = func(*cloneLimitSprite) { initialized++ }
		s.onClone = func(*cloneLimitSprite) { started++ }
	}
	for range 150 {
		a.Clone__0()
		b.Clone__0()
	}
	before := append([]Shape(nil), game.getAllShapes()...)
	a.Clone__1("ignored")
	b.CloneWith(nil)
	if initialized != 300 || started != 300 {
		t.Fatalf("initialized %d and started %d clones, want 300 each", initialized, started)
	}
	if !reflect.DeepEqual(before, game.getAllShapes()) {
		t.Fatal("rejected clone changed the active shapes or layer order")
	}
	if game.shapeMgr.cloneCount != 300 || game.shapeMgr.pendingClones != 0 {
		t.Fatalf("clone counts = %d active, %d pending", game.shapeMgr.cloneCount, game.shapeMgr.pendingClones)
	}
	// All clones are hidden; hidden clones must still count towards the limit.
	for _, shape := range before {
		if s, ok := shape.(*SpriteImpl); ok && s.Visible() {
			t.Fatal("test unexpectedly created a visible sprite")
		}
	}
}

func TestCloneLimitReleasesSlotOnDeletionAndReset(t *testing.T) {
	game := setupCloneLimitGame(t)
	source := newCloneLimitSprite(game, "Source")
	var started int
	source.onClone = func(*cloneLimitSprite) { started++ }
	for range 300 {
		source.Clone__0()
	}
	source.DeleteThisClone() // Originals must not release a slot.
	source.Clone__0()
	if started != 300 {
		t.Fatal("DeleteThisClone on an original released a slot")
	}
	clone := game.getAllShapes()[0].(*SpriteImpl)
	clone.DeleteThisClone()
	game.shapeMgr.removeShape(clone) // Removing an already deleted shape is a no-op.
	source.Clone__0()                // Reuse before the deferred native deletion flush.
	source.Clone__0()
	if started != 301 || game.shapeMgr.cloneCount != 300 {
		t.Fatalf("after deletion: started %d, active %d; want 301, 300", started, game.shapeMgr.cloneCount)
	}
	game.shapeMgr.reset()
	source = newCloneLimitSprite(game, "Restarted")
	source.onClone = func(*cloneLimitSprite) { started++ }
	for range 301 {
		source.Clone__0()
	}
	if started != 601 || game.shapeMgr.cloneCount != 300 {
		t.Fatalf("after reset: started %d, active %d; want 601, 300", started, game.shapeMgr.cloneCount)
	}
}

func TestCloneLimitBoundsRecursiveCloneHandlers(t *testing.T) {
	game := setupCloneLimitGame(t)
	source := newCloneLimitSprite(game, "Recursive")
	var started, continued int
	done := make(chan struct{})
	source.onClone = func(s *cloneLimitSprite) {
		started++
		if started <= 300 { // Keep the regression bounded even without the fix.
			s.Clone__0()
		}
		continued++
		if continued == 300 {
			close(done)
		}
	}
	source.Clone__0()
	updateRuntimeEventSchedulerUntil(t, gco, func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	})
	if started != 300 || continued != 300 {
		t.Fatalf("recursive chain: started %d, continued %d; want 300 each", started, continued)
	}
	if len(game.getAllShapes()) != 301 {
		t.Fatalf("shape count = %d, want 301", len(game.getAllShapes()))
	}
	for i, shape := range game.getAllShapes() {
		if got := shape.(*SpriteImpl).runtimeState.layer; got != firstSpriteLayer+i {
			t.Fatalf("shape %d layer = %d, want %d", i, got, firstSpriteLayer+i)
		}
	}
}

func TestCloneLimitReservesSlotDuringInitialization(t *testing.T) {
	game := setupCloneLimitGame(t)
	source := newCloneLimitSprite(game, "Source")
	for range 299 {
		source.Clone__0()
	}
	var initialized int
	source.onInit = func(*cloneLimitSprite) {
		initialized++
		if initialized == 1 {
			source.Clone__0()
		}
	}
	source.Clone__0()
	if initialized != 1 || game.shapeMgr.cloneCount != 300 || game.shapeMgr.pendingClones != 0 {
		t.Fatalf("initialized %d, active %d, pending %d; want 1, 300, 0", initialized, game.shapeMgr.cloneCount, game.shapeMgr.pendingClones)
	}
}

func TestCloneLimitReleasesFailedInitialization(t *testing.T) {
	game := setupCloneLimitGame(t)
	source := newCloneLimitSprite(game, "Source")
	sentinel := new(int)
	source.onInit = func(*cloneLimitSprite) { panic(sentinel) }
	func() {
		defer func() {
			if got := recover(); got != sentinel {
				t.Fatalf("panic = %v, want initialization failure", got)
			}
		}()
		source.Clone__0()
	}()
	source.onInit = nil
	for range 300 {
		source.Clone__0()
	}
	if game.shapeMgr.cloneCount != 300 || game.shapeMgr.pendingClones != 0 {
		t.Fatalf("failed initialization leaked a slot: active %d, pending %d", game.shapeMgr.cloneCount, game.shapeMgr.pendingClones)
	}
}

func TestCloneLimitDeletionDuringHandlerReleasesSlot(t *testing.T) {
	game := setupCloneLimitGame(t)
	source := newCloneLimitSprite(game, "Source")
	var started int
	source.onClone = func(s *cloneLimitSprite) {
		started++
		s.DeleteThisClone()
	}
	for range 301 {
		source.Clone__0()
	}
	if started != 301 || game.shapeMgr.cloneCount != 0 || game.shapeMgr.pendingClones != 0 {
		t.Fatalf("started %d, active %d, pending %d; want 301, 0, 0", started, game.shapeMgr.cloneCount, game.shapeMgr.pendingClones)
	}
}

func TestCloneLimitPreservesLayerOrderAfterRecycling(t *testing.T) {
	game := setupCloneLimitGame(t)
	source := newCloneLimitSprite(game, "Source")
	clones := make([]*SpriteImpl, 0, 300)
	want := make([]string, 0, 301)
	for i := range 300 {
		name := fmt.Sprintf("Clone%d", i)
		doClone(source, nil, func(clone *SpriteImpl) {
			clone.name = name
			clones = append(clones, clone)
		})
		want = append(want, name)
	}
	// Scratch puts newer clones immediately behind the source, ahead of older ones.
	want = append(want, "Source")
	assertLayerOrder(t, game, want)
	clones[150].DeleteThisClone()
	want = slices.Delete(want, 150, 151)
	assertLayerOrder(t, game, want)

	source.SetLayerTo(Back)
	want = append([]string{"Source"}, want[:len(want)-1]...)
	assertLayerOrder(t, game, want)
	var descendant *SpriteImpl
	doClone(clones[0].sprite, nil, func(clone *SpriteImpl) {
		clone.name = "Descendant"
		descendant = clone
	})
	if descendant == nil {
		t.Fatal("deleted clone slot was not reusable")
	}
	// A clone of a clone goes behind its immediate parent, using the live order.
	want = slices.Insert(want, 1, "Descendant")
	assertLayerOrder(t, game, want)
	descendant.SetLayerTo(Front)
	want = append(slices.Delete(want, 1, 2), "Descendant")
	assertLayerOrder(t, game, want)
	descendant.SetLayer__1(Backward, 2)
	want = slices.Insert(want[:len(want)-1], len(want)-3, "Descendant")
	assertLayerOrder(t, game, want)
	source.Clone__0() // Full again: must not disturb layer order.
	assertLayerOrder(t, game, want)

	flushCloneProxyUpdates(game)
	mgr := pkgengine.SpriteMgr.(*spyCloneSpriteMgr)
	for _, shape := range game.getAllShapes() {
		sprite := shape.(*SpriteImpl)
		if sprite.runtimeState.syncSprite == nil {
			continue
		}
		if got := mgr.zIndex(sprite.runtimeState.syncSprite.GetId()); got != int64(sprite.runtimeState.layer) {
			t.Fatalf("%s rendered z-index = %d, want %d", sprite.name, got, sprite.runtimeState.layer)
		}
	}
}

func TestCloneLayerOrderHonorsOnClonedMoves(t *testing.T) {
	for _, test := range []struct {
		name   string
		action layerAction
		want   []string
	}{
		{"front", Front, []string{"Back", "Source", "Front", "Clone"}},
		{"back", Back, []string{"Clone", "Back", "Source", "Front"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			game := setupCloneLimitGame(t)
			newCloneLimitSprite(game, "Back")
			source := newCloneLimitSprite(game, "Source")
			newCloneLimitSprite(game, "Front")
			source.onClone = func(clone *cloneLimitSprite) {
				clone.SetLayerTo(test.action)
			}
			doClone(source, nil, func(clone *SpriteImpl) { clone.name = "Clone" })
			assertLayerOrder(t, game, test.want)
		})
	}
}

func TestCloneRejectsNilSprite(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("cloning nil did not reject the input")
		}
	}()
	doClone(nil, nil, nil)
}

func TestSpriteSnapshotSkipsUnaddressableUserFields(t *testing.T) {
	value := struct {
		value int
	}{value: 42}
	if got := snapshotSpriteUserFields(reflect.ValueOf(value)); len(got) != 0 {
		t.Fatalf("unaddressable snapshot = %v, want no writable fields", got)
	}
}

func TestSpriteSnapshotRejectsMissingImplementation(t *testing.T) {
	value := &struct{ count int }{count: 42}
	defer func() {
		err, ok := recover().(error)
		if !ok || !strings.Contains(err.Error(), "missing SpriteImpl") {
			t.Fatalf("snapshot panic = %v, want missing SpriteImpl error", err)
		}
		if value.count != 42 {
			t.Fatal("rejected snapshot changed user state")
		}
	}()
	snapshotSpriteUserFields(reflect.ValueOf(value).Elem())
}

func TestDebugCloneStillPreservesOriginalState(t *testing.T) {
	game := setupCloneLimitGame(t)
	game.debugState.DebugInstr = true
	original := newCloneLimitSprite(game, "original")
	original.SetXYpos(10, 20)
	cloned := createRuntimeClone(&original.SpriteImpl)
	if cloned == nil || !cloned.IsCloned() || cloned.Xpos() != 10 || cloned.Ypos() != 20 {
		t.Fatal("debug-enabled cloning lost source state")
	}
}

func TestSpriteLayoutChecksInitFromOnlyWhenCloning(t *testing.T) {
	game := setupStageLoadTestGame(t, gameTestFS{
		"sprites/initializer/index.json": `{"costumes":[{"name":"idle","imageWidth":10,"imageHeight":10}]}`,
	})
	source := &layoutInvalidInitSprite{}
	gamer := reflect.ValueOf(&game.Game).Elem()
	prepared, err := prepareSprite(game.fs, "initializer", reflect.TypeOf(source).Elem(), gamer)
	if err != nil {
		t.Fatal(err)
	}
	game.loadPreparedSprite(source, "initializer", gamer, &prepared)
	if spriteOf(source) != &source.SpriteImpl || game.sprs["initializer"] != source || source.owner != &game.Game {
		t.Fatal("unused InitFrom method affected lookup, publication, or owner binding")
	}

	clone := layoutInvalidInitSprite{value: 23}
	before := &layoutInvalidInitSprite{value: 23}
	var recovered any
	func() {
		defer func() { recovered = recover() }()
		copySprite(reflect.ValueOf(&clone).Elem(), source)
	}()
	err, ok := recovered.(error)
	if !ok || !strings.Contains(err.Error(), "field value InitFrom") {
		t.Fatalf("clone panic = %v, want invalid InitFrom error", recovered)
	}
	if !reflect.DeepEqual(&clone, before) {
		t.Fatal("invalid InitFrom changed the clone before rejection")
	}
}

func TestSpriteLayoutKeepsInitFromAndReferenceSemantics(t *testing.T) {
	game := setupCloneLimitGame(t)
	source := &layoutCloneSprite{
		owner: game,
		value: layoutCloneValue{value: 7},
		items: []int{1},
	}
	source.baseObj.initWithSize(1, 1)
	source.g, source.name, source.sprite = game, "layout", source
	source.scriptEventBindings.bind(&game.scriptEvents, &source.SpriteImpl)
	source.components.initComponents(&source.SpriteImpl, &coreproject.SpriteConfig{})
	source.physics().collisionInfo.Type = physicsColliderNone
	source.physics().triggerInfo.Type = physicsColliderNone
	game.shapeMgr.add(&source.SpriteImpl)
	source.list.Init("source")
	var clone layoutCloneSprite
	base, cloned := copySprite(reflect.ValueOf(&clone).Elem(), source)
	if base != &clone.SpriteImpl || cloned != &clone || clone.sprite != &clone || clone.owner != game {
		t.Fatal("clone lost its implementation, self reference, or owner")
	}
	if clone.value.value != 8 || source.value.value != 7 || clone.variadic != 1 || clone.optional != 1 {
		t.Fatalf("InitFrom values = clone %d, source %d, variadic %d, optional %d", clone.value.value, source.value.value, clone.variadic, clone.optional)
	}
	clone.list.Set(0, "clone")
	clone.items[0] = 2
	if source.list.String() != "source" || source.items[0] != 2 {
		t.Fatal("clone changed List ownership or shallow slice reference semantics")
	}
}

func TestCloneComponentsUseOriginalAndParentState(t *testing.T) {
	game := setupCloneLimitGame(t)
	game.soundMgr.Init(&cloneSoundBackend{})
	original := newCloneLimitSprite(game, "original")
	original.pen().penWidth = 5
	original.SetSoundEffect(SoundPanEffect, -40)
	original.SetGraphicEffect(ColorEffect, 10)
	parent := createRuntimeClone(&original.SpriteImpl)
	parent.SetXYpos(30, -15)
	parent.SetHeading(90)
	parent.SetPhysicsMode(DynamicPhysics)
	parent.physics().collisionTargets["touching"] = true
	parent.pen().penWidth = 50
	parent.SetSoundEffect(SoundPanEffect, 80)
	parent.SetGraphicEffect(ColorEffect, 20)
	parent.applyMaterialParamsVec4("atlas_uv_rect2", mathf.Vec4{Z: 1, W: 1})

	// Give the parent distinct read-only configuration and per-instance runtime state.
	parentAnimation := parent.animation()
	parentAnimations := *parentAnimation.shared
	parentAnimations.defaultAnimation = "parent"
	parentAnimation.shared = &parentAnimations
	parentAnimation.curAnimState = &animState{Name: "playing"}
	parentAnimation.activeTweenStates = []*animState{{Name: "moving"}}
	parentAnimation.defaultAnimActive = true
	parentAnimation.doneAnimations = []*animState{{Name: "completed"}}
	original.pen().penWidth = 7
	original.SetSoundEffect(SoundPanEffect, 20)

	child := createRuntimeClone(parent)
	if child.greffUniforms[ColorEffect] != 20 {
		t.Fatal("graphic effects did not inherit the parent's current value")
	}
	// Scalar and vector parameters share one shader initialization per proxy.
	// The child's copied state must still initialize its own material.
	mgr := pkgengine.SpriteMgr.(*spyCloneSpriteMgr)
	if want := []pkgengine.Object{parent.getSpriteId(), child.getSpriteId()}; !slices.Equal(mgr.shaderIDs, want) {
		t.Fatalf("shader initialization = %v, want one per clone %v", mgr.shaderIDs, want)
	}
	if child.pen().penWidth != 7 || child.GetSoundEffect(SoundPanEffect) != 20 {
		t.Fatal("pen and sound did not inherit the current original state")
	}
	if child.Xpos() != 30 || child.Ypos() != -15 || child.Heading() != 90 {
		t.Fatalf("transform = (%v, %v, %v), want parent transform (30, -15, 90)", child.Xpos(), child.Ypos(), child.Heading())
	}
	if child.PhysicsMode() != DynamicPhysics {
		t.Fatalf("physics mode = %v, want parent mode %v", child.PhysicsMode(), DynamicPhysics)
	}
	childAnimation := child.animation()
	if childAnimation.shared != parentAnimation.shared {
		t.Fatal("animation configuration did not come from the parent")
	}
	if childAnimation.curAnimState != nil || len(childAnimation.activeTweenStates) != 0 ||
		childAnimation.defaultAnimActive || len(childAnimation.doneAnimations) != 0 ||
		len(child.physics().collisionTargets) != 0 {
		t.Fatal("clone inherited parent collision or animation runtime state")
	}

	// The fresh proxy already starts at the copied position; the parent's
	// unsubmitted movement must not pin the child's later physical movement.
	child.mergeEnginePosition(40, -20)
	requirePosition(t, child, 40, -20)
	child.SetXpos(99)
	child.SetPhysicsMode(StaticPhysics)
	child.pen().penWidth = 9
	child.SetSoundEffect(SoundPanEffect, 90)
	child.SetGraphicEffect(ColorEffect, 30)
	child.ClearGraphicEffects()
	if parent.greffUniforms[ColorEffect] != 20 || original.greffUniforms[ColorEffect] != 10 {
		t.Fatal("changing or clearing descendant effects changed its parent or original")
	}
	if len(mgr.shaderIDs) != 2 {
		t.Fatal("later effect updates reinitialized a clone's shader")
	}
	if parent.Xpos() != 30 || parent.PhysicsMode() != DynamicPhysics || parent.pen().penWidth != 50 || parent.GetSoundEffect(SoundPanEffect) != 80 {
		t.Fatal("changing descendant state changed the parent")
	}
	if original.Xpos() != 0 || original.PhysicsMode() != NoPhysics || original.pen().penWidth != 7 || original.GetSoundEffect(SoundPanEffect) != 20 {
		t.Fatal("changing descendant state changed the original")
	}
}

func (s *cloneAwakeOrderSprite) Main() {
	if !s.IsCloned() {
		return
	}
	if s.sawAwakeInMain != nil {
		*s.sawAwakeInMain = s.CostumeIndex() == 0
	}
	s.OnCloned__0(func() {
		if s.onClonedFired != nil {
			*s.onClonedFired = true
		}
		if s.clonedDone != nil {
			close(s.clonedDone)
		}
	})
}

func (s *cloneStatePreservingSprite) Main() {
	s.cloneValue = 1
	if !s.IsCloned() {
		return
	}
	s.OnCloned__0(func() {
		if s.recordValue != nil {
			*s.recordValue = s.cloneValue
		}
		if s.clonedDone != nil {
			close(s.clonedDone)
		}
	})
}

func (s *cloneAllFieldKindsSprite) Main() {
	s.XGo_Init()
	if !s.IsCloned() {
		return
	}
	s.OnCloned__0(func() {
		if s.observed != nil {
			s.observed.boolValue = s.boolValue
			s.observed.intValue = s.intValue
			s.observed.stringValue = s.stringValue
			s.observed.floatValue = s.floatValue
			s.observed.valueString = s.valueValue.String()
			s.observed.listString = s.listValue.String()
			s.observed.listLen = s.listValue.Len()
			s.observed.sliceValue = append([]int(nil), s.sliceValue...)
			if s.mapValue != nil {
				s.observed.mapValue = make(map[string]int, len(s.mapValue))
				for key, val := range s.mapValue {
					s.observed.mapValue[key] = val
				}
			}
			s.observed.structValue = s.structValue
			s.observed.arrayValue = s.arrayValue
			s.observed.pointerValue = s.pointerValue
			s.observed.interfaceValue = s.interfaceValue
		}
		if s.clonedDone != nil {
			close(s.clonedDone)
		}
	})
}

func (s *cloneAllFieldKindsSprite) XGo_Init() *cloneAllFieldKindsSprite {
	s.boolValue = false
	s.intValue = 1
	s.stringValue = "init"
	s.floatValue = 2.5
	s.valueValue = NewValue("init-value")
	s.listValue.Init("init")
	s.sliceValue = []int{9}
	s.mapValue = map[string]int{"init": 1}
	s.structValue = cloneFieldPayload{label: "init-struct", count: 1}
	s.arrayValue = [2]string{"init-left", "init-right"}
	s.pointerValue = &cloneFieldPayload{label: "init-pointer", count: 2}
	s.interfaceValue = "init-any"
	return s
}

func (s *cloneProxyInitSprite) Main() {
	if !s.IsCloned() || !s.registerCloned {
		return
	}
	s.OnCloned__0(func() {
		// Query synchronization is allowed during initialization, but it must
		// not bypass the clone's render-visibility gate.
		s.syncProxyForQuery()
		s.SetCostume__0(s.finalCostume)
		if s.hideOnCloned {
			s.Hide()
		}
		if s.deleteOnCloned {
			s.DeleteThisClone()
			return
		}
		if s.stopOthers {
			s.Stop(AllOtherScripts)
			return
		}
		if s.waitOnCloned != nil {
			engine.WaitForChan(s.waitOnCloned)
			if s.onClonedReturned != nil {
				close(s.onClonedReturned)
			}
		}
	})
}

func (s *spyCloneSpriteMgr) CreateBareSprite(pos mathf.Vec2) pkgengine.Object {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	return s.nextID
}

func (s *spyCloneSpriteMgr) SetTypeName(obj pkgengine.Object, typeName string) {}

func (s *spyCloneSpriteMgr) DestroySprite(obj pkgengine.Object) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.zIndexes, obj)
	return true
}

func (s *spyCloneSpriteMgr) SetVisible(obj pkgengine.Object, visible bool) {
	s.record(cloneProxyOperation{kind: "visible", object: obj, visible: visible})
}

func (s *spyCloneSpriteMgr) SetTexture(obj pkgengine.Object, path string) {
	s.record(cloneProxyOperation{kind: "texture", object: obj, path: path})
}

func (s *spyCloneSpriteMgr) SetRenderScale(obj pkgengine.Object, scale mathf.Vec2) {}

func (s *spyCloneSpriteMgr) SetMaterialShader(obj pkgengine.Object, path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.shaderIDs = append(s.shaderIDs, obj)
}

func (s *spyCloneSpriteMgr) SetMaterialParamsVec(pkgengine.Object, string, float64, float64, float64, float64) {
}

func (s *spyCloneSpriteMgr) SetMaterialParams(
	obj pkgengine.Object, effect string, amount float64,
) {
}

func (s *spyCloneSpriteMgr) SetTransform(
	obj pkgengine.Object, pos mathf.Vec2, rot float64, scale mathf.Vec2, visible bool, pivot mathf.Vec2,
) {
	s.record(cloneProxyOperation{kind: "transform", object: obj, visible: visible})
}

func (s *spyCloneSpriteMgr) BatchUpdateTransforms(buffer []float32) {
	if len(buffer) < 2 {
		return
	}
	updateCount := int(buffer[0])
	idx := 2
	for range updateCount {
		if idx+engine.SyncFieldsPerSprite > len(buffer) {
			return
		}
		s.record(cloneProxyOperation{
			kind:    "transform",
			object:  pkgengine.Object(int64(buffer[idx])),
			visible: buffer[idx+engine.SyncFieldsPerSprite-1] != 0,
		})
		idx += engine.SyncFieldsPerSprite
	}
}

func (s *spyCloneSpriteMgr) SetZIndex(obj pkgengine.Object, z int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.zIndexes[obj] = z
}

func (s *spyCloneSpriteMgr) SetTriggerLayer(obj pkgengine.Object, layer int64) {}

func (s *spyCloneSpriteMgr) SetTriggerMask(obj pkgengine.Object, mask int64) {}

func (s *spyCloneSpriteMgr) SetCollisionLayer(obj pkgengine.Object, layer int64) {}

func (s *spyCloneSpriteMgr) SetCollisionMask(obj pkgengine.Object, mask int64) {}

func (s *spyCloneSpriteMgr) SetTriggerEnabled(obj pkgengine.Object, trigger bool) {}

func (s *spyCloneSpriteMgr) SetCollisionEnabled(obj pkgengine.Object, enabled bool) {}

func (s *spyCloneSpriteMgr) SetGravityScale(obj pkgengine.Object, scale float64) {}

func (s *spyCloneSpriteMgr) SetPhysicsMode(obj pkgengine.Object, mode int64) {}

func (s *spyCloneSpriteMgr) record(op cloneProxyOperation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.operations = append(s.operations, op)
}

func (s *spyCloneSpriteMgr) recordedOperations() []cloneProxyOperation {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]cloneProxyOperation(nil), s.operations...)
}

func (s *spyCloneSpriteMgr) assertNoVisibleWrites(t *testing.T) {
	t.Helper()
	operations := s.recordedOperations()
	for _, op := range operations {
		if (op.kind == "visible" || op.kind == "transform") && op.visible {
			t.Fatalf("unexpected visible write; operations = %#v", operations)
		}
	}
}

func (s *spyCloneSpriteMgr) zIndex(obj pkgengine.Object) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.zIndexes[obj]
}

func (s *cloneLimitSprite) Main() {
	if s.onInit != nil {
		s.onInit(s)
	}
	s.OnCloned__0(func() {
		if s.onClone != nil {
			s.onClone(s)
		}
	})
}

func (*layoutInvalidInitializer) InitFrom(string) {}

func (*layoutInvalidInitSprite) Main() {}

func (v *layoutCloneValue) InitFrom(src any) int {
	v.value = src.(*layoutCloneValue).value + 1
	return v.value
}

func (*layoutCloneSprite) Main() {}

func (v *layoutVariadicValue) InitFrom(src ...*layoutVariadicValue) { *v = *src[0] + 1 }

func (v *layoutOptionalValue) InitFrom(src *layoutOptionalValue, _ ...any) { *v = *src + 1 }

func newCloneAwakeOrderSprite(g *Game, name string) *cloneAwakeOrderSprite {
	sawAwake := false
	onCloned := false
	sprite := &cloneAwakeOrderSprite{
		sawAwakeInMain: &sawAwake,
		onClonedFired:  &onCloned,
		clonedDone:     make(chan struct{}),
	}
	sprite.g = g
	sprite.name = name
	sprite.sprite = sprite
	sprite.scriptEventBindings.bind(&g.scriptEvents, &sprite.SpriteImpl)
	sprite.components.initComponents(&sprite.SpriteImpl, &coreproject.SpriteConfig{})
	sprite.physics().collisionInfo.Type = physicsColliderNone
	sprite.physics().triggerInfo.Type = physicsColliderNone
	prepareAwakeCostumes(&sprite.SpriteImpl)
	return sprite
}

func newCloneStatePreservingSprite(g *Game, name string, recordValue *float64) *cloneStatePreservingSprite {
	sprite := &cloneStatePreservingSprite{
		recordValue: recordValue,
		clonedDone:  make(chan struct{}),
	}
	sprite.baseObj.initWithSize(1, 1)
	sprite.g = g
	sprite.name = name
	sprite.sprite = sprite
	sprite.scriptEventBindings.bind(&g.scriptEvents, &sprite.SpriteImpl)
	sprite.components.initComponents(&sprite.SpriteImpl, &coreproject.SpriteConfig{})
	sprite.physics().collisionInfo.Type = physicsColliderNone
	sprite.physics().triggerInfo.Type = physicsColliderNone
	return sprite
}

func newCloneAllFieldKindsSprite(g *Game, name string, observed *cloneAllFieldKindsObserved) *cloneAllFieldKindsSprite {
	sprite := &cloneAllFieldKindsSprite{
		observed:   observed,
		clonedDone: make(chan struct{}),
	}
	sprite.baseObj.initWithSize(1, 1)
	sprite.g = g
	sprite.name = name
	sprite.sprite = sprite
	sprite.scriptEventBindings.bind(&g.scriptEvents, &sprite.SpriteImpl)
	sprite.components.initComponents(&sprite.SpriteImpl, &coreproject.SpriteConfig{})
	sprite.physics().collisionInfo.Type = physicsColliderNone
	sprite.physics().triggerInfo.Type = physicsColliderNone
	return sprite
}

func newCloneProxyInitSprite(g *Game, registerCloned, hideOnCloned bool) *cloneProxyInitSprite {
	sprite := &cloneProxyInitSprite{
		finalCostume:   "final",
		hideOnCloned:   hideOnCloned,
		registerCloned: registerCloned,
	}
	placeholder := newCostumeWithSize(1, 1)
	placeholder.name, placeholder.path = "placeholder", "placeholder.png"
	final := newCostumeWithSize(1, 1)
	final.name, final.path = "final", "final.png"
	sprite.costumes = []*costume{placeholder, final}
	sprite.costumeIndex = 0
	sprite.runtimeState.scale = 1
	sprite.spriteState.isVisible = true
	sprite.g = g
	sprite.name = "Score"
	sprite.sprite = sprite
	sprite.scriptEventBindings.bind(&g.scriptEvents, &sprite.SpriteImpl)
	sprite.components.initComponents(&sprite.SpriteImpl, &coreproject.SpriteConfig{})
	sprite.physics().collisionInfo.Type = physicsColliderNone
	sprite.physics().triggerInfo.Type = physicsColliderNone
	return sprite
}

func setupCloneSpriteMgr(t *testing.T) *spyCloneSpriteMgr {
	t.Helper()

	setupImmediateEngineCalls(t)

	original := pkgengine.SpriteMgr
	mgr := &spyCloneSpriteMgr{zIndexes: make(map[pkgengine.Object]int64)}
	pkgengine.SpriteMgr = mgr
	t.Cleanup(func() {
		pkgengine.SpriteMgr = original
	})
	return mgr
}

func flushCloneProxyUpdates(game *Game) {
	if game.syncBuffer == nil {
		game.syncBuffer = engine.NewSpriteSyncBuffer(initialSpriteSyncBufferSize)
	}
	if game.camera == nil {
		game.camera = &cameraImpl{g: game}
	}
	game.flushSpriteProxyChanges(game.shapeMgr.getTempShapes())
}

func setupCloneLimitGame(t *testing.T) *Game {
	t.Helper()
	setupCloneSpriteMgr(t)
	game := new(Game)
	game.initShapeMgr()
	game.bindScriptEvents()
	original := engine.GetGame()
	engine.SetGame(game)
	t.Cleanup(func() { engine.SetGame(original) })
	setupRuntimeScheduler(t)
	return game
}

func newCloneLimitSprite(game *Game, name string) *cloneLimitSprite {
	s := &cloneLimitSprite{}
	s.baseObj.initWithSize(1, 1)
	s.g, s.name, s.sprite = game, name, s
	s.scriptEventBindings.bind(&game.scriptEvents, &s.SpriteImpl)
	s.components.initComponents(&s.SpriteImpl, &coreproject.SpriteConfig{})
	s.physics().collisionInfo.Type = physicsColliderNone
	s.physics().triggerInfo.Type = physicsColliderNone
	game.shapeMgr.add(&s.SpriteImpl)
	return s
}
