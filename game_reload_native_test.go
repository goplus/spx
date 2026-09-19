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

package spx

import (
	"errors"
	"strings"
	"testing"

	pkgengine "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

type reloadWorkerPlatformMgr struct {
	pkgengine.IPlatformMgr
}

func (*reloadWorkerPlatformMgr) IsMainThread() bool { return false }

func TestReloadRejectsNonEngineThreadBeforePreflight(t *testing.T) {
	game, sprite, _ := setupReloadPreflightGame(t, nil)
	pkgengine.PlatformMgr = &reloadWorkerPlatformMgr{}

	err := XGot_Game_Reload(game, strings.NewReader("{"))
	if !errors.Is(err, errReloadWrongThread) {
		t.Fatalf("XGot_Game_Reload error = %v, want %v", err, errReloadWrongThread)
	}
	if game.Game.sprs["Sprite"] != sprite || game.Sprite != sprite {
		t.Fatal("off-thread reload mutated the live game")
	}
	assertReloadAvailable(t, &game.Game)
}
