package math32

import "fmt"

type Vector2 struct {
	X float64
	Y float64
}

func NewVector2(x, y float64) *Vector2 {
	return &Vector2{x, y}
}
func NewVector2Zero() *Vector2 {

	return &Vector2{0, 0}
}

func (this *Vector2) Set(x, y float64) {
	this.X = x
	this.Y = y
}
func (this *Vector2) String() string {
	return fmt.Sprintf("%f,%f\n", this.X, this.Y)
}

func (this *Vector2) Coords() (float64, float64) {
	return this.X, this.Y
}

func (this *Vector2) Add(otherVector *Vector2) *Vector2 {
	return &Vector2{this.X + otherVector.X,
		this.Y + otherVector.Y}
}

func (this *Vector2) Sub(otherVector *Vector2) *Vector2 {
	return &Vector2{this.X - otherVector.X,
		this.Y - otherVector.Y}
}

func (this *Vector2) Scale(scale float64) *Vector2 {
	return &Vector2{this.X * scale,
		this.Y * scale}
}

func (this *Vector2) Equals(otherVector *Vector2) bool {
	return this.X == otherVector.X && this.Y == otherVector.Y
}

func (this *Vector2) Multiply(otherVector *Vector2) *Vector2 {
	return &Vector2{this.X * otherVector.X,
		this.Y * otherVector.Y}
}

func (this *Vector2) Length() float64 {
	return Sqrt(this.X*this.X + this.Y*this.Y)
}

func (this *Vector2) LengthSquared() float64 {
	return (this.X*this.X + this.Y*this.Y)
}

func (this *Vector2) Normalize() {
	len := this.Length()

	if len == 0 {
		return
	}

	num := 1.0 / len

	this.X *= num
	this.Y *= num
}

func (this *Vector2) Clone() *Vector2 {
	return &Vector2{this.X,
		this.Y}
}

func (this *Vector2) CopyFrom(source *Vector2) {
	this.X = source.X
	this.Y = source.Y
}

func (this *Vector2) Lerp(end *Vector2, amount float64) *Vector2 {
	x := this.X + ((end.X - this.X) * amount)
	y := this.Y + ((end.Y - this.Y) * amount)

	return NewVector2(x, y)
}

// Receiver is returned for easy chaning.
func (this *Vector2) Invert() *Vector2 {
	this.X = -this.X
	this.Y = -this.Y
	return this
}

// Inverted returns a new *T which is receiver inverted.
func (this *Vector2) Inverted() *Vector2 {
	p := *this
	return p.Invert()
}
