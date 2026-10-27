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
func (p *Sprite) BaseSprite() *Sprite { return p }

func (p *Sprite) V_OnScreenExited() {
	p.OnScreenExitedEvent.Trigger()
}

func (p *Sprite) OnScreenExited() {
}

func (p *Sprite) V_OnScreenEntered() {
	p.OnScreenEnteredEvent.Trigger()
}

func (p *Sprite) OnScreenEntered() {
}

func (p *Sprite) V_OnFramesSetChanged() {
	p.OnFramesSetChangedEvent.Trigger()
}

func (p *Sprite) OnFramesSetChanged() {
}

func (p *Sprite) V_OnAnimationChanged() {
	p.OnAnimationChangedEvent.Trigger()
}

func (p *Sprite) OnAnimationChanged() {
}

func (p *Sprite) V_OnFrameChanged() {
	p.OnFrameChangedEvent.Trigger()
}

func (p *Sprite) OnFrameChanged() {
}

func (p *Sprite) V_OnAnimationLooped() {
	p.OnAnimationLoopedEvent.Trigger()
}

func (p *Sprite) OnAnimationLooped() {
}

func (p *Sprite) V_OnVfxFinished() {
	p.OnVfxFinishedEvent.Trigger()
}

func (p *Sprite) OnVfxFinished() {
}

func (p *Sprite) V_OnAnimationFinished() {
	p.OnAnimationFinishedEvent.Trigger()
}

func (p *Sprite) OnAnimationFinished() {
}

func (p *Sprite) V_OnTriggerEnter(other ISpriter) {
	p.OnTriggerEnterEvent.Trigger(other)
}

func (p *Sprite) V_OnTriggerExit(other ISpriter) {
	p.OnTriggerExitEvent.Trigger(other)
}

func (p *Sprite) OnTriggerEnter(ISpriter) {}

func (p *Sprite) OnTriggerExit(ISpriter) {}

func (p *Sprite) GetId() Object {
	return p.Id
}

func (p *Sprite) SetId(id Object) {
	p.Id = id
}

func (p *Sprite) Destroy() bool {
	return SpriteMgr.DestroySprite(p.Id)
}

func (p *Sprite) OnStart() {
}

func (p *Sprite) OnUpdate(delta float64) {
}

func (p *Sprite) OnFixedUpdate(delta float64) {
}

func (p *Sprite) OnDestroy() {
}

func (p *Sprite) AddPos(deltaX, deltaY float64) {
	pos := p.GetPosition()
	pos.X += deltaX
	pos.Y += deltaY
	p.SetPosition(pos)
}

func (p *Sprite) AddPosX(delta float64) {
	p.AddPos(delta, 0)
}

func (p *Sprite) AddPosY(delta float64) {
	p.AddPos(0, delta)
}

func (p *Sprite) GetPosX() float64 {
	return p.GetPosition().X
}

func (p *Sprite) GetPosY() float64 {
	return p.GetPosition().Y
}

func (p *Sprite) SetPosX(value float64) {
	pos := p.GetPosition()
	pos.X = value
	p.SetPosition(pos)
}

func (p *Sprite) SetPosY(value float64) {
	pos := p.GetPosition()
	pos.Y = value
	p.SetPosition(pos)
}

func (p *Sprite) AddVel(deltaX, deltaY float64) {
	pos := p.GetVelocity()
	pos.X += deltaX
	pos.Y += deltaY
	p.SetVelocity(pos)
}

func (p *Sprite) AddVelX(delta float64) {
	p.AddVel(delta, 0)
}

func (p *Sprite) AddVelY(delta float64) {
	p.AddVel(0, delta)
}

func (p *Sprite) GetVelX() float64 {
	return p.GetVelocity().X
}

func (p *Sprite) GetVelY() float64 {
	return p.GetVelocity().Y
}

func (p *Sprite) SetVelX(value float64) {
	pos := p.GetVelocity()
	pos.X = value
	p.SetVelocity(pos)
}

func (p *Sprite) SetVelY(value float64) {
	pos := p.GetVelocity()
	pos.Y = value
	p.SetVelocity(pos)
}

func (p *Sprite) AddScale(deltaX, deltaY float64) {
	pos := p.GetScale()
	pos.X += deltaX
	pos.Y += deltaY
	p.SetScale(pos)
}

func (p *Sprite) AddScaleX(delta float64) {
	p.AddScale(delta, 0)
}

func (p *Sprite) AddScaleY(delta float64) {
	p.AddScale(0, delta)
}

func (p *Sprite) GetScaleX() float64 {
	return p.GetScale().X
}

func (p *Sprite) GetScaleY() float64 {
	return p.GetScale().Y
}

func (p *Sprite) SetScaleX(value float64) {
	scale := p.GetScale()
	scale.X = value
	p.SetScale(scale)
}

func (p *Sprite) SetScaleY(value float64) {
	scale := p.GetScale()
	scale.Y = value
	p.SetScale(scale)
}

func (p *Sprite) PlayAnimation(name string) {
	p.PlayAnim(name, 1, false, false)
}

func (p *Sprite) DisablePhysic() {
	p.SetTriggerLayer(0)
	p.SetCollisionLayer(0)
	p.SetCollisionMask(0)
	p.SetTriggerMask(0)
}

func (p *Sprite) onCreate() {
	p.OnTriggerEnterEvent = NewEvent1[ISpriter]()
	p.OnTriggerExitEvent = NewEvent1[ISpriter]()
	p.OnScreenExitedEvent = NewEvent0()
	p.OnScreenEnteredEvent = NewEvent0()
	p.OnFramesSetChangedEvent = NewEvent0()
	p.OnAnimationChangedEvent = NewEvent0()
	p.OnFrameChangedEvent = NewEvent0()
	p.OnAnimationLoopedEvent = NewEvent0()
	p.OnAnimationFinishedEvent = NewEvent0()
	p.OnVfxFinishedEvent = NewEvent0()
}
