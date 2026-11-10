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
	"github.com/goplus/spbase/mathf"
	gdx "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

type positionTween struct {
	sprite     gdx.ISpriter
	startValue mathf.Vec2
	callback   func()
	step       int
	elapsed    float64
	steps      []tweenStep
}

type tweenStep struct {
	value     mathf.Vec2
	duration  float64
	startTime float64
}

func tweenPos(node gdx.ISpriter, pos mathf.Vec2, duration float64, callback func()) {
	startPositionTween(node, callback, tweenStep{value: pos, duration: duration})
}

func tweenPos2(node gdx.ISpriter, pos mathf.Vec2, duration float64, pos2 mathf.Vec2, duration2 float64, callback func()) {
	startPositionTween(node, callback,
		tweenStep{value: pos, duration: duration},
		tweenStep{value: pos2, duration: duration2},
	)
}

// Registration runs in the caller's serialized runtime context; advancing and
// dispatching completion callbacks belongs to the frame dispatcher.
func startPositionTween(sprite gdx.ISpriter, callback func(), steps ...tweenStep) {
	sprite = resolveRuntimeSprite(sprite)
	if sprite == nil {
		return
	}

	tween := &positionTween{
		sprite:   sprite,
		callback: callback,
		steps:    steps,
	}
	if runtimeSpriteAlive(sprite) {
		tween.startValue = sprite.GetPosition()
	}
	for i := 1; i < len(tween.steps); i++ {
		tween.steps[i].startTime = tween.steps[i-1].endTime()
	}
	state.jobs.tweens = append(state.jobs.tweens, tween)
}

func (p *frameJobs) updateTweens(delta float64, emit func(gdx.ISpriter, func())) {
	current := p.tweens
	p.tweens, p.tweenScratch = p.tweenScratch[:0], nil
	defer func() {
		clear(current)
		if cap(current) > cap(p.tweenScratch) {
			p.tweenScratch = current[:0]
		}
	}()
	ready := 0
	for _, tween := range current {
		if !runtimeSpriteAlive(tween.sprite) {
			continue
		}
		tween.advance(delta)
		if !tween.done() {
			p.tweens = append(p.tweens, tween)
		} else {
			current[ready] = tween
			ready++
		}
	}
	for _, tween := range current[:ready] {
		emit(tween.sprite, tween.callback)
	}
}

func (t *positionTween) advance(delta float64) {
	t.elapsed += delta
	for !t.done() && t.elapsed >= t.steps[t.step].endTime() {
		if runtimeSpriteAlive(t.sprite) {
			t.sprite.SetPosition(t.steps[t.step].value)
		}
		t.step++
		if !t.done() && runtimeSpriteAlive(t.sprite) {
			t.startValue = t.sprite.GetPosition()
		}
	}
	if !t.done() && runtimeSpriteAlive(t.sprite) {
		step := t.steps[t.step]
		percent := mathf.Clamp01f((t.elapsed - step.startTime) / step.duration)
		t.sprite.SetPosition(t.startValue.Lerpf(step.value, percent))
	}
}

func (t *positionTween) done() bool {
	return t.step >= len(t.steps)
}

func (s *tweenStep) endTime() float64 {
	return s.startTime + s.duration
}
