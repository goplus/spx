package math32

import (
	"fmt"
	"math"
)

type Vector3 struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

func NewVector3(x, y, z float64) *Vector3 {
	return &Vector3{x, y, z}
}

func NewVector3Zero() *Vector3 {
	return &Vector3{0, 0, 0}
}

func (pself *Vector3) Set(x, y, z float64) {
	pself.X = x
	pself.Y = y
	pself.Z = z
}

func (pself *Vector3) String() string {
	return fmt.Sprintf("(%f,%f,%f)", pself.X, pself.Y, pself.Z)
}

func (pself *Vector3) Coords() (float64, float64, float64) {
	return pself.X, pself.Y, pself.Z
}

func (pself *Vector3) Add(otherVector *Vector3) *Vector3 {
	return &Vector3{pself.X + otherVector.X, pself.Y + otherVector.Y, pself.Z + otherVector.Z}
}

func (pself *Vector3) Sub(otherVector *Vector3) *Vector3 {
	return &Vector3{pself.X - otherVector.X,
		pself.Y - otherVector.Y, pself.Z - otherVector.Z}
}

func (pself *Vector3) Scale(scale float64) *Vector3 {
	return &Vector3{pself.X * scale,
		pself.Y * scale, pself.Z * scale}
}

func (pself *Vector3) Equals(otherVector *Vector3) bool {
	return pself.X == otherVector.X && pself.Y == otherVector.Y && pself.Z == otherVector.Z
}

func (pself *Vector3) Multiply(otherVector *Vector3) *Vector3 {
	return &Vector3{pself.X * otherVector.X,
		pself.Y * otherVector.Y, pself.Z * otherVector.Z}
}

func (pself *Vector3) Length() float64 {
	return math.Sqrt(pself.X*pself.X + pself.Y*pself.Y + pself.Z*pself.Z)
}

func (pself *Vector3) LengthSquared() float64 {
	return (pself.X*pself.X + pself.Y*pself.Y + pself.Z*pself.Z)
}

func (pself *Vector3) Cross(otherVector *Vector3) Vector3 {
	return Vector3{pself.Y*otherVector.Z - pself.Z*otherVector.Y,
		pself.Z*otherVector.X - pself.X*otherVector.Z,
		pself.X*otherVector.Y - pself.Y*otherVector.X}
}

func (pself *Vector3) Ddot(otherVector *Vector3) float64 {
	return (pself.X*otherVector.X + pself.Y*otherVector.Y + pself.Z*otherVector.Z)
}

func (pself *Vector3) Normalize() {
	len := pself.Length()
	if len == 0 {
		return
	}
	num := 1.0 / len
	pself.X *= num
	pself.Y *= num
	pself.Z *= num
}

func (pself *Vector3) Clone() *Vector3 {
	return &Vector3{pself.X, pself.Y, pself.Z}
}

func (pself *Vector3) CopyFrom(source *Vector3) {
	pself.X = source.X
	pself.Y = source.Y
	pself.Z = source.Z
}

func (pself *Vector3) Lerp(end *Vector3, amount float64) *Vector3 {
	x := pself.X + ((end.X - pself.X) * amount)
	y := pself.Y + ((end.Y - pself.Y) * amount)
	z := pself.Z + ((end.Z - pself.Z) * amount)
	return NewVector3(x, y, z)
}

// Receiver is returned for easy chaning.
func (pself *Vector3) Invert() *Vector3 {
	pself.X = -pself.X
	pself.Y = -pself.Y
	pself.Z = -pself.Z
	return pself
}

// Inverted returns a new *T which is receiver inverted.
func (pself *Vector3) Inverted() *Vector3 {
	p := *pself
	return p.Invert()
}
