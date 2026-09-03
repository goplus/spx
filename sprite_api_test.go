package spx

import (
	"reflect"
	"testing"
	"time"

	"github.com/goplus/spx/v3/internal/coroutine"
)

func TestSetLayerPublicAPI(t *testing.T) {
	spriteType := reflect.TypeOf((*Sprite)(nil)).Elem()
	if _, ok := spriteType.MethodByName("ChangeLayer"); ok {
		t.Fatal("Sprite should not expose ChangeLayer")
	}
	if _, ok := spriteType.MethodByName("SetLayer__1"); !ok {
		t.Fatal("Sprite should keep SetLayer__1 for overload dispatch")
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

func TestAbortIfCurrentCoroutineOutsideCoroutine(t *testing.T) {
	if current := gco.Current(); current != nil {
		t.Fatalf("unexpected active coroutine before test: %v", current)
	}

	// Engine-driven destruction can run between coroutine turns. The scheduler
	// deliberately clears Current at that boundary, so this path must be a no-op
	// instead of dereferencing a stale or nil thread.
	(&SpriteImpl{}).abortIfCurrentCoroutine()
}

func TestAbortIfCurrentCoroutineExternalCallerDoesNotAbortActiveThread(t *testing.T) {
	co := setupRuntimeEventScheduler(t)
	sprite := &SpriteImpl{}
	started := make(chan struct{})
	release := make(chan struct{})
	thread := co.Create(sprite, func(coroutine.Thread) int {
		close(started)
		<-release
		return 0
	})
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("active coroutine did not start")
	}

	// This is an engine-side call from outside the managed coroutine. It must
	// not mistake the scheduler's Current value for the caller's coroutine.
	sprite.abortIfCurrentCoroutine()
	if thread.Stopped() {
		t.Fatal("external caller unexpectedly aborted the active coroutine")
	}
	close(release)
	co.Join(thread)
}
