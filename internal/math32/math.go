package math32

import (
	"image"

	"github.com/hajimehoshi/ebiten/v2"
)

func ApplyGeoForPoint(p image.Point, op *ebiten.GeoM) image.Point {
	x, y := op.Apply(float64(p.X), float64(p.Y))
	return image.Point{X: int(x), Y: int(y)}
}

func ApplyGeoForRect(rect image.Rectangle, op *ebiten.GeoM) image.Rectangle {
	boxrect := image.Rectangle{}
	v1 := image.Point{
		X: rect.Min.X,
		Y: rect.Min.Y,
	}

	v2 := image.Point{
		X: rect.Min.X,
		Y: rect.Max.Y,
	}

	v3 := image.Point{
		X: rect.Max.X,
		Y: rect.Max.Y,
	}

	v4 := image.Point{
		X: rect.Max.X,
		Y: rect.Min.Y,
	}

	varr := make([]image.Point, 0)
	varr = append(varr, ApplyGeoForPoint(v1, op))
	varr = append(varr, ApplyGeoForPoint(v2, op))
	varr = append(varr, ApplyGeoForPoint(v3, op))
	varr = append(varr, ApplyGeoForPoint(v4, op))

	boxrect.Min = varr[0]
	boxrect.Max = varr[0]
	for _, v := range varr {
		if boxrect.Min.X > v.X {
			boxrect.Min.X = v.X
		}
		if boxrect.Min.Y > v.Y {
			boxrect.Min.Y = v.Y
		}
		if boxrect.Max.X < v.X {
			boxrect.Max.X = v.X
		}
		if boxrect.Max.Y < v.Y {
			boxrect.Max.Y = v.Y
		}
	}

	return boxrect
}
