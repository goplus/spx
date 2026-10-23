package spx

import (
	"math"
	"testing"

	"github.com/goplus/spbase/mathf"
	coreproject "github.com/goplus/spx/v4/internal/core/project"
	"github.com/goplus/spx/v4/internal/coroutine"
	itime "github.com/goplus/spx/v4/internal/time"
)

func TestCloneComponentsUseOriginalAndParentState(t *testing.T) {
	game := setupCloneLimitGame(t)
	game.soundMgr.Init(&cloneSoundBackend{})
	original := newCloneLimitSprite(game, "original")
	original.pen().penWidth = 5
	original.SetSoundEffect(SoundPanEffect, -40)
	parent := createRuntimeClone(&original.SpriteImpl)
	parent.SetXYpos(30, -15)
	parent.SetHeading(90)
	parent.SetPhysicsMode(DynamicPhysics)
	parent.physics().collisionTargets["touching"] = true
	parent.pen().penWidth = 50
	parent.SetSoundEffect(SoundPanEffect, 80)

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
	if parent.Xpos() != 30 || parent.PhysicsMode() != DynamicPhysics || parent.pen().penWidth != 50 || parent.GetSoundEffect(SoundPanEffect) != 80 {
		t.Fatal("changing descendant state changed the parent")
	}
	if original.Xpos() != 0 || original.PhysicsMode() != NoPhysics || original.pen().penWidth != 7 || original.GetSoundEffect(SoundPanEffect) != 20 {
		t.Fatal("changing descendant state changed the original")
	}
}

func TestCloneBubbleIsLazyAndIndependent(t *testing.T) {
	game := setupCloneLimitGame(t)
	original := newCloneLimitSprite(game, "original")
	originalBubble := original.bubble()
	originalText := &textBubble{bubbleBase: original.newBubbleBase()}
	originalBubble.textObj = originalText
	game.shapeMgr.add(originalText)

	clone := createRuntimeClone(&original.SpriteImpl)
	if clone.components.bubble != nil {
		t.Fatal("clone inherited an allocated bubble component")
	}
	if originalBubble.textObj != originalText {
		t.Fatal("cloning changed the original bubble")
	}

	cloneBubble := clone.bubble()
	if cloneBubble == originalBubble || cloneBubble.sprite != clone {
		t.Fatal("lazy bubble component is not owned by the clone")
	}
	cloneText := &textBubble{bubbleBase: clone.newBubbleBase()}
	cloneBubble.textObj = cloneText
	game.shapeMgr.add(cloneText)

	clone.Destroy()
	if cloneBubble.textObj != nil || cloneBubble.quoteObj != nil {
		t.Fatal("destroy left clone bubble state active")
	}
	for _, shape := range game.getAllShapes() {
		if shape == cloneText {
			t.Fatal("destroy left the clone bubble in the shape list")
		}
	}
	if originalBubble.textObj != originalText {
		t.Fatal("destroying the clone changed the original bubble")
	}
}

func TestStandaloneComponentsCloneWithoutRegistry(t *testing.T) {
	sourceSprite, target := &SpriteImpl{}, &SpriteImpl{}
	sound := &soundComponent{sprite: sourceSprite}
	clonedSound := sound.cloneFor(target)
	if clonedSound.sprite != target || clonedSound.soundObj != 0 {
		t.Fatal("standalone sound clone acquired resources")
	}
	pen := &penComponent{sprite: sourceSprite}
	pen.penWidth = 7
	clonedPen := pen.cloneFor(target)
	if clonedPen.sprite != target || clonedPen.penWidth != 7 {
		t.Fatal("standalone pen clone lost state")
	}
}

func TestPhysicsComponentRejectsMalformedConfiguredShapes(t *testing.T) {
	sprite := &SpriteImpl{g: &Game{}, name: "invalid"}
	component := &physicsComponent{}
	component.initialize(sprite, &coreproject.SpriteConfig{
		CollisionShapeType: "rect", CollisionShapeParams: []float64{1},
		TriggerShapeType: "circle", TriggerShapeParams: []float64{1, 2},
	})
	if component.collisionInfo.Type != physicsColliderNone || component.collisionInfo.Params != nil ||
		component.triggerInfo.Type != physicsColliderAuto || component.triggerInfo.Params != nil {
		t.Fatalf("invalid shapes were retained: collision=%+v trigger=%+v", component.collisionInfo, component.triggerInfo)
	}
	sprite.g.physicsEnabled = true
	sprite.g.isAutoSetCollisionLayer = true
	sprite.g.sprCollisionInfos = map[string]*spriteCollisionInfo{"invalid": {Layer: 4, Mask: 10}}
	component.initCollisionParams()
	if component.collisionInfo.Layer != 4 || component.collisionInfo.Mask != 10 || component.triggerInfo.Layer != 4 || component.triggerInfo.Mask != 10 {
		t.Fatal("automatic physical collision groups did not match the configured sprite groups")
	}
}

