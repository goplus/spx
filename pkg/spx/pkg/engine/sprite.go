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

type Sprite struct {
	Id                       Object
	OnTriggerEnterEvent      *Event1[ISpriter]
	OnTriggerExitEvent       *Event1[ISpriter]
	OnScreenExitedEvent      *Event0
	OnScreenEnteredEvent     *Event0
	OnFramesSetChangedEvent  *Event0
	OnAnimationChangedEvent  *Event0
	OnFrameChangedEvent      *Event0
	OnAnimationLoopedEvent   *Event0
	OnAnimationFinishedEvent *Event0
	OnVfxFinishedEvent       *Event0
}

// BaseSprite identifies the shared engine object behind embedded sprite views.
func (s *Sprite) BaseSprite() *Sprite { return s }

func (s *Sprite) V_OnScreenExited() {
	s.OnScreenExitedEvent.Trigger()
}

func (s *Sprite) OnScreenExited() {
}

func (s *Sprite) V_OnScreenEntered() {
	s.OnScreenEnteredEvent.Trigger()
}

func (s *Sprite) OnScreenEntered() {
}

func (s *Sprite) V_OnFramesSetChanged() {
	s.OnFramesSetChangedEvent.Trigger()
}

func (s *Sprite) OnFramesSetChanged() {
}

func (s *Sprite) V_OnAnimationChanged() {
	s.OnAnimationChangedEvent.Trigger()
}

func (s *Sprite) OnAnimationChanged() {
}

func (s *Sprite) V_OnFrameChanged() {
	s.OnFrameChangedEvent.Trigger()
}

func (s *Sprite) OnFrameChanged() {
}

func (s *Sprite) V_OnAnimationLooped() {
	s.OnAnimationLoopedEvent.Trigger()
}

func (s *Sprite) OnAnimationLooped() {
}

func (s *Sprite) V_OnVfxFinished() {
	s.OnVfxFinishedEvent.Trigger()
}

func (s *Sprite) OnVfxFinished() {
}

func (s *Sprite) V_OnAnimationFinished() {
	s.OnAnimationFinishedEvent.Trigger()
}

func (s *Sprite) OnAnimationFinished() {
}

func (s *Sprite) V_OnTriggerEnter(other ISpriter) {
	s.OnTriggerEnterEvent.Trigger(other)
}

func (s *Sprite) V_OnTriggerExit(other ISpriter) {
	s.OnTriggerExitEvent.Trigger(other)
}

func (s *Sprite) OnTriggerEnter(ISpriter) {}

func (s *Sprite) OnTriggerExit(ISpriter) {}

func (s *Sprite) GetId() Object {
	return s.Id
}

func (s *Sprite) SetId(id Object) {
	s.Id = id
}

func (s *Sprite) Destroy() bool {
	return SpriteMgr.DestroySprite(s.Id)
}

func (s *Sprite) OnStart() {
}

func (s *Sprite) OnUpdate(delta float64) {
}

func (s *Sprite) OnFixedUpdate(delta float64) {
}

func (s *Sprite) OnDestroy() {
}

func (s *Sprite) AddPos(deltaX, deltaY float64) {
	pos := s.GetPosition()
	pos.X += deltaX
	pos.Y += deltaY
	s.SetPosition(pos)
}

func (s *Sprite) AddPosX(delta float64) {
	s.AddPos(delta, 0)
}

func (s *Sprite) AddPosY(delta float64) {
	s.AddPos(0, delta)
}

func (s *Sprite) GetPosX() float64 {
	return s.GetPosition().X
}

func (s *Sprite) GetPosY() float64 {
	return s.GetPosition().Y
}

func (s *Sprite) SetPosX(value float64) {
	pos := s.GetPosition()
	pos.X = value
	s.SetPosition(pos)
}

func (s *Sprite) SetPosY(value float64) {
	pos := s.GetPosition()
	pos.Y = value
	s.SetPosition(pos)
}

func (s *Sprite) AddVel(deltaX, deltaY float64) {
	pos := s.GetVelocity()
	pos.X += deltaX
	pos.Y += deltaY
	s.SetVelocity(pos)
}

func (s *Sprite) AddVelX(delta float64) {
	s.AddVel(delta, 0)
}

func (s *Sprite) AddVelY(delta float64) {
	s.AddVel(0, delta)
}

func (s *Sprite) GetVelX() float64 {
	return s.GetVelocity().X
}

func (s *Sprite) GetVelY() float64 {
	return s.GetVelocity().Y
}

func (s *Sprite) SetVelX(value float64) {
	pos := s.GetVelocity()
	pos.X = value
	s.SetVelocity(pos)
}

func (s *Sprite) SetVelY(value float64) {
	pos := s.GetVelocity()
	pos.Y = value
	s.SetVelocity(pos)
}

func (s *Sprite) AddScale(deltaX, deltaY float64) {
	pos := s.GetScale()
	pos.X += deltaX
	pos.Y += deltaY
	s.SetScale(pos)
}

func (s *Sprite) AddScaleX(delta float64) {
	s.AddScale(delta, 0)
}

func (s *Sprite) AddScaleY(delta float64) {
	s.AddScale(0, delta)
}

func (s *Sprite) GetScaleX() float64 {
	return s.GetScale().X
}

func (s *Sprite) GetScaleY() float64 {
	return s.GetScale().Y
}

func (s *Sprite) SetScaleX(value float64) {
	scale := s.GetScale()
	scale.X = value
	s.SetScale(scale)
}

func (s *Sprite) SetScaleY(value float64) {
	scale := s.GetScale()
	scale.Y = value
	s.SetScale(scale)
}

func (s *Sprite) PlayAnimation(name string) {
	s.PlayAnim(name, 1, false, false)
}

func (s *Sprite) DisablePhysic() {
	s.SetTriggerLayer(0)
	s.SetCollisionLayer(0)
	s.SetCollisionMask(0)
	s.SetTriggerMask(0)
}

func (s *Sprite) onCreate() {
	s.OnTriggerEnterEvent = NewEvent1[ISpriter]()
	s.OnTriggerExitEvent = NewEvent1[ISpriter]()
	s.OnScreenExitedEvent = NewEvent0()
	s.OnScreenEnteredEvent = NewEvent0()
	s.OnFramesSetChangedEvent = NewEvent0()
	s.OnAnimationChangedEvent = NewEvent0()
	s.OnFrameChangedEvent = NewEvent0()
	s.OnAnimationLoopedEvent = NewEvent0()
	s.OnAnimationFinishedEvent = NewEvent0()
	s.OnVfxFinishedEvent = NewEvent0()
}
