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
	"math"
	"testing"

	"github.com/goplus/spbase/mathf"
	assetutil "github.com/goplus/spx/v4/internal/assets"
	"github.com/goplus/spx/v4/internal/base/collision"
	coreproject "github.com/goplus/spx/v4/internal/core/project"
	internalengine "github.com/goplus/spx/v4/internal/engine"
	"github.com/goplus/spx/v4/internal/enginewrap"
	itime "github.com/goplus/spx/v4/internal/time"
	pkgengine "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

type touchingSyncSpriteMgr struct {
	enginewrap.SpriteMgrImpl
	positions               map[pkgengine.Object]mathf.Vec2
	visible                 map[pkgengine.Object]bool
	texturePaths            map[pkgengine.Object]string
	expectedTexturePaths    map[pkgengine.Object]string
	checkedTexturePaths     map[pkgengine.Object]string
	setTransformCalls       map[pkgengine.Object]int
	setPositionCalls        map[pkgengine.Object]int
	setRotationCalls        map[pkgengine.Object]int
	setScaleCalls           map[pkgengine.Object]int
	setPivotCalls           map[pkgengine.Object]int
	setVisibleCalls         map[pkgengine.Object]int
	setTextureCalls         map[pkgengine.Object]int
	setTextureAtlasCalls    map[pkgengine.Object]int
	setRenderScaleCalls     map[pkgengine.Object]int
	setTextureDirectCalls   map[pkgengine.Object]int
	setTextureAtlasPathByID map[pkgengine.Object]string
	collisionChecks         int
	onCollision             func()
	collisionResult         func(pkgengine.Object, pkgengine.Object) bool
	onColorCollision        func()
	lastColorThreshold      float64
	lastAlphaThreshold      float64
	colorQueryResult        *bool
	panicVisibleTrue        any
	panicVisibleFalse       any
	panicCollision          any
}

type touchingQuerySprite struct {
	SpriteImpl
}

type cloneCollisionSpriteMgr struct {
	*spyCloneSpriteMgr
	visible     map[pkgengine.Object]bool
	ghost       map[pkgengine.Object]float64
	sensedGhost bool
}

type mouseSensingSpriteMgr struct {
	*touchingSyncSpriteMgr
	point       mathf.Vec2
	visibleOnly bool
}

func TestBoundsWithoutColliderPreserveFractionalCostumeGeometry(t *testing.T) {
	sprite := newTestTransformSprite(10, 20)
	sprite.costumes = []*costume{newCostume(&coreproject.CostumeConfig{
		ImageWidth:       10.5,
		ImageHeight:      6.25,
		X:                2,
		Y:                1.5,
		BitmapResolution: 2,
	})}
	sprite.runtimeState.Scale = 1.5
	sprite.transform().direction = 90
	sprite.transform().pivot = mathf.NewVec2(0.5, -0.25)
	sprite.physics().triggerInfo.Type = physicsColliderNone

	got := sprite.bounds()
	want := mathf.Rect2{
		Position: mathf.NewVec2(7.75, 16.8125),
		Size:     mathf.NewVec2(7.875, 4.6875),
	}
	if got == nil || *got != want {
		t.Fatalf("bounds() = %v, want %v", got, want)
	}
}

func TestPendingCloneSensingIsIndependentFromRenderPublication(t *testing.T) {
	mgr := newTouchingSyncSpriteMgr()
	installTouchingSyncSpriteMgr(t, mgr)

	receiver := newTouchingQuerySprite("receiver", 0, 0, 1)
	target := newTouchingQuerySprite("target", 0, 0, 2)
	mgr.positions[1] = mathf.NewVec2(0, 0)
	mgr.positions[2] = mathf.NewVec2(0, 0)

	target.beginCloneProxyPublication()
	mgr.visible[2] = false
	if !receiver.Touching__0(target) {
		t.Fatal("published sprite could not sense a logically visible pending clone")
	}
	if mgr.visible[2] {
		t.Fatal("pending target remained render-visible after sensing")
	}
	if mgr.collisionChecks != 1 {
		t.Fatalf("pending target collision checks = %d, want 1", mgr.collisionChecks)
	}

	target.proxyPublication = nil
	mgr.visible[2] = true
	receiver.beginCloneProxyPublication()
	mgr.visible[1] = false
	visibilityCalls := mgr.setVisibleCalls[1]
	if !receiver.Touching__0(target) {
		t.Fatal("pending clone could not sense a published sprite")
	}
	if mgr.visible[1] {
		t.Fatal("pending clone remained natively visible after sensing")
	}
	if got := mgr.setVisibleCalls[1] - visibilityCalls; got != 2 {
		t.Fatalf("pending sensing visibility calls = %d, want one show and one restore", got)
	}
	if !receiver.isCloneProxyPublicationBlocked() {
		t.Fatal("sensing published the pending clone")
	}
}

func TestPendingCloneSensingByNameIsIndependentFromRenderPublication(t *testing.T) {
	_, mgr, receiver, target := newTouchingPair(t, false)

	target.beginCloneProxyPublication()
	mgr.visible[2] = false
	if !receiver.Touching__1("target") {
		t.Fatal("named sensing could not observe a logically visible pending clone")
	}
	if mgr.visible[2] {
		t.Fatal("pending named target remained render-visible after sensing")
	}
	if mgr.collisionChecks != 1 {
		t.Fatalf("pending named target collision checks = %d, want 1", mgr.collisionChecks)
	}

	target.proxyPublication = nil
	mgr.visible[2] = true
	receiver.beginCloneProxyPublication()
	mgr.visible[1] = false
	if !receiver.Touching__1("target") {
		t.Fatal("pending clone could not sense a published named target")
	}
}

