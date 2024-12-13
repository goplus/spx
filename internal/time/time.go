package time

import (
	"fmt"
	stime "time"
)

var (
	unscaledTimeSinceLevelLoad float64
	timeSinceLevelLoad         float64
	deltaTime                  float64
	unscaledDeltaTime          float64
	timeScale                  float64
	curFrame                   int64
	setTimeScaleCallback       func(float64)
	startTimestamp             stime.Time
	fps                        float64
)

func Sleep(ms float64) {
	stime.Sleep(stime.Duration(ms * float64(stime.Millisecond)))
}

func RealTimeSinceStartStr() string {
	return fmt.Sprintf("%f", RealTimeSinceStart())
}

func RealTimeSinceStart() float64 {
	return stime.Since(startTimestamp).Seconds()
}
func FPS() float64 {
	return fps
}
func Frame() int64 {
	return curFrame
}

func TimeScale() float64 {
	return timeScale
}

func SetTimeScale(value float64) {
	if setTimeScaleCallback != nil {
		setTimeScaleCallback(value)
	}
	timeScale = value
}

func DeltaTime() float64 {
	return deltaTime
}

func UnscaledDeltaTime() float64 {
	return unscaledDeltaTime
}

// no time scale
func UnscaledTimeSinceLevelLoad() float64 {
	return unscaledTimeSinceLevelLoad
}

func TimeSinceLevelLoad() float64 {
	return timeSinceLevelLoad
}

func Start(setTimeScaleCB func(float64)) {
	Update(1, 0, 0, 0, 0, 30)
	setTimeScaleCallback = setTimeScaleCB
	startTimestamp = stime.Now()
}

func Update(scale float64, realDuration float64, duration float64, delta float64, unscaledDelta float64, pfps float64) {
	timeScale = scale
	unscaledDeltaTime = unscaledDelta
	unscaledTimeSinceLevelLoad = realDuration
	timeSinceLevelLoad = duration
	deltaTime = delta
	curFrame += 1
	fps = pfps
}
