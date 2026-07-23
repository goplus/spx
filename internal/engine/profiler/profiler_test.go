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

package profiler

import (
	"testing"

	itime "github.com/goplus/spx/v3/internal/time"
)

func TestCalcfpsRebasesAfterEngineTimeRestart(t *testing.T) {
	itime.Start(nil)
	debugLastFrame = 100
	debugLastTime = -1
	fps = 30

	itime.Start(nil)

	if got := Calcfps(); got < 0 {
		t.Fatalf("Calcfps() after engine time restart = %v, want non-negative", got)
	}
	if debugLastFrame != 0 {
		t.Fatalf("debugLastFrame after reset = %d, want 0", debugLastFrame)
	}
}
