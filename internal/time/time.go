package time

import (
	stime "time"

	"github.com/goplus/spx/internal/timer"
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
	curFrameRealTimeSinceStart float64
)

func Sleep(ms float64) {
	stime.Sleep(stime.Microsecond * stime.Duration((ms * 1000)))
}

func RealTimeSinceStart() float64 {
	return stime.Since(startTimestamp).Seconds()
}

func RealTimeSinceCurFrame() float64 {
	return RealTimeSinceStart() - curFrameRealTimeSinceStart
}

func RealTimeSinceCurFrameMs() float64 {
	return (RealTimeSinceStart() - curFrameRealTimeSinceStart) * 1000
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
	curFrameRealTimeSinceStart = RealTimeSinceStart()
	timer.OnUpdate(deltaTime)
}
