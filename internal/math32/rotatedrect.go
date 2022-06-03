package math32

import (
	"fmt"
	"math"
)

type OBB struct {
	centerPoint *Vector2
	extents     *Vector2
	axes        []*Vector2
	_width      float64
	_height     float64
	_rotation   float64
}

func NewOBB(centerPoint *Vector2, width, height, rotation float64) *OBB {
	obb := &OBB{}
	obb.centerPoint = centerPoint
	obb.extents = NewVector2(width/2.0, height/2.0)
	obb.axes = make([]*Vector2, 2)
	obb.axes[0] = NewVector2(math.Cos(rotation), math.Sin(rotation))
	obb.axes[1] = NewVector2(-1*math.Sin(rotation), math.Cos(rotation))
	obb._width = width
	obb._height = height
	obb._rotation = rotation
	return obb
}

func (p *OBB) getProjectionRadius(axis *Vector2) float64 {
	return p.extents.X*math.Abs(axis.Ddot(p.axes[0])) + p.extents.Y*math.Abs(axis.Ddot(p.axes[1]))
}

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

func NewRotatedRect1(r *Rect) (rcvr *RotatedRect) {
	rcvr = &RotatedRect{}
	rcvr.Center = NewVector2(r.Width/2.0+r.X, r.Height/2.0+r.Y)
	rcvr.Size = NewSize(r.Width, r.Height)
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

	r := NewRect((math.Floor(math.Min(math.Min(math.Min(pt[0].X, pt[1].X), pt[2].X), pt[3].X))),
		(math.Floor(math.Min(math.Min(math.Min(pt[0].Y, pt[1].Y), pt[2].Y), pt[3].Y))),
		(math.Ceil(math.Max(math.Max(math.Max(pt[0].X, pt[1].X), pt[2].X), pt[3].X))),
		(math.Ceil(math.Max(math.Max(math.Max(pt[0].Y, pt[1].Y), pt[2].Y), pt[3].Y))))

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
	center := rcvr.Center
	size := rcvr.Size
	pt := make([]*Vector2, 4)

	pt[0] = NewVector2(0, 0)
	pt[0].X = center.X - a*size.Height - b*size.Width
	pt[0].Y = center.Y + b*size.Height - a*size.Width

	pt[1] = NewVector2(0, 0)
	pt[1].X = center.X + a*size.Height - b*size.Width
	pt[1].Y = center.Y - b*size.Height - a*size.Width

	pt[2] = NewVector2(0, 0)
	pt[2].X = 2*center.X - pt[0].X
	pt[2].Y = 2*center.Y - pt[0].Y

	pt[3] = NewVector2(0, 0)
	pt[3].X = 2*center.X - pt[1].X
	pt[3].Y = 2*center.Y - pt[1].Y

	return pt
}

func (rcvr *RotatedRect) IsCollision(other *RotatedRect) bool {
	OBB1 := NewOBB(rcvr.Center, rcvr.Size.Width, rcvr.Size.Height, rcvr.Angle*math.Pi/180.0)
	OBB2 := NewOBB(other.Center, other.Size.Width, other.Size.Height, other.Angle*math.Pi/180.0)

	nv := OBB1.centerPoint.Sub(OBB2.centerPoint)
	axisA1 := OBB1.axes[0]
	if OBB1.getProjectionRadius(axisA1)+OBB2.getProjectionRadius(axisA1) <= math.Abs(nv.Ddot(axisA1)) {
		return false
	}

	axisA2 := OBB1.axes[1]
	if OBB1.getProjectionRadius(axisA2)+OBB2.getProjectionRadius(axisA2) <= math.Abs(nv.Ddot(axisA2)) {
		return false
	}

	axisB1 := OBB2.axes[0]
	if OBB1.getProjectionRadius(axisB1)+OBB2.getProjectionRadius(axisB1) <= math.Abs(nv.Ddot(axisB1)) {
		return false
	}

	axisB2 := OBB2.axes[1]
	return OBB1.getProjectionRadius(axisB2)+OBB2.getProjectionRadius(axisB2) > math.Abs(nv.Ddot(axisB2))
}

func (rcvr *RotatedRect) String() string {
	if rcvr == nil {
		return ""
	}
	plist := rcvr.Points()
	return fmt.Sprintf("{center:%s - size:%s - angle:%f pos[%s  %s  %s  %s]}", rcvr.Center.String(), rcvr.Size.String(), rcvr.Angle, plist[0].String(), plist[1].String(), plist[2].String(), plist[3].String())
}