func TestLogicallyHiddenPendingCloneDoesNotAcquireSensingVisibility(t *testing.T) {
	mgr, receiver, target := newPendingCloneSensingPair(t)
	receiver.spriteState.IsVisible = false

	if receiver.Touching__0(target) {
		t.Fatal("logically hidden pending clone participated in sensing")
	}
	if got := mgr.setVisibleCalls[1]; got != 0 {
		t.Fatalf("hidden pending clone visibility calls = %d, want 0", got)
	}
	if mgr.collisionChecks != 0 {
		t.Fatalf("hidden pending clone collision checks = %d, want 0", mgr.collisionChecks)
	}
}

func TestPendingCloneSensingVisibilityLeaseIsReentrant(t *testing.T) {
	mgr, receiver, target := newPendingCloneSensingPair(t)

	var nestedTouching bool
	var hiddenBeforeOuterQueryReturned bool
	mgr.onCollision = func() {
		// Exercise the nested query-sync path: SetTransform must retain the
		// visibility owned by the outer lease.
		receiver.markProxyDirty()
		nestedTouching = receiver.Touching__0(target)
		hiddenBeforeOuterQueryReturned = !mgr.visible[1]
	}
	if !receiver.Touching__0(target) {
		t.Fatal("outer pending-clone sensing query returned false")
	}
	if !nestedTouching {
		t.Fatal("nested pending-clone sensing query returned false")
	}
	if hiddenBeforeOuterQueryReturned {
		t.Fatal("nested query released the outer visibility lease")
	}
	if mgr.visible[1] {
		t.Fatal("outer query did not restore pending clone visibility")
	}
	if got := mgr.setVisibleCalls[1]; got != 2 {
		t.Fatalf("nested sensing visibility calls = %d, want one show and one restore", got)
	}
	if mgr.collisionChecks != 2 {
		t.Fatalf("nested collision checks = %d, want 2", mgr.collisionChecks)
	}
}

func TestPendingCloneSensingShowPanicRestoresLease(t *testing.T) {
	mgr, receiver, target := newPendingCloneSensingPair(t)
	mgr.panicVisibleTrue = "temporary sensing show failed"

	var recovered any
	func() {
		defer func() { recovered = recover() }()
		receiver.Touching__0(target)
	}()
	if recovered != "temporary sensing show failed" {
		t.Fatalf("sensing panic = %v, want original show failure", recovered)
	}
	if mgr.visible[1] {
		t.Fatal("pending clone remained visible after partial show failure")
	}
	if got := receiver.proxyPublication.sensingVisibilityLeases.Load(); got != 0 {
		t.Fatalf("visibility leases after show failure = %d, want 0", got)
	}
	if !receiver.Touching__0(target) {
		t.Fatal("sensing lease did not recover after show failure")
	}
}

func TestPendingCloneSensingQueryPanicPreservesFailureAndRestoresVisibility(t *testing.T) {
	mgr, receiver, target := newPendingCloneSensingPair(t)
	mgr.panicCollision = "sensing query failed"
	mgr.panicVisibleFalse = "sensing visibility restore failed"

	var recovered any
	func() {
		defer func() { recovered = recover() }()
		receiver.Touching__0(target)
	}()
	if recovered != "sensing query failed" {
		t.Fatalf("sensing panic = %v, want query failure", recovered)
	}
	if mgr.visible[1] {
		t.Fatal("pending clone remained visible after query failure")
	}
	if mgr.panicVisibleFalse != nil {
		t.Fatal("query failure skipped visibility restoration")
	}
	if got := receiver.proxyPublication.sensingVisibilityLeases.Load(); got != 0 {
		t.Fatalf("visibility leases after query failure = %d, want 0", got)
	}
}

func TestTouchingSpriteSyncsDirtyTransformImmediately(t *testing.T) {
	mgr := newTouchingSyncSpriteMgr()
	installTouchingSyncSpriteMgr(t, mgr)

	mover := newTouchingTestSprite("mover", 0, 0, 1)
	target := newTouchingTestSprite("target", 10, 0, 2)

	mgr.positions[1] = mathf.NewVec2(0, 0)
	mgr.positions[2] = mathf.NewVec2(10, 0)

	mover.transform().direction = 90
	mover.Step__0(10)

	if !mover.touchingSprite(target) {
		t.Fatal("touchingSprite() = false, want true after immediate step sync")
	}
	if got := mgr.setTransformCalls[1]; got != 1 {
		t.Fatalf("SetTransform calls after first touching = %d, want 1", got)
	}
	if got := mgr.setPositionCalls[1]; got != 0 {
		t.Fatalf("SetPosition calls after first touching = %d, want 0", got)
	}
	if got := mgr.setRotationCalls[1]; got != 0 {
		t.Fatalf("SetRotation calls after first touching = %d, want 0", got)
	}
	if got := mgr.setScaleCalls[1]; got != 0 {
		t.Fatalf("SetScale calls after first touching = %d, want 0", got)
	}
	if got := mgr.setPivotCalls[1]; got != 0 {
		t.Fatalf("SetPivot calls after first touching = %d, want 0", got)
	}
	if got := mgr.setVisibleCalls[1]; got != 0 {
		t.Fatalf("SetVisible calls after first touching = %d, want 0", got)
	}

	if !mover.touchingSprite(target) {
		t.Fatal("second touchingSprite() = false, want cached proxy sync to remain valid")
	}
	if got := mgr.setTransformCalls[1]; got != 1 {
		t.Fatalf("SetTransform calls after second touching = %d, want 1", got)
	}

	buffer := internalengine.NewSpriteSyncBuffer(1)
	mover.collectProxyUpdate(buffer)
	if got := buffer.UpdateCount(); got != 0 {
		t.Fatalf("collectProxyUpdate() batched %d updates, want 0 after immediate query sync", got)
	}
}

