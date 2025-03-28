package engine

import (
	"math"
)

func Abs(x float64) float64 {
	return float64(math.Abs(float64(x)))
}
func Sign(x float64) int64 {
	if x < 0 {
		return -1
	} else if x > 0 {
		return 1
	}
	return 0
}
