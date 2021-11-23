package math32

import (
	"fmt"
	"math"
)

// FLT_EPSILON is the C++ FLT_EPSILON epsilon constant
const FLT_EPSILON float64 = 1.19209290e-07

type RotatedRect struct {
	Center *Vector2
	Size   *Size
	Angle  float64
}

func NewRotatedRect() (rcvr *RotatedRect) {
	rcvr = &RotatedRect{}
	rcvr.Center = NewVector2(0, 0)
	rcvr.Size = NewSize(0, 0)
	rcvr.Angle = 0
	return
}
func NewRotatedRect2(c *Vector2, s *Size, a float64) (rcvr *RotatedRect) {
	rcvr = &RotatedRect{}
	rcvr.Center = c.Clone()
	rcvr.Size = s.Clone()
	rcvr.Angle = a
	return
}
func NewRotatedRect3(v1 *Vector2, v2 *Vector2, v3 *Vector2) (rcvr *RotatedRect) {
	rcvr = NewRotatedRect()
	_center := v1.Add(v3).Scale(0.5)
	vecs := make([]*Vector2, 2)
	vecs[0] = v1.Sub(v2)
	vecs[1] = v2.Sub(v3)
	x := math.Max(v1.Length(), math.Max(v2.Length(), v3.Length()))
	a := math.Min(vecs[0].Length(), vecs[1].Length())
	if !(math.Abs(vecs[0].Ddot(vecs[1]))*a <= FLT_EPSILON*9*x*(vecs[0].Length())*vecs[1].Length()) {
		panic("NewRotatedRect3 Failed")
	}

	// wd_i stores which vector (0,1) or (1,2) will make the width
	// One of them will definitely have slope within -1 to 1
	wd_i := 0
	if math.Abs(vecs[1].Y) < math.Abs(vecs[1].X) {
		wd_i = 1
	}
	ht_i := (wd_i + 1) % 2

	_angle := math.Atan(vecs[wd_i].Y/vecs[wd_i].X) * 180.0 / math.Pi
	_width := vecs[wd_i].Length()
	_height := vecs[ht_i].Length()

	rcvr.Angle = _angle
	rcvr.Size = NewSize(_width, _height)
	rcvr.Center = _center

	return rcvr
}
func (rcvr *RotatedRect) Contains(v *Vector2) bool {
	hw := rcvr.Size.Width / 2.0
	hh := rcvr.Size.Height / 2.0
	O := rcvr.Angle
	center := rcvr.Center
	X := v.X
	Y := v.Y
	r := -O * (math.Pi / 180.0)
	nTempX := center.X + (X-center.X)*math.Cos(r) - (Y-center.Y)*math.Sin(r)
	nTempY := center.Y + (X-center.X)*math.Sin(r) + (Y-center.Y)*math.Cos(r)
	if nTempX > center.X-hw && nTempX < center.X+hw && nTempY > center.Y-hh && nTempY < center.Y+hh {
		return true
	}
	return false
}
func (rcvr *RotatedRect) BoundingRect() *Rect {

	pt := rcvr.Points()

	r := NewRect(int(math.Floor(math.Min(math.Min(math.Min(pt[0].X, pt[1].X), pt[2].X), pt[3].X))),
		int(math.Floor(math.Min(math.Min(math.Min(pt[0].Y, pt[1].Y), pt[2].Y), pt[3].Y))),
		int(math.Ceil(math.Max(math.Max(math.Max(pt[0].X, pt[1].X), pt[2].X), pt[3].X))),
		int(math.Ceil(math.Max(math.Max(math.Max(pt[0].Y, pt[1].Y), pt[2].Y), pt[3].Y))))

	r.Width -= r.X - 1
	r.Height -= r.Y - 1
	return r
}
func (rcvr *RotatedRect) Clone() *RotatedRect {
	return NewRotatedRect2(rcvr.Center, rcvr.Size, rcvr.Angle)
}
func (rcvr *RotatedRect) Equals(obj interface{}) bool {
	if rcvr == obj {
		return true
	}
	it, ok := obj.(*RotatedRect)
	if !ok {
		return false
	}
	return rcvr.Center.Equals(it.Center) && rcvr.Size.Equals(it.Size) && rcvr.Angle == it.Angle
}
func (rcvr *RotatedRect) Points() []*Vector2 {
	_angle := rcvr.Angle * math.Pi / 180.0
	b := math.Cos(_angle) * 0.5
	a := math.Sin(_angle) * 0.5

	pt := make([]*Vector2, 4)
	pt[0] = NewVector2(rcvr.Center.X-a*rcvr.Size.Height-b*rcvr.Size.Width, rcvr.Center.Y+b*rcvr.Size.Height-a*rcvr.Size.Width)
	pt[1] = NewVector2(rcvr.Center.X+a*rcvr.Size.Height-b*rcvr.Size.Width, rcvr.Center.Y-b*rcvr.Size.Height-a*rcvr.Size.Width)
	pt[2] = NewVector2(2*rcvr.Center.X-pt[0].X, 2*rcvr.Center.Y-pt[0].Y)
	pt[3] = NewVector2(2*rcvr.Center.X-pt[0].X, 2*rcvr.Center.Y-pt[0].Y)
	return pt
}

func (rcvr *RotatedRect) IsCollision(other *RotatedRect) bool {
	rect1 := rcvr.Points()
	rect2 := other.Points()

	p1 := rcvr.Center
	p2 := other.Center

	//vector p1p2
	vp1p2 := p2.Sub(p1)

	//rect1 ab bc
	AB := rect1[1].Sub(rect1[0])
	BC := rect1[2].Sub(rect1[1])

	//rect2 ab bc
	A1B1 := rect2[1].Sub(rect2[0])
	B1C1 := rect2[2].Sub(rect2[1])

	//
	deg11 := rcvr.Angle / 180.0 * math.Pi
	deg12 := (90 - rcvr.Angle) / 180.0 * math.Pi

	//
	deg21 := other.Angle / 180.0 * math.Pi
	deg22 := (90 - other.Angle) / 180.0 * math.Pi

	return IsCover(vp1p2, rcvr.Size.Width/2, deg11, A1B1, B1C1) &&
		IsCover(vp1p2, rcvr.Size.Height/2, deg12, A1B1, B1C1) &&
		IsCover(vp1p2, other.Size.Width/2, deg21, AB, BC) &&
		IsCover(vp1p2, other.Size.Height/2, deg22, AB, BC)

}

func (rcvr *RotatedRect) String() string {
	if rcvr == nil {
		return ""
	}
	return fmt.Sprintf("{center:%s - size:%s - angle:%f}", rcvr.Center.String(), rcvr.Size.String(), rcvr.Angle)
}