func TestTouchingColorSyncsDirtyCostumeImmediately(t *testing.T) {
	mgr := newTouchingSyncSpriteMgr()
	installTouchingSyncSpriteMgr(t, mgr)

	sprite := newTouchingTestSprite("mover", 0, 0, 1)
	sprite.costumes = []*costume{
		{name: "idle", path: "idle.png", frame: assetutil.FrameDescriptor{BitmapResolution: 1, Width: 1, Height: 1}},
		{name: "hit", path: "hit.png", frame: assetutil.FrameDescriptor{BitmapResolution: 1, Width: 1, Height: 1}},
	}
	sprite.costumeIndex = 0
	sprite.runtimeState.IsCostumeDirty = false
	sprite.runtimeState.IsAnimating = false

	mgr.texturePaths[1] = internalengine.ToAssetPath("idle.png")
	mgr.expectedTexturePaths[1] = internalengine.ToAssetPath("hit.png")

	sprite.SetCostume__0("hit")

	if !sprite.touchingColor(mathf.Color{}) {
		t.Fatal("touchingColor() = false, want true after immediate costume sync")
	}
	if got := mgr.setTextureCalls[1]; got != 1 {
		t.Fatalf("SetTexture calls after first touchingColor = %d, want 1", got)
	}
	if got := mgr.checkedTexturePaths[1]; got != internalengine.ToAssetPath("hit.png") {
		t.Fatalf("collision used texture %q, want %q", got, internalengine.ToAssetPath("hit.png"))
	}
	if sprite.runtimeState.IsCostumeDirty {
		t.Fatal("runtimeState.IsCostumeDirty = true, want false after immediate query sync")
	}

	if !sprite.touchingColor(mathf.Color{}) {
		t.Fatal("second touchingColor() = false, want synced costume to remain valid")
	}
	if got := mgr.setTextureCalls[1]; got != 1 {
		t.Fatalf("SetTexture calls after second touchingColor = %d, want 1", got)
	}
}

func TestTouchingColorUsesConfiguredThreshold(t *testing.T) {
	mgr := newTouchingSyncSpriteMgr()
	installTouchingSyncSpriteMgr(t, mgr)
	sprite := newTouchingTestSprite("sensor", 0, 0, 1)

	result := true
	mgr.colorQueryResult = &result
	if !sprite.touchingColor(mathf.NewColorRGB(1, 0, 0)) {
		t.Fatal("touchingColor() = false, want the configured query result")
	}
	if got := mgr.lastColorThreshold; math.Abs(got-0.035) > 1e-12 {
		t.Fatalf("color threshold = %v, want 0.035", got)
	}
	if got := mgr.lastAlphaThreshold; math.Abs(got-alphaThreshold) > 1e-12 {
		t.Fatalf("alpha threshold = %v, want %v", got, alphaThreshold)
	}
}

func TestShortLivedGroundClonesRemainTouchableAcrossFrames(t *testing.T) {
	for _, front := range []string{"Player", "Ground"} {
		t.Run(front+"InFront", func(t *testing.T) {
			co, game := setupRuntimeEventGame(t)
			game.initShapeMgr()
			game.displayState.WorldWidth, game.displayState.WorldHeight = 480, 360
			mgr := &cloneCollisionSpriteMgr{
				spyCloneSpriteMgr: setupCloneSpriteMgr(t),
				visible:           make(map[pkgengine.Object]bool),
				ghost:             make(map[pkgengine.Object]float64),
			}
			pkgengine.SpriteMgr = mgr
			itime.Start()

			ground := newCloneLimitSprite(game, "Ground")
			player := newCloneLimitSprite(game, "Player")
			if front == "Ground" {
				game.shapeMgr.items[0], game.shapeMgr.items[1] = game.shapeMgr.items[1], game.shapeMgr.items[0]
			}
			for _, sprite := range []*cloneLimitSprite{ground, player} {
				sprite.runtimeState.Scale = 1
				sprite.Show()
				sprite.initRuntimeProxy()
			}
			player.SetXYpos(100, 0)
			if player.Touching__1("Ground") {
				t.Fatal("the original Ground must not overlap Player")
			}

			created, deleted := 0, 0
			ground.onClone = func(clone *cloneLimitSprite) {
				clone.SetXYpos(100, 0)
				clone.SetGraphicEffect(GhostEffect, 100)
				created++
				game.Wait(0.01)
				deleted++
				clone.DeleteThisClone()
			}
			ground.OnStart(func() { ground.Clone__0() })
			ground.OnStart(func() { Forever(func() { ground.Clone__0() }) })
			var samples, missedFrames []int64
			player.OnStart(func() {
				Forever(func() {
					frame := itime.Frame()
					touching := player.Touching__1("Ground")
					if frame > 0 {
						samples = append(samples, frame)
						if !touching {
							missedFrames = append(missedFrames, frame)
						}
					}
					internalengine.RequestRedraw()
				})
			})

			game.handleEvent(&eventStart{generation: game.bootstrapGeneration()})
			const frames = 12
			for range frames {
				co.Update()
				flushCloneProxyUpdates(game)
				itime.Update(1.0/30, 30)
			}
			if len(samples) != frames-1 {
				t.Fatalf("collision samples = %v, want one per frame after startup", samples)
			}
			if len(missedFrames) != 0 {
				t.Errorf("Player missed Ground clones at frames %v", missedFrames)
			}
			if created < frames || deleted < frames-1 {
				t.Errorf("created/deleted clones = %d/%d, want continuous replacement", created, deleted)
			}
			if !mgr.sensedGhost {
				t.Error("collision queries never sensed a clone with GhostEffect 100")
			}
		})
	}
}