func TestTransformComponentScaleFallbacksAndTinyStage(t *testing.T) {
	sprite := newTestTransformSprite(0, 0)
	transform := sprite.transform()
	if got := transform.clampSpriteScale(3); got != 3 {
		t.Fatalf("unconfigured sprite scale = %v, want 3", got)
	}
	sprite.costumes = []*costume{newCostumeWithSize(0, 0)}
	if got := transform.clampSpriteScale(3); got != 3 {
		t.Fatalf("empty costume scale = %v, want 3", got)
	}
	sprite.costumes = []*costume{newCostumeWithSize(10, 10)}
	sprite.g.displayState.WorldWidth, sprite.g.displayState.WorldHeight = 1, 1
	if got := transform.clampSpriteScale(3); got != 0.5 {
		t.Fatalf("tiny stage scale = %v, want minimum visible scale 0.5", got)
	}
	transform.setSize(-2)
	if sprite.runtimeState.Scale != 0.5 {
		t.Fatalf("negative size = %v, want clamped visible size 0.5", sprite.runtimeState.Scale)
	}
}

func TestTransformComponentKeepsBottomFenceAndHeadingAtOwnPosition(t *testing.T) {
	sprite := newTestTransformSprite(0, 0)
	sprite.runtimeState.Scale = 1
	sprite.costumes = []*costume{newCostumeWithSize(20, 20)}
	sprite.g.displayState.WorldWidth, sprite.g.displayState.WorldHeight = 100, 100
	sprite.g.displayState.MinWorldX, sprite.g.displayState.MinWorldY = -50, -50
	transform := sprite.transform()
	if x, y := transform.fixWorldRange(0, -100); x != 0 || y != -50 {
		t.Fatalf("bottom fenced position = (%v, %v), want (0, -50)", x, y)
	}
	transform.direction = 35
	if got := transform.calculateTargetAngleToPos(0, 0); got != 35 {
		t.Fatalf("direction to current position = %v, want 35", got)
	}
	sprite.g.inputMgr.setMousePos(mathf.NewVec2(0, 10))
	if got := transform.directionToTarget(Mouse); !(math.Abs(got) <= 1e-9) {
		t.Fatalf("direction to mouse above sprite = %v, want 0", got)
	}
	transform.bounceOffEdge(edgeAreaStage)
	if transform.direction != 35 {
		t.Fatal("sprite away from edges changed its heading")
	}
}

func TestTransformComponentStepUsesConfiguredAnimationDuration(t *testing.T) {
	t.Cleanup(itime.ClearTimers)
	co := setupRuntimeScheduler(t)
	itime.Start(nil)
	sprite := newTestTransformSprite(0, 0)
	animation := &coreproject.AniConfig{StepDuration: 0.001}
	sprite.animation().shared.animations["walk"] = &animationEntry{name: "walk", config: animation}
	done := false
	thread := co.Create(sprite, func(coroutine.Thread) {
		sprite.transform().stepToPos(10, 0, 1, "walk")
		done = true
	})
	co.JoinFirstSlice(thread)
	if done {
		t.Fatal("configured step animation completed before advancing a frame")
	}
	for range 2 {
		itime.Update(1.0/30, 30)
		co.Update()
	}
	if x, y := sprite.getXY(); !done || x != 10 || y != 0 || sprite.animation().getCurTweenState() != nil {
		t.Fatalf("step completion = %v, position=(%v,%v), tween=%v", done, x, y, sprite.animation().getCurTweenState())
	}
	co.Join(thread)
}

func TestTransformComponentDebugVisualOperations(t *testing.T) {
	t.Cleanup(itime.ClearTimers)
	game := setupCloneLimitGame(t)
	game.debugState.DebugInstr = true
	sprite := newCloneLimitSprite(game, "moving")
	sprite.costumes = nil
	transform := sprite.transform()
	transform.setRotationStyle(LeftRight)
	transform.direction = 45
	transform.bounceOffEdge(edgeAreaStage)
	if transform.rotationStyle != LeftRight || transform.direction != 45 {
		t.Fatal("unbound sprite lost its rotation style or heading")
	}
	itime.Start(nil)
	done := false
	thread := gco.Create(sprite, func(coroutine.Thread) {
		transform.glide(5, 7, 0.01)
		done = true
	})
	gco.JoinFirstSlice(thread)
	for range 2 {
		itime.Update(1.0/30, 30)
		gco.Update()
	}
	if x, y := sprite.getXY(); !done || x != 5 || y != 7 {
		t.Fatalf("debug glide completed=%v, position=(%v,%v), want (5,7)", done, x, y)
	}
	gco.Join(thread)
}
