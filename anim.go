package spx

import (
	"github.com/goplus/spx/internal/anim"
	"github.com/goplus/spx/internal/tools"
)

type ANIMSTATUS uint8

const (
	ANIMSTATUS_PLAYING ANIMSTATUS = iota
	ANIMSTATUS_STOP
)

type ANIMVALTYPE uint8

const (
	ANIMVALTYPE_INT ANIMVALTYPE = iota
	ANIMVALTYPE_FLOAT
	ANIMVALTYPE_VECTOR2
)

type Anim struct {
	id                   int
	name                 string
	fps                  float64
	totalframe           int
	isloop               bool
	status               ANIMSTATUS
	animation            *anim.Animation
	keyframelist         []*anim.AnimationKeyFrame
	animationStartedDate int

	//playing
	playingCallback func(int, interface{})
	//stop
	stopCallback func()
}

func NewAnim(name string, valtype ANIMVALTYPE, fps float64, totalframe int) *Anim {
	a := &Anim{
		name:                 name,
		fps:                  fps,
		totalframe:           totalframe,
		isloop:               false,
		status:               ANIMSTATUS_PLAYING,
		animation:            nil,
		animationStartedDate: 0,
		keyframelist:         make([]*anim.AnimationKeyFrame, 0),
	}
	a.animation = anim.NewAnimation(name, fps, (int)(valtype), anim.ANIMATIONLOOPMODE_CYCLE)
	a.id = int(a.animation.GetAnimId())
	a.animation.SetOnPlayingListener(func(an *anim.Animation, currframe int, currval interface{}) {
		if a.playingCallback != nil {
			a.playingCallback(currframe, currval)
		}
	})
	a.animation.SetOnStopingListener(func(an *anim.Animation) {
		if a.stopCallback != nil {
			a.stopCallback()
		}
	})
	return a
}
func (a *Anim) AddKeyFrame(frameindex int, frameval interface{}) *Anim {
	a.keyframelist = append(a.keyframelist, &anim.AnimationKeyFrame{
		Frame: frameindex,
		Value: frameval,
	})
	a.animation.SetKeys(a.keyframelist)

	return a
}

func (a *Anim) SetLoop(isloop bool) *Anim {
	a.isloop = isloop
	return a
}

func (a *Anim) SetOnPlayingListener(playfuc func(int, interface{})) *Anim {
	a.playingCallback = playfuc
	return a
}

func (a *Anim) SetOnStopingListener(stopfuc func()) *Anim {
	a.stopCallback = stopfuc
	return a
}

func (a *Anim) Play() *Anim {
	a.status = ANIMSTATUS_PLAYING
	return a
}

func (a *Anim) Stop() *Anim {
	a.status = ANIMSTATUS_STOP
	if a.stopCallback != nil {
		a.stopCallback()
	}
	return a
}

//
func (a *Anim) update() bool {
	if a.status == ANIMSTATUS_STOP {
		return false
	}
	if a.animationStartedDate == 0 {
		a.animationStartedDate = tools.GetCurrentTimeMs()
	}
	//Getting time
	var delay float64
	delay = (float64)(tools.GetCurrentTimeMs() - a.animationStartedDate)
	// Animating
	ret := a.animation.Animate(nil, delay, 0, a.totalframe, a.isloop, 1.0)

	return ret
}
