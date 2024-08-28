package anim

import (
	"errors"
	"log"
	"math"

	spxfs "github.com/goplus/spx/fs"
	"github.com/goplus/spx/internal/anim/common"
	"github.com/goplus/spx/internal/anim/skeleton"
	"github.com/goplus/spx/internal/anim/vertex"
	"github.com/goplus/spx/internal/math32"
	"github.com/goplus/spx/internal/tools"
	"github.com/hajimehoshi/ebiten/v2"
)

type ANIMSTATUS uint8

const (
	AnimstatusPlaying ANIMSTATUS = iota
	AnimstatusStop
)

type ANIMVALTYPE uint8

const (
	ANIMATIONTYPE_INT     = 0
	ANIMATIONTYPE_FLOAT   = 1
	ANIMATIONTYPE_VECTOR2 = 2
)

const (
	ANIMATOR_TYPE_VERTEX   = "vertex"
	ANIMATOR_TYPE_SKELETON = "skeleton"
	ANIMATOR_TYPE_FRAME    = "frame"
)

type IAnimatable interface {
	GetTarget() IAnimationTarget
	Animate() bool
}

type IAnimation interface {
	GetAnimId() int64
	Animate(delay float64, from int, to int, loop bool, speedRatio float64) bool
}

type IAnimationTarget interface {
	GetAnimations() []IAnimation
	GetAnimatables() []IAnimatable
}

type IAnimator interface {
	SetPosition(pos math32.Vector2)
	Update()
	Draw(screen *ebiten.Image)
	Play(clipName string) *common.AnimClipState
	GetClipState(clipName string) *common.AnimClipState
	GetFrameData() common.AnimExportFrame
	UpdateToFrame(frameIndex int)
	GetClips() []string
}
type AnimatorExportData struct {
	ClipsNames  []string
	AvatarImage string
}
type AnimationExportData struct {
	Frames []common.AnimExportFrame
}

const (
	AnimValTypeInt ANIMVALTYPE = iota
	AnimValTypeFloat
	AnimValTypeVector2
)

type Anim struct {
	Id         int
	Name       string
	fps        float64
	speedRatio float64
	totalframe int
	isloop     bool
	status     ANIMSTATUS

	currentFrame int
	preFrame     int

	preRepeatCount int
	//playing
	playingCallback func(int, bool, float64)
	//stop
	stopCallback func()
	//error
	errorCallback func(error)

	//tween
	easingFunction tools.IEasingFunction
	keys           []*AnimChannel
	evalValue      map[string]interface{}
}

var globalAnimId int = 1

func ReadAvatarConfig(fs spxfs.Dir, spriteDir string, avatarPath string) common.AvatarConfig {
	var avatarConfig common.AvatarConfig
	err := common.LoadJson(&avatarConfig, fs, spriteDir, avatarPath)
	if err != nil {
		log.Panicf("avatar config [%s] not exist", avatarPath)
	}
	return avatarConfig
}
func ReadAnimatorConfig(fs spxfs.Dir, spriteDir string, animatorPath string) common.AnimatorConfig {
	var avatarConfig common.AnimatorConfig
	err := common.LoadJson(&avatarConfig, fs, spriteDir, animatorPath)
	if err != nil {
		log.Panicf("animator config [%s] not exist", animatorPath)
	}
	return avatarConfig
}

// loopmodel = -1
func NewAnimator(fs spxfs.Dir, spriteDir string, animatorPath string, avatarPath string) IAnimator {
	config := ReadAnimatorConfig(fs, spriteDir, animatorPath)
	avatarConfig := ReadAvatarConfig(fs, spriteDir, avatarPath)

	var animator IAnimator
	if config.Type == ANIMATOR_TYPE_VERTEX {
		animator = vertex.NewAnimator(fs, spriteDir, &config, &avatarConfig)
	} else {
		animator = skeleton.NewAnimator(fs, spriteDir, &config, &avatarConfig)
	}
	return animator
}

func GetExportData(pself IAnimator, animName string) (*AnimationExportData, error) {
	data := &AnimationExportData{}
	if pself.GetClipState(animName) == nil {
		return data, errors.New("can not find animation clip " + animName)
	}
	state := pself.Play(animName)
	data.Frames = make([]common.AnimExportFrame, state.FrameCount)
	for i := 0; i < state.FrameCount; i++ {
		pself.UpdateToFrame(i)
		data.Frames[i] = pself.GetFrameData()
	}
	return data, nil
}

