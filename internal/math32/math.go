// Copyright 2016 The G3N Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package math32 implements basic math functions which operate
// directly on float64 numbers without casting and contains
// types of common entities used in 3D Graphics such as vectors,
// matrices, quaternions and others.
package math32

import (
	"image"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
)

const Pi = math.Pi
const degreeToRadiansFactor = math.Pi / 180
const radianToDegreesFactor = 180.0 / math.Pi

var Infinity = float64(math.Inf(1))

// DegToRad converts a number from degrees to radians
func DegToRad(degrees float64) float64 {

	return degrees * degreeToRadiansFactor
}

// RadToDeg converts a number from radians to degrees
func RadToDeg(radians float64) float64 {

	return radians * radianToDegreesFactor
}

// Clamp clamps x to the provided closed interval [a, b]
func Clamp(x, a, b float64) float64 {

	if x < a {
		return a
	}
	if x > b {
		return b
	}
	return x
}

// ClampInt clamps x to the provided closed interval [a, b]
func ClampInt(x, a, b int) int {

	if x < a {
		return a
	}
	if x > b {
		return b
	}
	return x
}

func Abs(v float64) float64 {
	return float64(math.Abs(float64(v)))
}

func Acos(v float64) float64 {
	return float64(math.Acos(float64(v)))
}

func Asin(v float64) float64 {
	return float64(math.Asin(float64(v)))
}

func Atan(v float64) float64 {
	return float64(math.Atan(float64(v)))
}

func Atan2(y, x float64) float64 {
	return float64(math.Atan2(float64(y), float64(x)))
}

func Ceil(v float64) float64 {
	return float64(math.Ceil(float64(v)))
}

func Cos(v float64) float64 {
	return float64(math.Cos(float64(v)))
}

func Floor(v float64) float64 {
	return float64(math.Floor(float64(v)))
}

func Inf(sign int) float64 {
	return float64(math.Inf(sign))
}

func Round(v float64) float64 {
	return Floor(v + 0.5)
}

func IsNaN(v float64) bool {
	return math.IsNaN(float64(v))
}

func Sin(v float64) float64 {
	return float64(math.Sin(float64(v)))
}

func Sqrt(v float64) float64 {
	return float64(math.Sqrt(float64(v)))
}

func Max(a, b float64) float64 {
	return float64(math.Max(float64(a), float64(b)))
}

func Min(a, b float64) float64 {
	return float64(math.Min(float64(a), float64(b)))
}

func Mod(a, b float64) float64 {
	return float64(math.Mod(float64(a), float64(b)))
}

func NaN() float64 {
	return float64(math.NaN())
}

func Pow(a, b float64) float64 {
	return float64(math.Pow(float64(a), float64(b)))
}

func Tan(v float64) float64 {
	return float64(math.Tan(float64(v)))
}

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
