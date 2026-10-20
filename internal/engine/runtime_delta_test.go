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
	"slices"
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

func TestRuntimeBridgeTimersRespectObjectLifetime(t *testing.T) {
	resetRuntimeDeltaTestState()
	t.Cleanup(resetRuntimeDeltaTestState)
	bridge := runtimeBridge{}
	alive, removed := &gdx.Sprite{Id: 1}, &gdx.Sprite{Id: 2}
	node := &gdx.UiNode{Id: 3}
	state.sprites[1], state.sprites[2], state.uiNodes[3] = alive, removed, node
	if bridge.Sprites()[1] != alive || bridge.UiNodes()[3] != node || bridge.GetUINode(3) != node || !IsNodeExist(3) {
		t.Fatal("runtime registry adapters returned wrong nodes")
	}
	var calls []string
	bridge.DelayCall(.25, func() { calls = append(calls, "global") })
	bridge.DelaySpriteCall(.25, alive, func() { calls = append(calls, "alive") })
	bridge.DelaySpriteCall(.25, removed, func() { calls = append(calls, "removed") })
	bridge.DeleteSprite(2)
	bridge.DeleteUINode(3)
	bridge.InternalUpdateEngine(.125)
	if len(calls) != 0 {
		t.Fatal("timers fired before their deadline")
	}
	bridge.InternalUpdateEngine(.125)
	bridge.InternalUpdateEngine(1)
	if !slices.Equal(calls, []string{"global", "alive"}) || bridge.GetUINode(3) != nil {
		t.Fatalf("timer calls=%v, deleted node=%v", calls, bridge.GetUINode(3))
	}
	if bridge.AdvanceTimeSinceGameStart(.5) != .5 || bridge.AdvanceTimeSinceGameStart(.25) != .75 || bridge.TimeSinceGameStarted() != .75 {
		t.Fatal("runtime elapsed time did not accumulate")
	}
}