func NewAnim(name string, fps float64, totalframe int, isLoop bool) *Anim {
	this := &Anim{}

	this.Name = name
	this.fps = fps
	this.totalframe = totalframe
	this.isloop = isLoop
	this.status = AnimstatusPlaying

	this.speedRatio = 1.0
	this.Id = globalAnimId
	this.currentFrame = math.MaxInt32
	this.preFrame = math.MinInt32
	this.preRepeatCount = 0

	this.keys = make([]*AnimChannel, 0)
	this.evalValue = make(map[string]interface{}, 0)
	this.Id = globalAnimId
	globalAnimId++
	return this
}

func (this *Anim) AddChannel(name string, dataType ANIMVALTYPE, values []*AnimationKeyFrame) {
	animChan := NewAnimChannel(name, int(dataType), this.easingFunction, values)
	this.keys = append(this.keys, animChan)
}

func (a *Anim) Fps() float64 {
	return a.fps
}

func (a *Anim) Status() ANIMSTATUS {
	return a.status
}

func (a *Anim) SetLoop(isloop bool) *Anim {
	a.isloop = isloop
	return a
}

func (a *Anim) Play() *Anim {
	a.status = AnimstatusPlaying
	return a
}

func (a *Anim) Stop() *Anim {
	if a.status == AnimstatusStop {
		return a
	}
	a.status = AnimstatusStop
	if a.stopCallback != nil {
		a.stopCallback()
	}
	return a
}

func (this *Anim) Update(delay float64) bool {
	if this.status == AnimstatusStop {
		return false
	}
	// Check limits
	if this.totalframe == 0 {
		this.onStop()
		return false
	}
	if len(this.keys) == 0 {
		if this.errorCallback != nil {
			this.errorCallback(errors.New("this keys is empty"))
		}
		return false
	}
	// Animating
	from := 0
	to := this.totalframe
	// Compute ratio
	rangeval := float64(to + 1 - from)
	ratio := delay * float64(this.fps*this.speedRatio) / 1000.0
	repeatCount := int(ratio/rangeval) >> 0
	isReplay := repeatCount != this.preRepeatCount
	if isReplay {
		this.preRepeatCount = repeatCount
	}
	_, progress := math.Modf(ratio / rangeval)
	if ratio >= rangeval && !this.isloop { // If we are out of range and not looping get back to caller
		//add compete
		this.interpolate(to)
		if this.playingCallback != nil && this.preFrame != to {
			this.playingCallback(to, isReplay, 1)
		}

		//stop callback
		this.onStop()
		return false
	}

	this.currentFrame = from
	if rangeval != 0 {
		this.currentFrame = from + int(ratio)%int(rangeval)
	}
	//\\log.Printf("this.currentFrame %d, val %d, rangeval %g, delay %g, this.fps %g, speedRatio %f ratio %g", this.currentFrame, (int(ratio) % int(rangeval)), rangeval, delay, this.fps, speedRatio, ratio)

	if this.currentFrame == this.preFrame {
		//anti not stop
		return true
	}
	this.preFrame = this.currentFrame
	this.interpolate(this.currentFrame)

	if this.playingCallback != nil {
		this.playingCallback(this.currentFrame, isReplay, progress)
	}
	return true
}

func (this *Anim) SetEasingFunction(easingFunc tools.IEasingFunction) {
	this.easingFunction = easingFunc
	for _, key := range this.keys {
		key.SetEasingFunction(easingFunc)
	}
}

func (this *Anim) SetOnPlayingListener(playfuc func(int, bool, float64)) {
	this.playingCallback = playfuc
}

func (this *Anim) SetOnStopingListener(stopfuc func()) {
	this.stopCallback = stopfuc
}

func (this *Anim) SetOnErrorListener(errorfuc func(error)) {
	this.errorCallback = errorfuc
}

func (this *Anim) SampleChannel(name string) interface{} {
	return this.evalValue[name]
}

func (this *Anim) onStop() {
	if this.status == AnimstatusStop {
		return
	}
	if this.stopCallback != nil {
		this.stopCallback()
	}
}

func (this *Anim) interpolate(curFrame int) {
	for _, key := range this.keys {
		this.evalValue[key.Name] = key.interpolate(curFrame)
	}
}
