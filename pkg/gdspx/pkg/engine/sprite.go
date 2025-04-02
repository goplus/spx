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

func (pself *Sprite) onCreate() {
	pself.OnTriggerEnterEvent = NewEvent1[ISpriter]()
	pself.OnTriggerExitEvent = NewEvent1[ISpriter]()
	pself.OnScreenExitedEvent = NewEvent0()
	pself.OnScreenEnteredEvent = NewEvent0()
	pself.OnFramesSetChangedEvent = NewEvent0()
	pself.OnAnimationChangedEvent = NewEvent0()
	pself.OnFrameChangedEvent = NewEvent0()
	pself.OnAnimationLoopedEvent = NewEvent0()
	pself.OnAnimationFinishedEvent = NewEvent0()
}

func (pself *Sprite) V_OnScreenExited() {
	pself.OnScreenExitedEvent.Trigger()
}

func (pself *Sprite) OnScreenExited() {
}

func (pself *Sprite) V_OnScreenEntered() {
	pself.OnScreenEnteredEvent.Trigger()
}

func (pself *Sprite) OnScreenEntered() {
}

func (pself *Sprite) V_OnFramesSetChanged() {
	pself.OnFramesSetChangedEvent.Trigger()
}

func (pself *Sprite) OnFramesSetChanged() {
}

func (pself *Sprite) V_OnAnimationChanged() {
	pself.OnAnimationChangedEvent.Trigger()
}

func (pself *Sprite) OnAnimationChanged() {
}

func (pself *Sprite) V_OnFrameChanged() {
	pself.OnFrameChangedEvent.Trigger()
}

func (pself *Sprite) OnFrameChanged() {
}

func (pself *Sprite) V_OnAnimationLooped() {
	pself.OnAnimationLoopedEvent.Trigger()
}

func (pself *Sprite) OnAnimationLooped() {
}

func (pself *Sprite) V_OnVfxFinished() {
	pself.OnVfxFinishedEvent.Trigger()
}

func (pself *Sprite) OnVfxFinished() {
}

func (pself *Sprite) V_OnAnimationFinished() {
	pself.OnAnimationFinishedEvent.Trigger()
}

func (pself *Sprite) OnAnimationFinished() {
}

func (pself *Sprite) V_OnTriggerEnter(other ISpriter) {
	pself.OnTriggerEnterEvent.Trigger(other)
}

func (pself *Sprite) V_OnTriggerExit(other ISpriter) {
	pself.OnTriggerExitEvent.Trigger(other)
}

func (pself *Sprite) OnTriggerEnter(ISpriter) {}

func (pself *Sprite) OnTriggerExit(ISpriter) {}

func (pself *Sprite) GetId() Object {
	return pself.Id
}
func (pself *Sprite) SetId(id Object) {
	pself.Id = id
}
func (pself *Sprite) Destroy() bool {
	return SpriteMgr.DestroySprite(pself.Id)
}
func (pself *Sprite) OnStart() {
}
func (pself *Sprite) OnUpdate(delta float64) {
}
func (pself *Sprite) OnFixedUpdate(delta float64) {
}

func (pself *Sprite) OnDestroy() {
}

func (pself *Sprite) AddPos(deltaX, deltaY float64) {
	pos := pself.GetPosition()
	pos.X += deltaX
	pos.Y += deltaY
	pself.SetPosition(pos)
}
func (pself *Sprite) AddPosX(delta float64) {
	pself.AddPos(delta, 0)
}

func (pself *Sprite) AddPosY(delta float64) {
	pself.AddPos(0, delta)
}

func (pself *Sprite) GetPosX() float64 {
	return pself.GetPosition().X
}

func (pself *Sprite) GetPosY() float64 {
	return pself.GetPosition().Y
}

func (pself *Sprite) SetPosX(value float64) {
	pos := pself.GetPosition()
	pos.X = value
	pself.SetPosition(pos)
}

func (pself *Sprite) SetPosY(value float64) {
	pos := pself.GetPosition()
	pos.Y = value
	pself.SetPosition(pos)
}

func (pself *Sprite) AddVel(deltaX, deltaY float64) {
	pos := pself.GetVelocity()
	pos.X += deltaX
	pos.Y += deltaY
	pself.SetVelocity(pos)
}
func (pself *Sprite) AddVelX(delta float64) {
	pself.AddVel(delta, 0)
}

func (pself *Sprite) AddVelY(delta float64) {
	pself.AddVel(0, delta)
}

func (pself *Sprite) GetVelX() float64 {
	return pself.GetVelocity().X
}

func (pself *Sprite) GetVelY() float64 {
	return pself.GetVelocity().Y
}

func (pself *Sprite) SetVelX(value float64) {
	pos := pself.GetVelocity()
	pos.X = value
	pself.SetVelocity(pos)
}

func (pself *Sprite) SetVelY(value float64) {
	pos := pself.GetVelocity()
	pos.Y = value
	pself.SetVelocity(pos)
}

func (pself *Sprite) AddScale(deltaX, deltaY float64) {
	pos := pself.GetScale()
	pos.X += deltaX
	pos.Y += deltaY
	pself.SetScale(pos)
}
func (pself *Sprite) AddScaleX(delta float64) {
	pself.AddScale(delta, 0)
}

func (pself *Sprite) AddScaleY(delta float64) {
	pself.AddScale(0, delta)
}

func (pself *Sprite) GetScaleX() float64 {
	return pself.GetScale().X
}

func (pself *Sprite) GetScaleY() float64 {
	return pself.GetScale().Y
}

func (pself *Sprite) SetScaleX(value float64) {
	pos := pself.GetScale()
	pos.X = value
	pself.SetScale(pos)
}

func (pself *Sprite) SetScaleY(value float64) {
	pos := pself.GetScale()
	pos.Y = value
	pself.SetScale(pos)
}

func (pself *Sprite) PlayAnimation(name string) {
	pself.PlayAnim(name, 1, false, false)
}

func (pself *Sprite) DisablePhysic() {
	pself.SetTriggerLayer(0)
	pself.SetCollisionLayer(0)
	pself.SetCollisionMask(0)
	pself.SetTriggerMask(0)
}
