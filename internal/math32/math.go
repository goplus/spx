package math32

import (
	"log"
	"math"
)

func Lerp(a float64, b float64, progress float64) float64 {
	return a + (b-a)*progress
}
func LerpVec2(a *Vector2, b *Vector2, progress float64) Vector2 {
	vec := Vector2{}
	vec.X = Lerp(a.X, b.X, progress)
	vec.Y = Lerp(a.Y, b.Y, progress)
	return vec
}

func Clamp01(a float64) float64 {
	if a < 0 {
		return 0
	}
	if a > 1 {
		return 1
	}
	return a
}

func Clamp(curr, min, max float64) float64 {
	curr = math.Max(curr, min)
	curr = math.Min(curr, max)
	return curr
}

func GetProjectionRadius(checkAxis, axis *Vector2) float64 {
	return math.Abs(axis.X*checkAxis.X + axis.Y*checkAxis.Y)
}

func IsCover(vp1p2 *Vector2, checkAxisRadius float64, deg float64, targetAxis1 *Vector2, targetAxis2 *Vector2) bool {

	checkAxis := NewVector2(math.Cos(deg), math.Sin(deg))
	targetAxisRadius := (GetProjectionRadius(checkAxis, targetAxis1) + GetProjectionRadius(checkAxis, targetAxis2)) / 2
	centerPointRadius := GetProjectionRadius(checkAxis, vp1p2)
	log.Printf("checkAxis %s, (%f,%f,%f)", checkAxis.String(), checkAxisRadius, targetAxisRadius, centerPointRadius)
	return checkAxisRadius+targetAxisRadius > centerPointRadius
}
