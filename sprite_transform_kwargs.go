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

// Kwargs-style transform methods expose kwargs-friendly movement and turning APIs.
func (p *SpriteImpl) StepWith__0(step float64) {
	speed, animation := motionOptions(nil)
	p.transform().Step(step, speed, animation)
}

func (p *SpriteImpl) StepWith__1(step float64, opts *MotionOptions) {
	speed, animation := motionOptions(opts)
	p.transform().Step(step, speed, animation)
}

func (p *SpriteImpl) StepToWith__0(target any) {
	speed, animation := motionOptions(nil)
	p.doStepTo(target, speed, animation)
}

func (p *SpriteImpl) StepToWith__1(target any, opts *MotionOptions) {
	speed, animation := motionOptions(opts)
	p.doStepTo(target, speed, animation)
}

func (p *SpriteImpl) StepToWith__2(x, y float64, opts *MotionOptions) {
	speed, animation := motionOptions(opts)
	// Coordinate overloads already have their target position, so they mirror
	// the positional StepTo variants and intentionally skip doStepTo's logging.
	p.transform().StepToPos(x, y, speed, animation)
}

func (p *SpriteImpl) TurnWith__0(dir Direction) {
	speed, animation := motionOptions(nil)
	p.transform().Turn(dir, speed, animation)
}

func (p *SpriteImpl) TurnWith__1(dir Direction, opts *MotionOptions) {
	speed, animation := motionOptions(opts)
	p.transform().Turn(dir, speed, animation)
}

func (p *SpriteImpl) TurnToWith__0(target any) {
	speed, animation := motionOptions(nil)
	p.transform().TurnTo(target, speed, animation)
}

func (p *SpriteImpl) TurnToWith__1(target any, opts *MotionOptions) {
	speed, animation := motionOptions(opts)
	p.transform().TurnTo(target, speed, animation)
}
