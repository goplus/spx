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
	"github.com/goplus/spbase/mathf"
	"github.com/goplus/spx/v4/internal/engine"
)

func (p *inputManager) resolveInputSessionState(session *inputSession, delta float64) (InputReplayState, error) {
	session.operationMu.Lock()
	defer session.operationMu.Unlock()
	c := &p.frame
	resolved, err := session.consumeSampledInputTickLocked(delta, func() (InputReplayState, []InputReplayMouseEvent, []InputReplayKeyEvent) {
		state := p.sampleInputState()
		return state, replayMouseEventsFromEngine(c.mouseEvents), replayKeyEventsFromEngine(c.keyEvents)
	})
	if err != nil {
		return InputReplayState{}, err
	}
	if resolved.firstTick {
		point := mathf.Vec2{X: resolved.initial.position.X, Y: resolved.initial.position.Y}
		c.history.Reset(point, resolved.initial.buttons&1 != 0)
		p.clickGate.InitWithClock(mouseClickInterval, p.g.inputClock)
		p.swipe.InitWithClock(p.g.inputClock)
	}
	c.mouseEvents = engineMouseEventsFromReplay(resolved.frame.MouseEvents, c.mouseEvents)
	c.keyEvents = engineKeyEventsFromReplay(resolved.frame.KeyEvents, c.keyEvents)
	return resolved.frame.State, nil
}

func (p *inputManager) effectiveKeyPressed(key Key) bool {
	if state, replaying := p.g.currentInputSession().playbackInput(key); replaying {
		return state.keyPressed
	}
	return engine.Managers().InputMgr.GetKey(int64(key))
}

func (p *inputManager) effectiveMousePos() mathf.Vec2 {
	if mouse, replaying := p.g.currentInputSession().playbackInput(KeyAny); replaying {
		return mathf.Vec2{X: mouse.position.X, Y: mouse.position.Y}
	}
	return p.mousePos
}

func (p *inputManager) effectiveMousePressed() bool {
	if mouse, replaying := p.g.currentInputSession().playbackInput(KeyAny); replaying {
		return mouse.buttons&0b011 != 0
	}
	return engine.AnyMouseButtonPressed()
}
