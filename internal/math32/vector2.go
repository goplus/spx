package math32

import (
	"fmt"
	"math"
)

type Vector2 struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

func NewVector2(x, y float64) *Vector2 {
	return &Vector2{x, y}
}

func NewVector2Zero() *Vector2 {
	return &Vector2{0, 0}
}

func (pself *Vector2) Set(x, y float64) {
	pself.X = x
	pself.Y = y
}

func (pself *Vector2) String() string {
	return fmt.Sprintf("(%f,%f)", pself.X, pself.Y)
}

func (pself *Vector2) Coords() (float64, float64) {
	return pself.X, pself.Y
}

func (pself *Vector2) Add(otherVector *Vector2) *Vector2 {
	return &Vector2{pself.X + otherVector.X,
		pself.Y + otherVector.Y}
}

func (pself *Vector2) Sub(otherVector *Vector2) *Vector2 {
	return &Vector2{pself.X - otherVector.X,
		pself.Y - otherVector.Y}
}

func (pself *Vector2) Scale(scale float64) *Vector2 {
	return &Vector2{pself.X * scale,
		pself.Y * scale}
}

func (pself *Vector2) Equals(otherVector *Vector2) bool {
	return pself.X == otherVector.X && pself.Y == otherVector.Y
}

func (pself *Vector2) Multiply(otherVector *Vector2) *Vector2 {
	return &Vector2{pself.X * otherVector.X,
		pself.Y * otherVector.Y}
}

func (pself *Vector2) Length() float64 {
	return math.Sqrt(pself.X*pself.X + pself.Y*pself.Y)
}

func (pself *Vector2) LengthSquared() float64 {
	return (pself.X*pself.X + pself.Y*pself.Y)
}

func (pself *Vector2) Cross(otherVector *Vector2) float64 {
	return (pself.X*otherVector.Y - pself.Y*otherVector.X)
}

func (pself *Vector2) Ddot(otherVector *Vector2) float64 {
	return (pself.X*otherVector.X + pself.Y*otherVector.Y)
}

func (pself *Vector2) Normalize() {
	len := pself.Length()
	if len == 0 {
		return
	}
	num := 1.0 / len
	pself.X *= num
	pself.Y *= num
}

func (pself *Vector2) Clone() *Vector2 {
	return &Vector2{pself.X,
		pself.Y}
}

func (pself *Vector2) CopyFrom(source *Vector2) {
	pself.X = source.X
	pself.Y = source.Y
}

func (pself *Vector2) Lerp(end *Vector2, amount float64) *Vector2 {
	x := pself.X + ((end.X - pself.X) * amount)
	y := pself.Y + ((end.Y - pself.Y) * amount)
	return NewVector2(x, y)
}

// Receiver is returned for easy chaning.
func (pself *Vector2) Invert() *Vector2 {
	pself.X = -pself.X
	pself.Y = -pself.Y
	return pself
}

// Inverted returns a new *T which is receiver inverted.
func (pself *Vector2) Inverted() *Vector2 {
	p := *pself
	return p.Invert()
}
