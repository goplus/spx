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

	coreproject "github.com/goplus/spx/v2/internal/core/project"
	itime "github.com/goplus/spx/v2/internal/time"
)

func TestApplyDeterministicConfigDefaults(t *testing.T) {
	var game Game
	defer itime.SetFixedDeltaTime(0)
	defer ResetRandomSeed()

	game.applyDeterministicConfig(coreproject.RuntimeConfig{Deterministic: true})
	if got, ok := itime.FixedDeltaTime(); ok || got != 0 {
		t.Fatalf("FixedDeltaTime() = (%v, %v), want (0, false) off web", got, ok)
	}
	if game.configuredFixedTimestep != 0 {
		t.Fatalf("configuredFixedTimestep = %v, want 0 off web", game.configuredFixedTimestep)
	}

	gotA := []float64{Rand__0(1, 10), Rand__1(0, 1)}
	game.applyDeterministicConfig(coreproject.RuntimeConfig{Deterministic: true})
	gotB := []float64{Rand__0(1, 10), Rand__1(0, 1)}
	for i := range gotA {
		if gotA[i] != gotB[i] {
			t.Fatalf("deterministic random mismatch: %v vs %v", gotA, gotB)
		}
	}
}

func TestResolveFixedTimestepUsesConservativeDefaultOnWeb(t *testing.T) {
	if got := resolveFixedTimestep(0, true, true); got != defaultDeterministicWebTimestep {
		t.Fatalf("resolveFixedTimestep(web) = %v, want %v", got, defaultDeterministicWebTimestep)
	}
}

func TestResolveFixedTimestepKeepsExplicitValueOnWeb(t *testing.T) {
	if got := resolveFixedTimestep(1.0/15, true, true); got != 1.0/15 {
		t.Fatalf("resolveFixedTimestep(explicit web) = %v, want %v", got, 1.0/15)
	}
}

func TestResolveFixedTimestepDisablesValueOffWeb(t *testing.T) {
	if got := resolveFixedTimestep(1.0/30, true, false); got != 0 {
		t.Fatalf("resolveFixedTimestep(non-web) = %v, want 0", got)
	}
}
