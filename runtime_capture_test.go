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
	"testing"

	"github.com/goplus/spx/v2/internal/engine"
	itime "github.com/goplus/spx/v2/internal/time"
)

func TestFrameDecoratorTurnsOnKeyIntoFrameTask(t *testing.T) {
	g := &Game{}
	engine.SetGame(g)
	defer engine.SetGame(nil)
	itime.Start(nil)

	g.scriptEventBindings.init(&g.scriptEvents, g)
	ran := 0
	Frame(1, func() {
		g.OnKey__0(KeyUp, func() {
			ran++
		})
	})

	if got := len(g.scriptEvents.manager.SnapshotKeyPressed()); got != 0 {
		t.Fatalf("OnKey registered %d key sinks, want scheduled frame task", got)
	}
	itime.Update(1.0/30, 30)
	g.runFrameTasks()
	if ran != 1 {
		t.Fatalf("scheduled OnKey body ran %d times, want 1", ran)
	}
}

func TestCaptureDecoratorAddsPendingFrameCapture(t *testing.T) {
	g := &Game{}
	engine.SetGame(g)
	defer engine.SetGame(nil)
	itime.Start(nil)

	g.scriptEventBindings.init(&g.scriptEvents, g)
	Frame(1, func() {
		CaptureAndCheck("step_001.png", func() {
			g.OnKey__0(KeyUp, func() {})
		})
	})

	itime.Update(1.0/30, 30)
	g.runFrameTasks()
	if len(g.pendingFrameCapture) != 1 {
		t.Fatalf("pendingFrameCapture len = %d, want 1", len(g.pendingFrameCapture))
	}
	if got := g.pendingFrameCapture[0]; got.name != "step_001.png" || !got.check {
		t.Fatalf("pendingFrameCapture[0] = %+v, want capture-and-check step_001.png", got)
	}
}