func TestSpriteSensingWithoutProxyAndInvalidTargets(t *testing.T) {
	sprite := newTestTransformSprite(0, 0)
	if sprite.bounds() != nil {
		t.Fatal("sprite without costume has bounds")
	}
	if sprite.touchPoint(1, 2) || sprite.touchingColor(mathf.Color{}) || sprite.touchingColors(mathf.Color{}, mathf.Color{}) || sprite.touchingSprite(newTestTransformSprite(0, 0)) || sprite.checkTouchingScreen(int(Edge), edgeAreaStage) != 0 || sprite.checkNearestTouchedBoundary(edgeAreaStage) != 0 {
		t.Fatal("unbound sprite reported a collision")
	}
	if sprite.TouchingWith(&spriteWithoutImplementation{}) {
		t.Fatal("sprite without implementation participated in sensing")
	}
	target := &cloneLimitSprite{}
	target.spriteState.IsDying = true
	if sprite.TouchingWith(target) {
		t.Fatal("dying target participated in sensing")
	}
	sprite.spriteState.IsDying = true
	if sprite.TouchingWith("target") {
		t.Fatal("dying receiver participated in sensing")
	}
	sprite.spriteState.IsDying = false
	defer func() {
		if recover() == nil {
			t.Fatal("unsupported collision target did not panic")
		}
	}()
	sprite.TouchingWith(struct{}{})
}

func TestSpriteMouseSensingUsesWorldPointWithoutVisibilityFilter(t *testing.T) {
	game, mgr, sprite, _ := newTouchingPair(t, false)
	pointMgr := &mouseSensingSpriteMgr{touchingSyncSpriteMgr: mgr}
	pkgengine.SpriteMgr = pointMgr
	game.inputMgr.setMousePos(mathf.NewVec2(12, 34))
	sprite.Hide()
	if !sprite.TouchingWith(Mouse) || pointMgr.point != mathf.NewVec2(12, 34) || pointMgr.visibleOnly {
		t.Fatalf("mouse query point=%v visibleOnly=%v", pointMgr.point, pointMgr.visibleOnly)
	}
}

func TestPendingCloneSensingReportsCleanupFailureAndDoubleRelease(t *testing.T) {
	mgr := newTouchingSyncSpriteMgr()
	installTouchingSyncSpriteMgr(t, mgr)
	sprite := newTouchingQuerySprite("receiver", 0, 0, 1)
	sprite.beginCloneProxyPublication()
	mgr.panicVisibleFalse = "cleanup failure"
	failure := capturePendingCloneSensingPanic(func() { withPendingCloneSensingVisibility([]*SpriteImpl{&sprite.SpriteImpl}, func() {}) })
	if failure != "cleanup failure" || sprite.proxyPublication.sensingVisibilityLeases.Load() != 0 {
		t.Fatalf("cleanup failure=%v leases=%d", failure, sprite.proxyPublication.sensingVisibilityLeases.Load())
	}
	failure = capturePendingCloneSensingPanic(func() {
		releasePendingCloneSensingVisibility(pendingCloneSensingVisibilityLease{sprite: &sprite.SpriteImpl, publication: sprite.proxyPublication, proxy: sprite.runtimeState.SyncSprite})
	})
	if failure != "spx: negative pending clone sensing visibility lease count" {
		t.Fatalf("double release failure=%v", failure)
	}
}

func TestTouchingNameReadsCurrentState(t *testing.T) {
	game, mgr, receiver, target := newTouchingPair(t, false)
	target.SetXYpos(10, 0)
	for range 2 {
		if receiver.Touching__1("target") {
			t.Fatal("far target touched receiver")
		}
	}
	if mgr.collisionChecks != 0 {
		t.Fatalf("far target caused %d native checks", mgr.collisionChecks)
	}

	target.SetXYpos(0, 0)
	if !receiver.Touching__1("target") {
		t.Fatal("same-frame move was missed")
	}
	if got := mgr.positions[2]; got != (mathf.Vec2{}) {
		t.Fatalf("native target position = %v, want origin", got)
	}
	target.Hide()
	if receiver.Touching__1("target") {
		t.Fatal("hidden target touched receiver")
	}
	target.Show()
	if !receiver.Touching__1("target") {
		t.Fatal("same-frame show was missed")
	}
	game.shapeMgr.removeShape(&target.SpriteImpl)
	if receiver.Touching__1("target") {
		t.Fatal("removed target touched receiver")
	}
	game.shapeMgr.add(&target.SpriteImpl)
	if !receiver.Touching__1("target") {
		t.Fatal("re-added target was missed")
	}
}

