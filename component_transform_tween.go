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

import "math"

// doTurnAnimation handles turn animation with fallback to direct heading change.
// This helper eliminates code duplication between Turn and TurnTo methods.
func (t *transformComponent) doTurnAnimation(
	from, to float64,
	speed float64,
	animation SpriteAnimationName,
	fallback func(),
) {
	if animation == "" {
		animation = t.sprite.getStateAnimName(StateTurn)
	}

	ani, ok := t.sprite.getAnimation(animation)
	if !ok {
		fallback()
		return
	}

	absDelta := math.Abs(from - to)
	duration := ani.TurnToDuration / fullCircleDegrees * absDelta / math.Max(speed, minSpeed)
	t.doAnimatedTween(animation, ani, from, to, aniTypeTurn, duration, speed)
}

// doAnimatedTween creates and executes a tween animation with the specified parameters.
// This helper eliminates code duplication in animation methods.
func (t *transformComponent) doAnimatedTween(
	name SpriteAnimationName,
	base *aniConfig,
	from, to any,
	aniType aniTypeEnum,
	duration float64,
	speed float64,
) {
	speed = math.Max(speed, minSpeed)

	aniCopy := *base
	aniCopy.From = from
	aniCopy.To = to
	aniCopy.AniType = aniType
	aniCopy.Duration = duration
	aniCopy.IsLoop = true
	aniCopy.Speed = speed

	t.sprite.doTween(name, &aniCopy)
}
