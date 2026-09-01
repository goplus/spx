//go:build !js && !pure_engine

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

package platform

import (
	"testing"

	gdx "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

type testPlatformMgr struct {
	gdx.IPlatformMgr
	main bool
}

func (p testPlatformMgr) IsMainThread() bool {
	return p.main
}

func TestTryCallEngineDirectlyUsesGodot(t *testing.T) {
	previous := gdx.PlatformMgr
	t.Cleanup(func() { gdx.PlatformMgr = previous })

	gdx.PlatformMgr = testPlatformMgr{main: true}
	called := false
	if !TryCallEngineDirectly(func() { called = true }) {
		t.Fatal("Godot main thread was not reported")
	}
	if !called {
		t.Fatal("engine call did not run on the Godot main thread")
	}

	gdx.PlatformMgr = testPlatformMgr{main: false}
	called = false
	if TryCallEngineDirectly(func() { called = true }) {
		t.Fatal("Godot worker thread was reported as the main thread")
	}
	if called {
		t.Fatal("engine call ran on a Godot worker thread")
	}
}

func TestTryCallEngineDirectlyWithoutPlatformManager(t *testing.T) {
	previous := gdx.PlatformMgr
	t.Cleanup(func() { gdx.PlatformMgr = previous })
	gdx.PlatformMgr = nil
	called := false
	if TryCallEngineDirectly(func() { called = true }) {
		t.Fatal("missing platform manager was reported as the main thread")
	}
	if called {
		t.Fatal("engine call ran without a platform manager")
	}
}
