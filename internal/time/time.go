package time

import (
	stdtime "time"
)

const defaultFPS = 30

var (
	unscaledTimeSinceLevelLoad float64
	timeSinceLevelLoad         float64
	deltaTime                  float64
	unscaledDeltaTime          float64
	timeScale                  float64
	curFrame                   int64
	setTimeScaleCallback       func(float64)
	startTimestamp             stdtime.Time
	lastTimestamp              stdtime.Time
	fps                        float64
	curFrameRealTimeSinceStart float64
)

func Sleep(ms float64) {
	stdtime.Sleep(stdtime.Microsecond * stdtime.Duration(ms*1000))
}

func RealTimeSinceStart() float64 {
	return stdtime.Since(startTimestamp).Seconds()
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

func UnscaledTimeSinceLevelLoad() float64 {
	return unscaledTimeSinceLevelLoad
}

func TimeSinceLevelLoad() float64 {
	return timeSinceLevelLoad
}

func Start(setTimeScaleCB func(float64)) {
	unscaledTimeSinceLevelLoad = 0
	timeSinceLevelLoad = 0
	deltaTime = 0
	unscaledDeltaTime = 0
	timeScale = 1
	curFrame = 0
	fps = defaultFPS
	curFrameRealTimeSinceStart = 0
	setTimeScaleCallback = setTimeScaleCB
	now := stdtime.Now()
	startTimestamp = now
	lastTimestamp = now
	ResetTimer()
}

func Update(delta float64, pfps float64) {
	timeSinceLevelLoad += delta

	curTime := stdtime.Now()
	unscaledTimeSinceLevelLoad := curTime.Sub(startTimestamp).Seconds()
	unscaledDeltaTime := curTime.Sub(lastTimestamp).Seconds()
	lastTimestamp = curTime

	applyFrameUpdate(unscaledTimeSinceLevelLoad, timeSinceLevelLoad, delta, unscaledDeltaTime, pfps)
}

func applyFrameUpdate(realDuration float64, duration float64, delta float64, unscaledDelta float64, pfps float64) {
	unscaledDeltaTime = unscaledDelta
	unscaledTimeSinceLevelLoad = realDuration
	timeSinceLevelLoad = duration
	deltaTime = delta
	curFrame += 1
	fps = pfps
	curFrameRealTimeSinceStart = realDuration
}
