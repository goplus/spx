package engine

import (
	"math"

	mathf "github.com/goplus/spbase/mathf"
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

func DegToRad(degrees float64) float64 {
	return degrees * (math.Pi / 180.0)
}

func RadToDeg(radians float64) float64 {
	return radians * (180.0 / math.Pi)
}

func AngleToPoint(v mathf.Vec2, v2 mathf.Vec2) float64 {
	return Angle(v.Sub(v2))
}

// Angle returns the angle of v in radians, measured from the positive X axis.
func Angle(v mathf.Vec2) float64 {
	return float64(mathf.Atan2(float64(v.Y), float64(v.X)))
}
