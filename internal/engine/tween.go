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

//lint:file-ignore ST1001 Bridge code intentionally dot-imports mathf to mirror engine type names.

import (
	. "github.com/goplus/spbase/mathf"
	gdx "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

type posTweenInfo struct {
	value     Vec2
	duration  float64
	startTime float64
}

type tweenCallInfo struct {
	sprite     gdx.ISpriter
	startValue Vec2
	callback   func()
	curIndex   int
	timer      float64
	infos      []posTweenInfo
}

var (
	tweenInfos     = make([]*tweenCallInfo, 0)
	tempTweenInfos = make([]*tweenCallInfo, 0)
)

func (i *posTweenInfo) getEndTime() float64 {
	return i.startTime + i.duration
}

func (i *tweenCallInfo) isDone() bool {
	return i.curIndex >= len(i.infos)
}

func (i *tweenCallInfo) advance(delta float64) {
	i.timer += delta
	for !i.isDone() && i.timer >= i.infos[i.curIndex].getEndTime() {
		if runtimeSpriteAlive(i.sprite) {
			i.sprite.SetPosition(i.infos[i.curIndex].value)
		}
		i.curIndex++
		if !i.isDone() && runtimeSpriteAlive(i.sprite) {
			i.startValue = i.sprite.GetPosition()
		}
	}
	if !i.isDone() && runtimeSpriteAlive(i.sprite) {
		segment := i.infos[i.curIndex]
		percent := Clamp01f((i.timer - segment.startTime) / segment.duration)
		i.sprite.SetPosition(i.startValue.Lerpf(segment.value, percent))
	}
}

func updateTweens(delta float64, emit func(gdx.ISpriter, func())) {
	tweenInfos, tempTweenInfos = tempTweenInfos[:0], tweenInfos
	for _, curTween := range tempTweenInfos {
		if !runtimeSpriteAlive(curTween.sprite) {
			continue
		}
		curTween.advance(delta)
		if !curTween.isDone() {
			tweenInfos = append(tweenInfos, curTween)
		}
	}
	for _, curTween := range tempTweenInfos {
		if curTween.isDone() {
			emit(curTween.sprite, curTween.callback)
		}
	}
	clear(tempTweenInfos)
	tempTweenInfos = tempTweenInfos[:0]
}

func tweenPos(node gdx.ISpriter, pos Vec2, duration float64, callback func()) {
	startPositionTween(node, callback, posTweenInfo{value: pos, duration: duration})
}

func tweenPos2(node gdx.ISpriter, pos Vec2, duration float64, pos2 Vec2, duration2 float64, callback func()) {
	startPositionTween(node, callback,
		posTweenInfo{value: pos, duration: duration},
		posTweenInfo{value: pos2, duration: duration2},
	)
}

func startPositionTween(sprite gdx.ISpriter, callback func(), segments ...posTweenInfo) {
	sprite = resolveRuntimeSprite(sprite)
	if sprite == nil {
		return
	}

	info := &tweenCallInfo{
		sprite:   sprite,
		callback: callback,
		infos:    segments,
	}
	if runtimeSpriteAlive(sprite) {
		info.startValue = sprite.GetPosition()
	}
	for i := 1; i < len(info.infos); i++ {
		info.infos[i].startTime = info.infos[i-1].getEndTime()
	}
	tweenInfos = append(tweenInfos, info)
}
