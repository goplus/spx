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
	"os"
	"path/filepath"
	"testing"

	coreproject "github.com/goplus/spx/v2/internal/core/project"
)

func TestApplyStoredRuntimeConfigReusesResolvedInput(t *testing.T) {
	t.Setenv("SPX_SCREENSHOT_KEY", "")

	conf := Config{
		Width:            640,
		Height:           480,
		FullScreen:       true,
		EventQueuePolicy: "block",
	}

	var game Game

	proj := coreproject.ProjectConfig{}
	game.applyRuntimeConfig(&conf, &proj)

	cwd, _ := os.Getwd()
	wantTitle := filepath.Base(cwd) + " (by XGo Builder)"
	if game.runtimeConfigInput.Title != wantTitle {
		t.Fatalf("runtimeConfigInput.Title = %q, want %q", game.runtimeConfigInput.Title, wantTitle)
	}
	if !proj.FullScreen {
		t.Fatal("applyRuntimeConfig did not propagate fullscreen override")
	}
	if got := game.gameRuntimeState.EventQueuePolicy; got != parseEventQueuePolicy("block") {
		t.Fatalf("EventQueuePolicy = %v, want block", got)
	}
	if game.displayState.WindowWidth != 640 || game.displayState.WindowHeight != 480 {
		t.Fatalf("window size = %dx%d, want 640x480", game.displayState.WindowWidth, game.displayState.WindowHeight)
	}

	proj = coreproject.ProjectConfig{}
	game.applyStoredRuntimeConfig(&proj)
	if !proj.FullScreen {
		t.Fatal("applyStoredRuntimeConfig did not reuse stored fullscreen override")
	}
	if game.displayState.WindowWidth != 640 || game.displayState.WindowHeight != 480 {
		t.Fatalf("reapplied window size = %dx%d, want 640x480", game.displayState.WindowWidth, game.displayState.WindowHeight)
	}

}