func TestTouchingNameUsesRotatedCostume(t *testing.T) {
	_, mgr, receiver, target := newTouchingPair(t, false)
	receiver.SetXYpos(4, 0)
	target.costumes = []*costume{
		{name: "small", atlas: true, frame: assetutil.FrameDescriptor{Width: 2, Height: 10, BitmapResolution: 1}},
		{name: "wide", atlas: true, frame: assetutil.FrameDescriptor{Width: 10, Height: 2, BitmapResolution: 1}},
	}
	target.transform().direction = 90
	mgr.collisionResult = func(pkgengine.Object, pkgengine.Object) bool { return true }
	if receiver.Touching__1("target") {
		t.Fatal("unrotated narrow costume touched receiver")
	}
	target.transform().direction = 0
	target.markProxyDirty()
	if !receiver.Touching__1("target") {
		t.Fatal("rotated costume overlap was culled")
	}
	target.transform().direction = 90
	target.markProxyDirty()
	if receiver.Touching__1("target") {
		t.Fatal("unrotated costume unexpectedly touched receiver")
	}
	target.SetCostume__0("wide")
	if !receiver.Touching__1("target") {
		t.Fatal("same-frame costume enlargement was missed")
	}
	target.SetCostume__0("small")
	target.SetSize(5)
	if !receiver.Touching__1("target") {
		t.Fatal("same-frame scale change was missed")
	}
}

func TestTouchingNameUsesPhysicsShape(t *testing.T) {
	_, mgr, receiver, target := newTouchingPair(t, true)
	receiver.physics().collisionInfo = physicConfig{Type: physicsColliderRect, Params: []float64{1, 1}}
	target.physics().collisionInfo = physicConfig{Type: physicsColliderRect, Params: []float64{2, 10}}
	receiver.SetXYpos(4, 0)
	target.transform().direction = 90
	mgr.collisionResult = func(pkgengine.Object, pkgengine.Object) bool { return true }
	if receiver.Touching__1("target") {
		t.Fatal("unrotated collider touched receiver")
	}
	target.transform().direction = 0
	target.markProxyDirty()
	if !receiver.Touching__1("target") {
		t.Fatal("rotated collider overlap was culled")
	}
	target.physics().collisionInfo = physicConfig{
		Type:   physicsColliderPolygon,
		Params: []float64{-5, -1, 5, -1, 0, 1},
	}
	target.transform().direction = 90
	target.markProxyDirty()
	if !receiver.Touching__1("target") {
		t.Fatal("polygon overlap was culled")
	}
}

func TestTouchingNameFallsBackWithoutCostume(t *testing.T) {
	for _, tc := range []struct {
		name         string
		breakCostume func(*touchingQuerySprite)
	}{
		{"missing", func(sp *touchingQuerySprite) { sp.costumes = nil }},
		{"negative index", func(sp *touchingQuerySprite) { sp.costumeIndex = -1 }},
		{"past end", func(sp *touchingQuerySprite) { sp.costumeIndex = len(sp.costumes) }},
		{"nil costume", func(sp *touchingQuerySprite) { sp.costumes[0] = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, mgr, receiver, target := newTouchingPair(t, true)
			target.physics().collisionInfo = physicConfig{Type: physicsColliderRect, Params: []float64{2, 2}}
			target.runtimeState.IsCostumeDirty = false
			tc.breakCostume(target)
			mgr.collisionResult = func(pkgengine.Object, pkgengine.Object) bool { return true }
			if _, bounded := target.queryBounds(false); bounded {
				t.Fatal("invalid costume produced a query bound")
			}
			if !receiver.Touching__1("target") {
				t.Fatal("native collision fallback was skipped")
			}
		})
	}
}

func TestPhysicsQueryBoundsRespectRotationStyle(t *testing.T) {
	sp := newTouchingQuerySprite("target", 100, 50, 2)
	sp.runtimeState.Scale = 2
	sp.physics().collisionInfo = physicConfig{
		Type:   physicsColliderRect,
		Pivot:  mathf.NewVec2(10, 2),
		Params: []float64{2, 4},
	}
	for _, tc := range []struct {
		name  string
		style RotationStyle
		want  collision.AABB
	}{
		{"left right", LeftRight, collision.AABB{MinX: 78, MinY: 50, MaxX: 82, MaxY: 58}},
		{"none", None, collision.AABB{MinX: 118, MinY: 50, MaxX: 122, MaxY: 58}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sp.transform().rotationStyle = tc.style
			sp.transform().direction = -30
			got, bounded := sp.queryBounds(false)
			if !bounded || got != tc.want {
				t.Fatalf("physics query bounds = %+v, %t; want %+v, true", got, bounded, tc.want)
			}
		})
	}
}

func TestTouchingNameSeesCloneImmediately(t *testing.T) {
	game, mgr, receiver, target := newTouchingPair(t, false)
	target.SetXYpos(20, 0)
	clone := newTouchingQuerySprite("target", 0, 0, 3)
	clone.g = game
	clone.spriteState.Cloned = true
	mgr.positions[3] = mathf.NewVec2(0, 0)
	game.shapeMgr.add(&clone.SpriteImpl)
	if !receiver.Touching__1("target") {
		t.Fatal("new clone was missed")
	}
	game.shapeMgr.removeShape(&clone.SpriteImpl)
	if receiver.Touching__1("target") {
		t.Fatal("removed clone still touched receiver")
	}
}

