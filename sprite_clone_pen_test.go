package spx

import (
	"reflect"
	"testing"

	"github.com/goplus/spx/v3/internal/engine"
)

func TestClonePenStateComesFromOriginal(t *testing.T) {
	game := setupCloneLimitGame(t)
	original := newCloneLimitSprite(game, "original")
	original.pen().penState = penState{
		penColor:        toMathfColor(HSB(20, 80, 90)),
		penWidth:        5,
		penHue:          20,
		legacyPenColor:  legacyPenState{hue: 20, shade: 40},
		penSaturation:   80,
		penBrightness:   90,
		penTransparency: 30,
		isPenDown:       true,
	}
	penObj := engine.Object(42)
	original.pen().penObj = &penObj
	parent := createRuntimeClone(&original.SpriteImpl)
	parent.pen().penWidth = 50
	parent.pen().penHue = 80
	parent.pen().isPenDown = false
	original.pen().penWidth = 7
	child := createRuntimeClone(parent)
	if got, want := child.pen().penState, original.pen().penState; got != want {
		t.Fatalf("descendant pen state = %+v, want current original state %+v", got, want)
	}
	if child.pen().penObj != nil {
		t.Fatal("cloning allocated or shared a drawing resource")
	}
	child.pen().penWidth = 9
	if original.pen().penWidth != 7 || parent.pen().penWidth != 50 {
		t.Fatal("clone pen state aliases an ancestor")
	}
}

func TestClonePenStateFromStageInstance(t *testing.T) {
	game := setupCloneLimitGame(t)
	template := newCloneLimitSprite(game, "template")
	template.pen().penWidth = 5
	var instance cloneLimitSprite
	instantiateStageSprite(reflect.ValueOf(&instance).Elem(), template, spriteProperties{})
	game.shapeMgr.add(&instance.SpriteImpl)
	instance.pen().penWidth = 10
	child := createRuntimeClone(&instance.SpriteImpl)
	if child.pen().penWidth != 10 {
		t.Fatal("clone used template state instead of its original instance")
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
