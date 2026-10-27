//go:build !pure_engine

/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
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

package enginewrap

import (
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/goplus/spbase/mathf"
	gdx "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

type dispatchCameraMgr struct {
	gdx.ICameraMgr
	position mathf.Vec2
	reads    int
}

type dispatchInputMgr struct {
	gdx.IInputMgr
	pressed map[int64]bool
	queries []int64
}

type dispatchSpriteMgr struct {
	gdx.ISpriteMgr
	retrieve func([]int64, []float32) bool
}

func TestManagerResultWaitsForMainThreadDispatch(t *testing.T) {
	previousCallback, previousCamera := mainCallback, gdx.CameraMgr
	backend := &dispatchCameraMgr{position: mathf.NewVec2(-12, 34)}
	gdx.CameraMgr = backend
	queued := make(chan func(), 1)
	released := make(chan struct{})
	release := sync.OnceFunc(func() { close(released) })
	Init(func(call func()) { queued <- call; <-released })
	finished := make(chan mathf.Vec2, 1)
	done := make(chan struct{})
	t.Cleanup(func() {
		release()
		<-done
		mainCallback, gdx.CameraMgr = previousCallback, previousCamera
	})
	go func() {
		defer close(done)
		finished <- cameraMgr.GetPosition()
	}()

	var call func()
	select {
	case call = <-queued:
	case <-time.After(5 * time.Second):
		t.Fatal("camera request was not dispatched")
	}
	if backend.reads != 0 {
		t.Fatal("backend ran before the main-thread dispatcher executed the request")
	}
	call()
	if backend.reads != 1 {
		t.Fatalf("backend calls = %d, want one", backend.reads)
	}
	select {
	case <-finished:
		t.Fatal("manager returned before the synchronous dispatcher completed")
	default:
	}
	release()
	select {
	case got := <-finished:
		if got != backend.position {
			t.Fatalf("camera position = %v, want %v", got, backend.position)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("manager did not return after dispatch completed")
	}

	Init(func(call func()) { call() })
	cameraMgr.SetPosition(mathf.NewVec2(56, -78))
	if got := cameraMgr.GetPosition(); got != mathf.NewVec2(56, -78) {
		t.Fatalf("camera aliases lost the updated position: %v", got)
	}
}

func TestMousePressedUsesPrimaryButtons(t *testing.T) {
	previousCallback, previousInput := mainCallback, gdx.InputMgr
	Init(func(call func()) { call() })
	t.Cleanup(func() { mainCallback, gdx.InputMgr = previousCallback, previousInput })
	for _, test := range []struct {
		name    string
		buttons []int64
		want    bool
	}{
		{"none", nil, false},
		{"left", []int64{1}, true},
		{"right", []int64{2}, true},
		{"middle only", []int64{3}, false},
		{"both primary", []int64{1, 2}, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			backend := &dispatchInputMgr{pressed: make(map[int64]bool)}
			for _, button := range test.buttons {
				backend.pressed[button] = true
			}
			gdx.InputMgr = backend
			if got := inputMgr.MousePressed(); got != test.want {
				t.Fatalf("MousePressed() = %v, want %v", got, test.want)
			}
			for _, button := range backend.queries {
				if button != MOUSE_BUTTON_LEFT && button != MOUSE_BUTTON_RIGHT {
					t.Errorf("queried non-primary mouse button %d", button)
				}
			}
		})
	}
}

func TestBatchPositionBridgePreservesBuffersAndFailure(t *testing.T) {
	previousCallback, previousSprite := mainCallback, gdx.SpriteMgr
	insideDispatch := false
	Init(func(call func()) {
		insideDispatch = true
		defer func() { insideDispatch = false }()
		call()
	})
	t.Cleanup(func() { mainCallback, gdx.SpriteMgr = previousCallback, previousSprite })
	for _, success := range []bool{true, false} {
		ids := []int64{1<<54 + 3, 7}
		positions := []float32{9, 8, 7, 6}
		calls := 0
		gdx.SpriteMgr = &dispatchSpriteMgr{retrieve: func(gotIDs []int64, out []float32) bool {
			calls++
			if !insideDispatch || len(gotIDs) != len(ids) || &gotIDs[0] != &ids[0] || len(out) != len(positions) || &out[0] != &positions[0] {
				t.Fatal("batch buffers were copied or invoked outside main-thread dispatch")
			}
			if success {
				copy(out, []float32{-10, 20, 30, -40})
			}
			return success
		}}
		if got := spriteMgr.BatchRetrievePositions(ids, positions); got != success || calls != 1 {
			t.Fatalf("batch result = %v, calls = %d, want %v and one call", got, calls, success)
		}
		want := []float32{9, 8, 7, 6}
		if success {
			want = []float32{-10, 20, 30, -40}
		}
		if !slices.Equal(positions, want) || !slices.Equal(ids, []int64{1<<54 + 3, 7}) {
			t.Fatalf("batch payload changed: IDs=%v positions=%v", ids, positions)
		}
	}
	marker := new(int)
	gdx.SpriteMgr = &dispatchSpriteMgr{retrieve: func([]int64, []float32) bool { panic(marker) }}
	defer func() {
		if got := recover(); got != marker || insideDispatch {
			t.Errorf("panic = %v, dispatch active = %v; want original panic after cleanup", got, insideDispatch)
		}
	}()
	spriteMgr.BatchRetrievePositions(nil, nil)
}

func (m *dispatchCameraMgr) GetCameraPosition() mathf.Vec2 {
	m.reads++
	return m.position
}

func (m *dispatchCameraMgr) SetCameraPosition(position mathf.Vec2) { m.position = position }

func (m *dispatchInputMgr) GetMouseState(button int64) bool {
	m.queries = append(m.queries, button)
	return m.pressed[button]
}

func (m *dispatchSpriteMgr) BatchRetrievePositions(ids []int64, out []float32) bool {
	return m.retrieve(ids, out)
}
