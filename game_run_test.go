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
	"time"

	"github.com/goplus/spx/v3/internal/engine"
)

func TestSchedNowWarnsInsteadOfPanickingOnMainExecutionTimeout(t *testing.T) {
	originalGame := engine.GetGame()
	t.Cleanup(func() {
		engine.SetGame(originalGame)
	})

	var g Game
	g.initGame(nil)

	setSchedInMain(true)
	setMainSchedTime(time.Now().Add(-2 * time.Duration(mainExecTimeoutSec) * time.Second))

	SchedNow()

	if isSchedInMainState() {
		t.Fatal("SchedNow should demote timed-out Main execution instead of keeping the main timeout state")
	}
	if !mainSchedTime().IsZero() {
		t.Fatalf("mainSchedTime = %v, want zero after timed-out Main demotion", mainSchedTime())
	}
}

func TestSchedWarnsInsteadOfPanickingOnMainExecutionTimeout(t *testing.T) {
	originalGame := engine.GetGame()
	t.Cleanup(func() {
		engine.SetGame(originalGame)
	})

	var g Game
	g.initGame(nil)

	setSchedInMain(true)
	setMainSchedTime(time.Now().Add(-2 * time.Duration(mainExecTimeoutSec) * time.Second))

	Sched()

	if isSchedInMainState() {
		t.Fatal("Sched should demote timed-out Main execution instead of keeping the main timeout state")
	}
	if !mainSchedTime().IsZero() {
		t.Fatalf("mainSchedTime = %v, want zero after timed-out Main demotion", mainSchedTime())
	}
}
