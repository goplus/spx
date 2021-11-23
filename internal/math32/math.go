package math32

import (
	"image"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

func ApplyGeoForVector2(p *Vector2, op *ebiten.GeoM) *Vector2 {
	x, y := op.Apply(float64(p.X), float64(p.Y))
	return NewVector2(x, y)
}

func ApplyGeoForRotatedRect(rect image.Rectangle, op *ebiten.GeoM) *RotatedRect {

	v1 := &Vector2{
		X: float64(rect.Min.X),
		Y: float64(rect.Min.Y),
	}

	v2 := &Vector2{
		X: float64(rect.Min.X),
		Y: float64(rect.Max.Y),
	}

	v3 := &Vector2{
		X: float64(rect.Max.X),
		Y: float64(rect.Max.Y),
	}

	v4 := &Vector2{
		X: float64(rect.Max.X),
		Y: float64(rect.Min.Y),
	}

	varr := make([]*Vector2, 0)
	varr = append(varr, ApplyGeoForVector2(v1, op))
	varr = append(varr, ApplyGeoForVector2(v2, op))
	varr = append(varr, ApplyGeoForVector2(v3, op))
	varr = append(varr, ApplyGeoForVector2(v4, op))

	rRect := NewRotatedRect3(varr[0], varr[1], varr[2])
	return rRect
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
	return checkAxisRadius+targetAxisRadius > centerPointRadius
}
