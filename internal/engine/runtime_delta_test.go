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
	"testing"

	itime "github.com/goplus/spx/v3/internal/time"
	gdx "github.com/goplus/spx/v3/pkg/spx/pkg/engine"
)

func resetRuntimeDeltaTestState() {
	delaySpriteCalls = delaySpriteCalls[:0]
	tempDelaySpriteCalls = tempDelaySpriteCalls[:0]
	tweenInfos = tweenInfos[:0]
	tempTweenInfos = tempTweenInfos[:0]
	state.sprites = make(map[Object]gdx.ISpriter)
	state.uiNodes = make(map[Object]gdx.IUiNode)
	state.timeSinceStart = 0
	itime.SetFixedDeltaTime(0)
	itime.Start(nil)
}

func advanceRuntimeDeltaFrame(realDelta float64) {
	logicalDelta := itime.EffectiveLogicalDeltaTime(realDelta)
	itime.Update(logicalDelta, 60)
	runtimeBridge{}.InternalUpdateEngine(logicalDelta)
}

func TestInternalUpdateEngineUsesScriptDeltaForDelayCall(t *testing.T) {
	resetRuntimeDeltaTestState()
	t.Cleanup(resetRuntimeDeltaTestState)

	itime.SetFixedDeltaTime(0.1)

	firedAt := 0
	delayCall(0.25, func() {
		if firedAt == 0 {
			firedAt = int(itime.Frame())
		}
	})

	for _, realDelta := range []float64{0.05, 0.20, 0.01} {
		advanceRuntimeDeltaFrame(realDelta)
	}

	if firedAt != 3 {
		t.Fatalf("delayCall fired at frame %d, want 3 with fixed delta", firedAt)
	}
}

func TestInternalUpdateEngineUsesScriptDeltaForTweens(t *testing.T) {
	resetRuntimeDeltaTestState()
	t.Cleanup(resetRuntimeDeltaTestState)

	itime.SetFixedDeltaTime(0.1)

	tweenInfos = append(tweenInfos, &tweenCallInfo{
		infos: []posTweenInfo{{duration: 1}},
	})

	advanceRuntimeDeltaFrame(0.05)
	advanceRuntimeDeltaFrame(0.20)

	if got := tweenInfos[0].timer; got != 0.2 {
		t.Fatalf("tween timer = %v, want 0.2 after two fixed-delta updates", got)
	}
}

func TestInternalUpdateEngineKeepsRealDeltaWithoutFixedTimestep(t *testing.T) {
	resetRuntimeDeltaTestState()
	t.Cleanup(resetRuntimeDeltaTestState)

	tweenInfos = append(tweenInfos, &tweenCallInfo{
		infos: []posTweenInfo{{duration: 1}},
	})

	advanceRuntimeDeltaFrame(0.05)
	advanceRuntimeDeltaFrame(0.20)

	if got := tweenInfos[0].timer; got != 0.25 {
		t.Fatalf("tween timer = %v, want 0.25 with real delta", got)
	}
}
