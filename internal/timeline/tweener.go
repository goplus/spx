package timeline

import (
	"log"
	"math"

	"github.com/goplus/spx/internal/math32"
)

var debugTweener bool = false

type TweenerMode int

const (
	CUR_TO_END TweenerMode = iota
	START_TO_END
	START_TO_CUR
)

type ITweener interface {
	ITimeline
	GetTweener() *Tweener
	// Rewind()
}

type Tweener struct {
	Timeline
	mode     TweenerMode
	getter   func() interface{}
	setter   func(interface{})
	startVal interface{}
	curVal   interface{} // only for START_TO_END mode
	endVal   interface{}
	duration float64
}

func (t *Tweener) GetTweener() *Tweener {
	return t
}

func (t *Tweener) Init__0(getter func() interface{}, setter func(interface{}), endVal interface{}, duration float64) *Tweener {
	t.mode = CUR_TO_END
	t.getter = getter
	t.setter = setter
	t.endVal = endVal
	if duration < 0.0 {
		duration = 0.0
	}
	t.duration = duration
	t.Timeline.Init()
	t.Timeline.onActive = t.onActive
	t.Timeline.onStep = func(time *float64) ITimeline {
		return t.onStep(time)
	}
	if debugTweener {
		log.Printf("Tweener.Init__0 %X", t.ID)
	}
	return t
}

func (t *Tweener) Init__1(setter func(interface{}), startVal interface{}, endVal interface{}, duration float64) *Tweener {
	t.mode = START_TO_END
	t.setter = setter
	t.startVal = startVal
	t.curVal = startVal
	t.endVal = endVal
	if duration < 0.0 {
		duration = 0.0
	}
	t.duration = duration
	t.Timeline.Init()
	t.Timeline.onActive = t.onActive
	t.Timeline.onStep = func(time *float64) ITimeline {
		return t.onStep(time)
	}
	if debugTweener {
		log.Printf("Tweener.Init__1 %X", t.ID)
	}
	return t
}

func (t *Tweener) Init__2(getter func() interface{}, setter func(interface{}), startVal interface{}, duration float64) *Tweener {
	t.mode = START_TO_CUR
	t.getter = getter
	t.setter = setter
	t.startVal = startVal
	t.duration = duration
	t.Timeline.Init()
	t.Timeline.onActive = t.onActive
	t.Timeline.onStep = func(time *float64) ITimeline {
		return t.onStep(time)
	}
	if debugTweener {
		log.Printf("Tweener.Init__2 %X", t.ID)
	}
	return t
}

func (t *Tweener) GetTimeline() *Timeline {
	return &t.Timeline
}

func (t *Tweener) onActive(active bool) {
	if debugTweener {
		log.Printf("Tweener.onActive %t %X", active, t.ID)
	}

	switch t.mode {
	case CUR_TO_END:
		if active {
			t.startVal = t.getter()
		} else {
			t.setter(t.endVal)
		}
	case START_TO_CUR:
		if active {
			t.endVal = t.getter()
			t.setter(t.startVal)
		} else {
			t.setter(t.endVal)
		}
	case START_TO_END:
		if active {
			t.setter(t.startVal)
		} else {
			t.setter(t.endVal)
		}
	}
}

func (t *Tweener) Step(time *float64) ITimeline {
	timeline := &t.Timeline
	running := timeline.Step(time)
	if running == timeline {
		return t
	} else {
		return running
	}
}

func (t *Tweener) onStep(time *float64) ITimeline {
	switch t.startVal.(type) {
	case float64:
		return t.onStepFloat64(time)
	case math32.Vector2:
		return t.onStepV2(time)
	case *math32.Vector2:
		panic("should pass Vector2, not *Vector2, check types of tweener.getter return and startVal and endVal")
	}
	return t
}

func (t *Tweener) onStepFloat64(time *float64) ITimeline {
	var start float64
	var ok bool
	if start, ok = t.startVal.(float64); !ok {
		return nil
	}
	var end float64
	if end, ok = t.endVal.(float64); !ok {
		return nil
	}

	switch t.mode {
	case CUR_TO_END, START_TO_CUR:
		delta := end - start
		if math.Abs(delta) < EPSILON {
			return nil
		}
		if t.duration < EPSILON {
			return nil
		}
		if t.getter == nil || t.setter == nil {
			return nil
		}
		cur, ok2 := t.getter().(float64)
		if !ok2 {
			return nil
		}

		speed := delta / t.duration
		need := math.Abs((end - cur) / speed)
		if *time < need {
			t.setter(cur + *time*speed)
			*time = 0.0
			return t
		} else {
			*time -= need
			t.setter(t.endVal)
			return nil
		}
	case START_TO_END:
		delta := end - start
		if math.Abs(delta) < EPSILON {
			return nil
		}
		if t.duration < EPSILON {
			return nil
		}
		if t.setter == nil {
			return nil
		}
		cur, ok2 := t.curVal.(float64)
		if !ok2 {
			return nil
		}

		speed := delta / t.duration
		need := math.Abs((end - cur) / speed)
		if *time < need {
			t.curVal = cur + *time*speed
			t.setter(t.curVal)
			*time = 0.0
			return t
		} else {
			*time -= need
			t.curVal = t.endVal
			t.setter(t.endVal)
			return nil
		}
	}
	return nil
}

func (t *Tweener) onStepV2(time *float64) ITimeline {
	var start math32.Vector2
	var ok bool
	if start, ok = t.startVal.(math32.Vector2); !ok {
		return nil
	}
	var end math32.Vector2
	if end, ok = t.endVal.(math32.Vector2); !ok {
		return nil
	}

	switch t.mode {
	case CUR_TO_END, START_TO_CUR:
		delta := (&end).Sub(&start)
		if delta.LengthSquared() < EPSILON {
			return nil
		}
		if t.duration < EPSILON {
			return nil
		}
		if t.getter == nil || t.setter == nil {
			return nil
		}
		cur, ok2 := t.getter().(math32.Vector2)
		if !ok2 {
			return nil
		}
		speed := delta.Scale(1.0 / t.duration)
		need := math.Abs((&end).Sub(&cur).Length() / speed.Length())
		if *time < need {
			v2 := (&cur).Add((speed.Scale(*time)))
			t.setter(*v2)
			*time = 0.0
			return t
		} else {
			*time -= need
			t.setter(t.endVal)
			return nil
		}
	case START_TO_END:
		delta := (&end).Sub(&start)
		if delta.LengthSquared() < EPSILON {
			return nil
		}
		if t.duration < EPSILON {
			return nil
		}
		if t.setter == nil {
			return nil
		}
		cur, ok2 := t.curVal.(math32.Vector2)
		if !ok2 {
			return nil
		}

		speed := delta.Scale(1.0 / t.duration)
		need := math.Abs((&end).Sub(&cur).Length() / speed.Length())
		if *time < need {
			t.curVal = *((&cur).Add(speed.Scale(*time)))
			t.setter(t.curVal)
			*time = 0.0
			return t
		} else {
			*time -= need
			t.curVal = t.endVal
			t.setter(t.endVal)
			return nil
		}
	}
	return nil
}

func (t *Tweener) From() ITweener {
	newT := &Tweener{}
	switch t.mode {
	case CUR_TO_END:
		newT.Init__2(t.getter, t.setter, t.endVal, t.duration)
	case START_TO_CUR:
		newT.Init__1(t.setter, t.endVal, t.startVal, t.duration)
	case START_TO_END:
		newT.Init__0(t.getter, t.setter, t.startVal, t.duration)
	}
	return newT
}
