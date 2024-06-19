package anim

type ANIMSTATUS uint8

const (
	AnimstatusPlaying ANIMSTATUS = iota
	AnimstatusStop
)

type ANIMVALTYPE uint8

const (
	AnimValTypeInt ANIMVALTYPE = iota
	AnimValTypeFloat
	AnimValTypeVector2
)

type Anim struct {
	Id           int
	Name         string
	fps          float64
	totalframe   int
	isloop       bool
	status       ANIMSTATUS
	animation    *Animation
	keyframelist []*AnimationKeyFrame

	//playing
	playingCallback func(int, bool, interface{})
	//stop
	stopCallback func()
}

func NewAnim(name string, valtype ANIMVALTYPE, fps float64, totalframe int) *Anim {
	a := &Anim{
		Name:         name,
		fps:          fps,
		totalframe:   totalframe,
		isloop:       false,
		status:       AnimstatusPlaying,
		animation:    nil,
		keyframelist: make([]*AnimationKeyFrame, 0),
	}
	a.animation = NewAnimation(name, fps, (int)(valtype), ANIMATIONLOOPMODE_CYCLE)
	a.Id = int(a.animation.GetAnimId())
	a.animation.SetOnPlayingListener(func(an *Animation, currframe int, isReplay bool, currval interface{}) {
		if a.playingCallback != nil {
			a.playingCallback(currframe, isReplay, currval)
		}
	})
	a.animation.SetOnStopingListener(func(an *Animation) {
		if a.status == AnimstatusStop {
			return
		}
		if a.stopCallback != nil {
			a.stopCallback()
		}
	})
	return a
}

func (a *Anim) AddKeyFrame(frameindex int, frameval interface{}) *Anim {
	a.keyframelist = append(a.keyframelist, &AnimationKeyFrame{
		Frame: frameindex,
		Value: frameval,
	})
	a.animation.SetKeys(a.keyframelist)

	return a
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

func (a *Anim) SetOnPlayingListener(playfuc func(int, bool, interface{})) *Anim {
	a.playingCallback = playfuc
	return a
}

func (a *Anim) SetOnStopingListener(stopfuc func()) *Anim {
	a.stopCallback = stopfuc
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

func (a *Anim) Update(delay float64) bool {
	if a.status == AnimstatusStop {
		return false
	}
	// Animating
	ret := a.animation.Animate(nil, delay, 0, a.totalframe, a.isloop, 1.0)

	return ret
}
