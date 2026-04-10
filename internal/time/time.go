package time

import (
	stdtime "time"
)

const DefaultFPS = 30

var (
	realTimeSinceLevelLoad float64
	timeSinceLevelLoad     float64
	deltaTime              float64
	realDeltaTime          float64
	timeScale              float64
	curFrame               int64
	setTimeScaleCallback   func(float64)
	startTimestamp         stdtime.Time
	lastTimestamp          stdtime.Time
	fps                    float64
)

func Sleep(ms float64) {
	stdtime.Sleep(stdtime.Microsecond * stdtime.Duration(ms*1000))
}

func RealTimeSinceStart() float64 {
	return stdtime.Since(startTimestamp).Seconds()
}

func RealTimeSinceCurFrame() float64 {
	return stdtime.Since(lastTimestamp).Seconds()
}

func RealTimeSinceCurFrameMs() float64 {
	return RealTimeSinceCurFrame() * 1000
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
	return realDeltaTime
}

func UnscaledTimeSinceLevelLoad() float64 {
	return realTimeSinceLevelLoad
}

func TimeSinceLevelLoad() float64 {
	return timeSinceLevelLoad
}

func Start(setTimeScaleCB func(float64)) {
	now := stdtime.Now()

	realTimeSinceLevelLoad, timeSinceLevelLoad = 0, 0
	deltaTime, realDeltaTime = 0, 0
	timeScale = 1
	curFrame = 0
	fps = DefaultFPS

	setTimeScaleCallback = setTimeScaleCB
	startTimestamp, lastTimestamp = now, now
	ResetTimer()
}

func Update(delta float64, pfps float64) {
	curTime := stdtime.Now()
	realTimeSinceLevelLoad = curTime.Sub(startTimestamp).Seconds()
	realDeltaTime = curTime.Sub(lastTimestamp).Seconds()
	lastTimestamp = curTime

	deltaTime = delta
	timeSinceLevelLoad += delta
	curFrame += 1
	fps = pfps
}
