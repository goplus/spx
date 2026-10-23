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

package engine

import (
	"math"
	"slices"
	"testing"

	"github.com/goplus/spx/v4/internal/coroutine"
	gdx "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

func TestManagersReturnsStableSingleton(t *testing.T) {
	first := Managers()
	if first == nil {
		t.Fatal("Managers() returned nil")
	}
	if got := Managers(); got != first {
		t.Fatalf("Managers() = %p, want stable singleton %p", got, first)
	}
}

func TestBridgeGetSpriteChecksConcreteType(t *testing.T) {
	previous := state
	t.Cleanup(func() { state = previous })
	sprite := &Sprite{}
	other := &gdx.Sprite{}
	state = runtimeState{sprites: map[Object]gdx.ISpriter{1: sprite, 2: other}}

	for _, tc := range []struct {
		name string
		id   Object
		want *Sprite
	}{
		{"engine sprite", 1, sprite},
		{"other implementation", 2, nil},
		{"missing sprite", 3, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := GetSprite(tc.id); got != tc.want {
				t.Fatalf("GetSprite(%d) = %p, want %p", tc.id, got, tc.want)
			}
		})
	}
	if got := (runtimeBridge{}).GetSprite(2); got != other {
		t.Fatalf("runtime bridge GetSprite(2) = %v, want original implementation", got)
	}
}

type bridgeMainThreadPlatform struct{ gdx.IPlatformMgr }

func (bridgeMainThreadPlatform) IsMainThread() bool { return true }

type bridgeDestroySprite struct {
	gdx.ISpriter
	calls int
}

func (s *bridgeDestroySprite) Destroy() bool {
	s.calls++
	return true
}

type bridgeDestroyNode struct {
	gdx.IUiNode
	calls int
}

func (n *bridgeDestroyNode) Destroy() bool {
	n.calls++
	return true
}

func TestBridgeClearAllSpritesDestroysAndUnregistersNodes(t *testing.T) {
	previousState, previousCo, previousPlatform := state, gco, gdx.PlatformMgr
	t.Cleanup(func() {
		state = previousState
		SetCoroutines(previousCo)
		gdx.PlatformMgr = previousPlatform
	})
	SetCoroutines(coroutine.New(nil))
	gdx.PlatformMgr = bridgeMainThreadPlatform{}

	for _, tc := range []struct {
		name  string
		clear func()
	}{
		{"public bridge", ClearAllSprites},
		{"runtime bridge", runtimeBridge{}.ClearAllSprites},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sprite, node := &bridgeDestroySprite{}, &bridgeDestroyNode{}
			state = runtimeState{
				sprites: map[Object]gdx.ISpriter{1: sprite},
				uiNodes: map[Object]gdx.IUiNode{2: node},
			}
			tc.clear()
			if len(state.sprites) != 0 || len(state.uiNodes) != 0 {
				t.Fatalf("remaining sprites/nodes = %d/%d, want 0/0", len(state.sprites), len(state.uiNodes))
			}
			// Clearing an already-empty registry must not destroy nodes again.
			tc.clear()
			if sprite.calls != 1 || node.calls != 1 {
				t.Fatalf("sprite/node Destroy calls = %d/%d, want 1/1", sprite.calls, node.calls)
			}
		})
	}
}

func TestFloatSliceConversions(t *testing.T) {
	t.Run("nil and empty", func(t *testing.T) {
		if F64Tof32(nil) != nil || F32Tof64(nil) != nil {
			t.Fatal("conversion did not preserve nil")
		}
		if got := F64Tof32([]float64{}); got == nil || len(got) != 0 {
			t.Fatalf("F64Tof32(empty) = %#v, want non-nil empty slice", got)
		}
		if got := F32Tof64([]float32{}); got == nil || len(got) != 0 {
			t.Fatalf("F32Tof64(empty) = %#v, want non-nil empty slice", got)
		}
	})
	t.Run("values and ownership", func(t *testing.T) {
		input := []float64{-2.5, 0, 1.25}
		narrowed := F64Tof32(input)
		if !slices.Equal(narrowed, []float32{-2.5, 0, 1.25}) {
			t.Fatalf("F64Tof32 = %v", narrowed)
		}
		widened := F32Tof64(narrowed)
		if !slices.Equal(widened, input) {
			t.Fatalf("F32Tof64 = %v, want %v", widened, input)
		}
		narrowed[0] = 7
		widened[1] = 8
		if input[0] != -2.5 || input[1] != 0 || widened[0] != -2.5 || narrowed[1] != 0 {
			t.Fatal("conversion shared mutable storage")
		}
	})
	t.Run("IEEE values", func(t *testing.T) {
		input := []float64{math.Copysign(0, -1), math.Inf(1), math.Inf(-1), math.NaN(), math.MaxFloat64, -math.MaxFloat64}
		got := F32Tof64(F64Tof32(input))
		if len(got) != len(input) {
			t.Fatalf("converted length = %d, want %d", len(got), len(input))
		}
		if got[0] != 0 || !math.Signbit(got[0]) || !math.IsInf(got[1], 1) || !math.IsInf(got[2], -1) || !math.IsNaN(got[3]) {
			t.Fatalf("special values were not preserved: %v", got[:4])
		}
		if !math.IsInf(got[4], 1) || !math.IsInf(got[5], -1) {
			t.Fatalf("overflow did not preserve sign: %v", got[4:])
		}
	})
}

func TestBridgeMathAndWindowScale(t *testing.T) {
	previous := WindowScale()
	t.Cleanup(func() { SetWindowScale(previous) })
	for _, scale := range []float64{0.5, 1, 2.5} {
		SetWindowScale(scale)
		if got := WindowScale(); got != scale {
			t.Fatalf("WindowScale = %v, want %v", got, scale)
		}
	}
	for _, degree := range []float64{-360, -180, -90, 0, 90, 180, 360} {
		rad := DegToRad(degree)
		if math.Abs(rad-degree*math.Pi/180) > 1e-12 || math.Abs(RadToDeg(rad)-degree) > 1e-12 {
			t.Errorf("angle conversion failed for %v degrees", degree)
		}
	}
	for _, tc := range []struct{ rad, sin, cos float64 }{
		{0, 0, 1}, {math.Pi / 2, 1, 0}, {math.Pi, 0, -1}, {-math.Pi / 2, -1, 0},
	} {
		got := Sincos(tc.rad)
		if math.Abs(got.X-tc.sin) > 1e-12 || math.Abs(got.Y-tc.cos) > 1e-12 {
			t.Errorf("Sincos(%v) = %v, want (%v, %v)", tc.rad, got, tc.sin, tc.cos)
		}
	}
	for _, value := range []float64{-2.5, 0, 3.25} {
		if got := UniformVec2(value); got.X != value || got.Y != value {
			t.Errorf("UniformVec2(%v) = %v", value, got)
		}
	}
}