func TestTouchingNameFollowsShapeOrder(t *testing.T) {
	game, mgr, receiver, target := newTouchingPair(t, false)
	mgr.collisionResult = func(pkgengine.Object, pkgengine.Object) bool { return true }
	first := func(want *SpriteImpl) {
		t.Helper()
		if got := game.touchingSpriteBy(&receiver.SpriteImpl, "target"); got != want {
			t.Fatalf("first touching sprite = %p, want %p", got, want)
		}
	}
	first(&target.SpriteImpl)
	clone := newTouchingQuerySprite("target", 0, 0, 3)
	clone.g = game
	clone.spriteState.Cloned = true
	game.shapeMgr.addClonedShape(&target.SpriteImpl, &clone.SpriteImpl)
	first(&clone.SpriteImpl)
	game.shapeMgr.activateShape(&clone.SpriteImpl)
	first(&target.SpriteImpl)
	game.shapeMgr.goBackLayers(&target.SpriteImpl, -1)
	first(&clone.SpriteImpl)
	game.shapeMgr.removeShape(&target.SpriteImpl)
	first(&clone.SpriteImpl)
	game.shapeMgr.init()
	if game.shapeMgr.named != nil {
		t.Fatal("scene reset retained the name index")
	}
	first(nil)
	game.shapeMgr.spritesNamed("missing-a")
	game.shapeMgr.spritesNamed("missing-b")
	if game.shapeMgr.named != nil {
		t.Fatal("empty scene allocated a name index")
	}
}

func BenchmarkTouchingName(b *testing.B) {
	for _, tc := range []struct {
		name    string
		start   int
		rebuild bool
	}{
		{"none", 300, false},
		{"first", 0, false},
		{"last", 299, false},
		{"all", -1, false},
		{"none/rebuild", 300, true},
		{"last/rebuild", 299, true},
	} {
		b.Run(tc.name, func(b *testing.B) {
			mgr := newTouchingSyncSpriteMgr()
			installTouchingSyncSpriteMgr(b, mgr)
			game := &Game{}
			game.initShapeMgr()
			receiver := newTouchingQuerySprite("receiver", 0, 0, 1)
			receiver.g = game
			game.shapeMgr.add(&receiver.SpriteImpl)
			mgr.positions[1] = mathf.NewVec2(0, 0)
			for i := range 300 {
				name := "other"
				if i == tc.start || tc.start == -1 {
					name = "target"
				}
				target := newTouchingQuerySprite(name, float64(1000+i*10), 0, int64(i+2))
				target.g = game
				game.shapeMgr.add(&target.SpriteImpl)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if tc.rebuild {
					clear(game.shapeMgr.named)
				}
				if receiver.Touching__1("target") {
					b.Fatal("unexpected collision")
				}
			}
		})
	}
}

func TestTouchingNameUsesMinimumCircleRadius(t *testing.T) {
	for _, tc := range []struct {
		name          string
		radius, scale float64
	}{
		{"small radius", 0.001, 1},
		{"small scale", 1, 0.001},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, mgr, receiver, target := newTouchingPair(t, true)
			for _, sp := range []*touchingQuerySprite{receiver, target} {
				sp.physics().collisionInfo = physicConfig{Type: physicsColliderCircle, Params: []float64{tc.radius}}
				sp.runtimeState.Scale = tc.scale
			}
			target.SetXYpos(0.015, 0)
			mgr.collisionResult = func(pkgengine.Object, pkgengine.Object) bool { return true }
			if !receiver.Touching__1("target") {
				t.Fatal("minimum-radius circle overlap was culled")
			}
		})
	}
}

func TestCollisionQueriesRejectUnavailableOrInvalidGeometry(t *testing.T) {
	game, _, receiver, _ := newTouchingPair(t, true)
	if game.touchingSpriteBy(nil, "target") != nil || game.touchingSpriteBy(&SpriteImpl{}, "target") != nil {
		t.Fatal("unbound receiver participated in collision query")
	}
	sprite := &receiver.SpriteImpl
	for _, cfg := range []physicConfig{
		{Type: physicsColliderRect, Params: []float64{0, 1}},
		{Type: physicsColliderCircle, Params: []float64{0}},
		{Type: physicsColliderPolygon, Params: []float64{0, 0, 1, 1}},
		{Type: physicsColliderPolygon, Params: []float64{0, 0, 1, 1, 2, 2, 3}},
	} {
		sprite.physics().collisionInfo = cfg
		if _, bounded := sprite.queryBounds(false); bounded {
			t.Errorf("invalid geometry %v produced usable bounds", cfg)
		}
	}
	sprite.physics().collisionInfo = physicConfig{Type: physicsColliderCapsule, Params: []float64{3, 2}}
	sprite.runtimeState.Scale = 1
	sprite.transform().direction = 90
	box, bounded := sprite.queryBounds(false)
	if !bounded || box != (collision.AABB{MinX: -3, MinY: -3, MaxX: 3, MaxY: 3}) {
		t.Fatalf("short capsule bounds = %v, %v, want diameter-sized square", box, bounded)
	}
	sprite.costumes = nil
	if _, bounded := sprite.queryBounds(true); bounded {
		t.Fatal("pixel bounds without costume should be unavailable")
	}
}

func (s *touchingSyncSpriteMgr) SetTransform(
	obj pkgengine.Object,
	pos mathf.Vec2,
	rot float64,
	scale mathf.Vec2,
	visible bool,
	pivot mathf.Vec2,
) {
	s.positions[obj] = pos
	s.visible[obj] = visible
	s.setTransformCalls[obj]++
}

func (s *touchingSyncSpriteMgr) SetPosition(obj pkgengine.Object, pos mathf.Vec2) {
	s.positions[obj] = pos
	s.setPositionCalls[obj]++
}

