package time

var (
	realTimeSinceStartup float64
	timeSinceLevelLoad   float64
	deltaTime            float64
	unscaledDeltaTime    float64
	timeScale            float64
	curFrame             int64
	setTimeScaleCallback func(float64)
)

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
func RealTimeSinceStartup() float64 {
	return realTimeSinceStartup
}

func TimeSinceLevelLoad() float64 {
	return timeSinceLevelLoad
}

func Start(setTimeScaleCB func(float64)) {
	Update(1, 0, 0, 0, 0)
	setTimeScaleCallback = setTimeScaleCB
}

func Update(scale float64, realDuration float64, duration float64, delta float64, unscaledDelta float64) {
	timeScale = scale
	unscaledDeltaTime = unscaledDelta
	realTimeSinceStartup = realDuration
	timeSinceLevelLoad = duration
	deltaTime = delta
	curFrame += 1
}
