package spx

import (
	"reflect"
	"testing"
)

func TestSetLayerPublicAPI(t *testing.T) {
	spriteType := reflect.TypeOf((*Sprite)(nil)).Elem()
	if _, ok := spriteType.MethodByName("ChangeLayer"); ok {
		t.Fatal("Sprite should not expose ChangeLayer")
	}

	spriteImplType := reflect.TypeOf((*SpriteImpl)(nil))
	if _, ok := spriteImplType.MethodByName("ChangeLayer"); ok {
		t.Fatal("SpriteImpl should not expose ChangeLayer")
	}
	if _, ok := spriteImplType.MethodByName("SetLayer__1"); !ok {
		t.Fatal("SpriteImpl should keep SetLayer__1 for overload dispatch")
	}

	if XGoo_Sprite_SetLayerWith != ".SetLayerTo,.SetLayer__1" {
		t.Fatalf("XGoo_Sprite_SetLayerWith = %q, want %q", XGoo_Sprite_SetLayerWith, ".SetLayerTo,.SetLayer__1")
	}
	if XGoo_SpriteImpl_SetLayerWith != ".SetLayerTo,.SetLayer__1" {
		t.Fatalf("XGoo_SpriteImpl_SetLayerWith = %q, want %q", XGoo_SpriteImpl_SetLayerWith, ".SetLayerTo,.SetLayer__1")
	}
}
