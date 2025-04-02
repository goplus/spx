package engine

import (
	. "github.com/realdream-ai/mathf"
)

var (
	Math_PI = float64(3.1415926535897932384626433833)
)

func DegToRad(p_y float64) float64 {
	return p_y * (Math_PI / 180.0)
}

func RadToDeg(p_y float64) float64 {
	return p_y * (180.0 / Math_PI)
}
func AngleToPoint(v Vec2, v2 Vec2) float64 {
	return Angle(v.Sub(v2))
}

func Angle(v Vec2) float64 {
	return float64(Atan2(float64(v.Y), float64(v.X)))
}
