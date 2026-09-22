package spx

import "testing"

func TestCloneComponentsUseOriginalAndParentState(t *testing.T) {
	game := setupCloneLimitGame(t)
	game.soundMgr.Init(&cloneSoundBackend{})
	original := newCloneLimitSprite(game, "original")
	original.pen().penWidth = 5
	original.SetSoundEffect(SoundPanEffect, -40)
	parent := createRuntimeClone(&original.SpriteImpl)
	parent.SetXYpos(30, -15)
	parent.SetHeading(90)
	parent.physics().mass = 7
	parent.physics().collisionTargets["touching"] = true
	parent.pen().penWidth = 50
	parent.SetSoundEffect(SoundPanEffect, 80)

	// Give the parent distinct read-only configuration and per-instance runtime state.
	parentAnimation := parent.animation()
	parentAnimations := *parentAnimation.shared
	parentAnimations.defaultAnimation = "parent"
	parentAnimation.shared = &parentAnimations
	parentAnimation.curAnimState = &animState{Name: "playing"}
	parentAnimation.curTweenState = &animState{Name: "moving"}
	parentAnimation.activeTweenStates = []*animState{parentAnimation.curTweenState}
	parentAnimation.defaultAnimActive = true
	parentAnimation.doneAnimations = []string{"completed"}
	original.pen().penWidth = 7
	original.SetSoundEffect(SoundPanEffect, 20)

	child := createRuntimeClone(parent)
	if child.pen().penWidth != 7 || child.GetSoundEffect(SoundPanEffect) != 20 {
		t.Fatal("pen and sound did not inherit the current original state")
	}
	if child.Xpos() != 30 || child.Ypos() != -15 || child.Heading() != 90 {
		t.Fatalf("transform = (%v, %v, %v), want parent transform (30, -15, 90)", child.Xpos(), child.Ypos(), child.Heading())
	}
	if child.physics().mass != 7 {
		t.Fatalf("mass = %v, want parent mass 7", child.physics().mass)
	}
	childAnimation := child.animation()
	if childAnimation.shared != parentAnimation.shared {
		t.Fatal("animation configuration did not come from the parent")
	}
	if childAnimation.curAnimState != nil || childAnimation.curTweenState != nil ||
		len(childAnimation.activeTweenStates) != 0 || childAnimation.defaultAnimActive ||
		len(childAnimation.doneAnimations) != 0 || len(child.physics().collisionTargets) != 0 {
		t.Fatal("clone inherited parent collision or animation runtime state")
	}

	child.SetXpos(99)
	child.physics().mass = 9
	child.pen().penWidth = 9
	child.SetSoundEffect(SoundPanEffect, 90)
	if parent.Xpos() != 30 || parent.physics().mass != 7 || parent.pen().penWidth != 50 || parent.GetSoundEffect(SoundPanEffect) != 80 {
		t.Fatal("changing descendant state changed the parent")
	}
	if original.Xpos() != 0 || original.physics().mass != 1 || original.pen().penWidth != 7 || original.GetSoundEffect(SoundPanEffect) != 20 {
		t.Fatal("changing descendant state changed the original")
	}
}
