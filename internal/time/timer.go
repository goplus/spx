package time

var (
	gameTimer float64
)

func Tick(deltaTime float64) {
	gameTimer += deltaTime
}

func Timer() float64 {
	return gameTimer
}

func ResetTimer() {
	gameTimer = 0
}