func (s *touchingSyncSpriteMgr) SetRotation(obj pkgengine.Object, rot float64) {
	s.setRotationCalls[obj]++
}

func (s *touchingSyncSpriteMgr) SetScale(obj pkgengine.Object, scale mathf.Vec2) {
	s.setScaleCalls[obj]++
}

func (s *touchingSyncSpriteMgr) SetPivot(obj pkgengine.Object, pivot mathf.Vec2) {
	s.setPivotCalls[obj]++
}

func (s *touchingSyncSpriteMgr) SetVisible(obj pkgengine.Object, visible bool) {
	s.visible[obj] = visible
	s.setVisibleCalls[obj]++
	if visible && s.panicVisibleTrue != nil {
		failure := s.panicVisibleTrue
		s.panicVisibleTrue = nil
		panic(failure)
	}
	if !visible && s.panicVisibleFalse != nil {
		failure := s.panicVisibleFalse
		s.panicVisibleFalse = nil
		panic(failure)
	}
}

func (s *touchingSyncSpriteMgr) SetTexture(obj pkgengine.Object, path string) {
	s.texturePaths[obj] = path
	s.setTextureCalls[obj]++
}

func (s *touchingSyncSpriteMgr) SetTextureAtlas(obj pkgengine.Object, path string, rect2 mathf.Rect2) {
	s.texturePaths[obj] = path
	s.setTextureAtlasPathByID[obj] = path
	s.setTextureAtlasCalls[obj]++
}

func (s *touchingSyncSpriteMgr) SetTextureDirect(obj pkgengine.Object, path string) {
	s.texturePaths[obj] = path
	s.setTextureDirectCalls[obj]++
}

func (s *touchingSyncSpriteMgr) SetRenderScale(obj pkgengine.Object, scale mathf.Vec2) {
	s.setRenderScaleCalls[obj]++
}

func (s *touchingSyncSpriteMgr) SetMaterialShader(obj pkgengine.Object, path string) {}

func (s *touchingSyncSpriteMgr) SetZIndex(pkgengine.Object, int64) {}

func (s *touchingSyncSpriteMgr) SetMaterialParamsVec(pkgengine.Object, string, float64, float64, float64, float64) {
}

func (s *touchingSyncSpriteMgr) SetTriggerEnabled(pkgengine.Object, bool) {}

func (s *touchingSyncSpriteMgr) SetCollisionEnabled(pkgengine.Object, bool) {}

func (s *touchingSyncSpriteMgr) SetMaterialParamsVec4(obj pkgengine.Object, effect string, vec4 mathf.Vec4) {
}

func (s *touchingSyncSpriteMgr) CheckCollisionWithSprite(obj, objB pkgengine.Object, alphaThreshold float64, usePixelPerfect bool) bool {
	s.collisionChecks++
	if call := s.onCollision; call != nil {
		s.onCollision = nil
		call()
	}
	if s.panicCollision != nil {
		failure := s.panicCollision
		s.panicCollision = nil
		panic(failure)
	}
	if !s.visible[obj] || !s.visible[objB] {
		return false
	}
	if s.collisionResult != nil {
		return s.collisionResult(obj, objB)
	}
	a, b := s.positions[obj], s.positions[objB]
	return math.Abs(a.X-b.X) < 1e-9 && math.Abs(a.Y-b.Y) < 1e-9
}

func (s *touchingSyncSpriteMgr) CheckCollisionByColor(obj pkgengine.Object, color mathf.Color, colorThreshold, alphaThreshold float64) bool {
	if s.onColorCollision != nil {
		s.onColorCollision()
	}
	s.lastColorThreshold = colorThreshold
	s.lastAlphaThreshold = alphaThreshold
	s.checkedTexturePaths[obj] = s.texturePaths[obj]
	if s.colorQueryResult != nil {
		return *s.colorQueryResult
	}
	expected, ok := s.expectedTexturePaths[obj]
	return !ok || s.texturePaths[obj] == expected
}

func (s *touchingSyncSpriteMgr) CheckCollisionByColors(obj pkgengine.Object, spriteColor, targetColor mathf.Color, colorThreshold, alphaThreshold float64) bool {
	return s.CheckCollisionByColor(obj, targetColor, colorThreshold, alphaThreshold)
}

func (*touchingQuerySprite) Main() {}

func (m *cloneCollisionSpriteMgr) SetVisible(id pkgengine.Object, visible bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.visible[id] = visible
}

func (m *cloneCollisionSpriteMgr) SetTransform(id pkgengine.Object, _ mathf.Vec2, _ float64, _ mathf.Vec2, visible bool, _ mathf.Vec2) {
	m.SetVisible(id, visible)
}

func (m *cloneCollisionSpriteMgr) SetMaterialParams(id pkgengine.Object, effect string, amount float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if effect == GhostEffect.String() {
		m.ghost[id] = amount
	}
}

func (m *cloneCollisionSpriteMgr) BatchUpdateTransforms(buffer []float32) {
	m.mu.Lock()
	defer m.mu.Unlock()
	idx := 2
	for range int(buffer[0]) {
		m.visible[pkgengine.Object(buffer[idx])] = buffer[idx+internalengine.SyncFieldsPerSprite-1] != 0
		idx += internalengine.SyncFieldsPerSprite
	}
	for range int(buffer[1]) {
		id := pkgengine.Object(buffer[idx])
		delete(m.visible, id)
		delete(m.ghost, id)
		idx++
	}
}

