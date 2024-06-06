package engine

import (
	"time"
)

var Time = newTime()

type TimeInfo struct {
	lastUpdate         time.Time
	DeltaTime          float64
	Time               float64
	TimeSinceLevelLoad float64
}

func newTime() *TimeInfo {
	return &TimeInfo{
		lastUpdate:         time.Now(),
		DeltaTime:          0,
		Time:               0,
		TimeSinceLevelLoad: 0,
	}
}

func (t *TimeInfo) OnLevelLoaded() {
	t.TimeSinceLevelLoad = 0
}

func (t *TimeInfo) Update() {
	now := time.Now()
	t.DeltaTime = now.Sub(t.lastUpdate).Seconds()
	t.Time += t.DeltaTime
	t.TimeSinceLevelLoad += t.DeltaTime
	t.lastUpdate = now
}
