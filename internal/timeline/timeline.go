package timeline

import (
	"fmt"
	"log"
	"sync"
)

const EPSILON float64 = 0.0001
const DEFAULT_TRANSITION float64 = 0.5

var debugTimeline bool = false
var idSeed int64
var idSeedM sync.Mutex

type ITimeline interface {
	GetTimeline() *Timeline
	Step(time *float64) ITimeline
	SetActive(bool)
}

type Timeline struct {
	ID           int64
	active       bool
	offset       float64
	speed        float64
	fadeIn       *Interval
	fadeOut      *Interval
	freezingTime float64
	next         ITimeline
	group        *TimelineGroup
	onStep       func(*float64) ITimeline
	onActive     func(bool)
}

func (t *Timeline) Init() *Timeline {
	idSeedM.Lock()
	defer idSeedM.Unlock()
	t.ID = idSeed
	idSeed++
	if debugTimeline {
		log.Printf("Timeline %X", t.ID)
	}
	t.speed = 1.0
	return t
}

func (t *Timeline) GetTimeline() *Timeline {
	return t
}

func (t *Timeline) SetActive(on bool) {
	if t.active == on {
		return
	}
	if debugTimeline {
		log.Println("SetActive", on, t.ID)
	}
	t.active = on
	if t.onActive != nil {
		t.onActive(on)
	}
}

func (t *Timeline) Step(time *float64) ITimeline {
	if *time < 0.0 {
		*time = 0.0
	}

	var running ITimeline = t
	var loopLimit int = 10000
	for running != nil && *time > EPSILON && loopLimit > 0 {
		loopLimit--
		var realStep, scaledTime, oldScaledTime float64
		var oldRunning ITimeline
		r := running.GetTimeline()

		// step until time <= 0 || offset <= 0
		var min float64 = *time // minimum step in consideration of time, offset, fadeout.end, freezingTime
		if r.offset > EPSILON {
			if min > r.offset {
				min = r.offset
			}
			*time -= min
			r.offset -= min
			continue
		}

		r.SetActive(true)

		if r.fadeOut != nil {
			end := r.fadeOut.End()
			if end < 0.0 {
				end = 0.0
			}
			if min > end {
				min = end
			}
		}

		// step until time <= 0 || fadeOut.end <= 0 || freezingTime <= 0
		if r.freezingTime > EPSILON {
			if min > r.freezingTime {
				min = r.freezingTime
			}
			*time -= min
			r.freezingTime -= min
			if r.fadeIn != nil {
				r.fadeIn.Step(min)
			}
			if r.fadeOut != nil {
				r.fadeOut.Step(min)
			}

			goto CHECK_FADE_OUT
		}

		// step until time <= 0 || fadeOut.end <= 0 || runOut.end <= 0
		scaledTime = min * r.speed
		oldScaledTime = scaledTime
		oldRunning = running
		running = r.onStep(&scaledTime)
		if r.speed > EPSILON {
			realStep = (oldScaledTime - scaledTime) / r.speed
		} else {
			realStep = min
		}
		*time -= realStep
		if r.fadeIn != nil {
			r.fadeIn.Step(realStep)
		}
		if r.fadeOut != nil {
			r.fadeOut.Step(realStep)
		}

		if running != oldRunning {
			if running != nil {
				continue
			}

			// running == null, means the timeline has run out, should check whether fade out
			if r.fadeOut == nil || r.fadeOut.End() <= EPSILON {
				oldRunning.SetActive(false)
				running = r.next
				continue
			}

			// step until time <= 0 || fadeOut.end <= 0
			end2 := r.fadeOut.End()
			if end2 < 0.0 {
				end2 = 0.0
			}
			min2 := *time
			if min2 > end2 {
				min2 = end2
			}
			*time -= min2
			if r.fadeIn != nil {
				r.fadeIn.Step(min2)
			}
			if r.fadeOut != nil {
				r.fadeOut.Step(min2)
			}
			running = oldRunning
		}

	CHECK_FADE_OUT:
		t2 := running.GetTimeline()
		if t2.fadeOut != nil && t2.fadeOut.End() <= EPSILON {
			t2.SetActive(false)
			running = t2.next
		}
	}

	if loopLimit <= 0 {
		panic("LOOP LIMIT")
	}
	return running
}

func (t *Timeline) String() string {
	var nID int64 = 0
	if t.next != nil {
		nID = t.next.GetTimeline().ID
	}
	var gID int64 = 0
	if t.group != nil {
		gID = t.group.ID
	}
	return fmt.Sprintf("{id:%x,active:%t,offset:%.3f,speed:%.3f,in:%s,out:%s,frz:%.3f,next:%x,group:%x}",
		t.ID, t.active, t.offset, t.speed, t.fadeIn, t.fadeOut, t.freezingTime, nID, gID)
}
