package time

var (
	gameTimer float64
)

func Timer() float64 {
	return gameTimer
}

func ResetTimer() {
	gameTimer = 0
}