func (m *cloneCollisionSpriteMgr) CheckCollisionWithSprite(a, b pkgengine.Object, _ float64, _ bool) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Broad phase checked overlap; ghost does not affect sensing.
	if !m.visible[a] || !m.visible[b] {
		return false
	}
	if m.ghost[a] == 1 || m.ghost[b] == 1 {
		m.sensedGhost = true
	}
	return true
}

func (m *mouseSensingSpriteMgr) CheckCollisionWithPoint(_ pkgengine.Object, p mathf.Vec2, visibleOnly bool) bool {
	m.point = p
	m.visibleOnly = visibleOnly
	return true
}

func newTouchingSyncSpriteMgr() *touchingSyncSpriteMgr {
	return &touchingSyncSpriteMgr{
		positions:               map[pkgengine.Object]mathf.Vec2{},
		visible:                 map[pkgengine.Object]bool{},
		texturePaths:            map[pkgengine.Object]string{},
		expectedTexturePaths:    map[pkgengine.Object]string{},
		checkedTexturePaths:     map[pkgengine.Object]string{},
		setTransformCalls:       map[pkgengine.Object]int{},
		setPositionCalls:        map[pkgengine.Object]int{},
		setRotationCalls:        map[pkgengine.Object]int{},
		setScaleCalls:           map[pkgengine.Object]int{},
		setPivotCalls:           map[pkgengine.Object]int{},
		setVisibleCalls:         map[pkgengine.Object]int{},
		setTextureCalls:         map[pkgengine.Object]int{},
		setTextureAtlasCalls:    map[pkgengine.Object]int{},
		setRenderScaleCalls:     map[pkgengine.Object]int{},
		setTextureDirectCalls:   map[pkgengine.Object]int{},
		setTextureAtlasPathByID: map[pkgengine.Object]string{},
	}
}

func installTouchingSyncSpriteMgr(t testing.TB, mgr *touchingSyncSpriteMgr) {
	t.Helper()

	setupImmediateEngineCalls(t)

	original := pkgengine.SpriteMgr
	pkgengine.SpriteMgr = mgr
	t.Cleanup(func() {
		pkgengine.SpriteMgr = original
	})
}

func newTouchingTestSprite(name string, x, y float64, id pkgengine.Object) *SpriteImpl {
	sprite := newTestTransformSprite(x, y)
	sprite.name = name
	sprite.g.displayState.WorldWidth = 480
	sprite.g.displayState.WorldHeight = 360
	sprite.baseObj.initWithSize(1, 1)
	sprite.runtimeState.Scale = 1
	sprite.spriteState.IsVisible = true
	sprite.runtimeState.SyncSprite = &internalengine.Sprite{}
	sprite.runtimeState.SyncSprite.SetId(id)
	if mgr, ok := pkgengine.SpriteMgr.(*touchingSyncSpriteMgr); ok {
		mgr.visible[id] = true
	}
	return sprite
}

func newTouchingQuerySprite(name string, x, y float64, id pkgengine.Object) *touchingQuerySprite {
	sprite := &touchingQuerySprite{}
	sprite.g = &Game{}
	sprite.name = name
	sprite.components.initComponents(&sprite.SpriteImpl, &coreproject.SpriteConfig{
		X:             x,
		Y:             y,
		RotationStyle: "normal",
		FAnimations:   map[SpriteAnimationName]*coreproject.AniConfig{},
		AnimBindings:  map[string]string{},
	})
	sprite.g.displayState.WorldWidth = 480
	sprite.g.displayState.WorldHeight = 360
	sprite.baseObj.initWithSize(1, 1)
	sprite.runtimeState.Scale = 1
	sprite.spriteState.IsVisible = true
	sprite.runtimeState.SyncSprite = &internalengine.Sprite{}
	sprite.runtimeState.SyncSprite.SetId(id)
	sprite.sprite = sprite
	if mgr, ok := pkgengine.SpriteMgr.(*touchingSyncSpriteMgr); ok {
		mgr.visible[id] = true
	}
	return sprite
}

func newPendingCloneSensingPair(t *testing.T) (*touchingSyncSpriteMgr, *touchingQuerySprite, *touchingQuerySprite) {
	t.Helper()
	mgr := newTouchingSyncSpriteMgr()
	installTouchingSyncSpriteMgr(t, mgr)
	receiver := newTouchingQuerySprite("receiver", 0, 0, 1)
	target := newTouchingQuerySprite("target", 0, 0, 2)
	receiver.beginCloneProxyPublication()
	mgr.positions[1] = mathf.NewVec2(0, 0)
	mgr.positions[2] = mathf.NewVec2(0, 0)
	mgr.visible[1] = false
	return mgr, receiver, target
}

func newTouchingPair(t *testing.T, physics bool) (*Game, *touchingSyncSpriteMgr, *touchingQuerySprite, *touchingQuerySprite) {
	t.Helper()
	mgr := newTouchingSyncSpriteMgr()
	installTouchingSyncSpriteMgr(t, mgr)
	game := &Game{physicsEnabled: physics}
	game.initShapeMgr()
	receiver := newTouchingQuerySprite("receiver", 0, 0, 1)
	target := newTouchingQuerySprite("target", 0, 0, 2)
	receiver.g, target.g = game, game
	game.shapeMgr.add(&receiver.SpriteImpl)
	game.shapeMgr.add(&target.SpriteImpl)
	mgr.positions[1] = mathf.NewVec2(0, 0)
	mgr.positions[2] = mathf.NewVec2(0, 0)
	return game, mgr, receiver, target
}
