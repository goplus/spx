package math32

import "fmt"

type Rect struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
}

func NewRect(x float64, y float64, width float64, height float64) (rc *Rect) {
	rc = &Rect{}
	rc.X = x
	rc.Y = y
	rc.Width = width
	rc.Height = height
	return
}
func NewRect2() (rc *Rect) {
	rc = NewRect(0, 0, 0, 0)
	return
}
func NewRect3(p1 *Vector2, p2 *Vector2) (rc *Rect) {
	rc = &Rect{}
	rc.X = func() float64 {
		if p1.X < p2.X {
			return (p1.X)
		} else {
			return (p2.X)
		}
	}()
	rc.Y = func() float64 {
		if p1.Y < p2.Y {
			return (p1.Y)
		} else {
			return (p2.Y)
		}
	}()
	rc.Width = func() float64 {
		if p1.X > p2.X {
			return (p1.X)
		} else {
			return (p2.X)
		}
	}() - rc.X
	rc.Height = func() float64 {
		if p1.Y > p2.Y {
			return (p1.Y)
		} else {
			return (p2.Y)
		}
	}() - rc.Y
	return
}
func NewRect4(p *Vector2, s *Size) (rc *Rect) {
	rc = NewRect((p.X), (p.Y), (s.Width), (s.Height))
	return
}
func (rc *Rect) Area() float64 {
	return float64(rc.Width * rc.Height)
}

func (rc *Rect) Clone() *Rect {
	return NewRect(rc.X, rc.Y, rc.Width, rc.Height)
}
func (rc *Rect) Contains(p *Vector2) bool {
	return float64(rc.X) <= p.X && p.X < float64(rc.X+rc.Width) && float64(rc.Y) <= p.Y && p.Y < float64(rc.Y+rc.Height)
}
func (rc *Rect) Empty() bool {
	return rc.Width <= 0 || rc.Height <= 0
}
func (rc *Rect) Equals(obj interface{}) bool {
	if rc == obj {
		return true
	}
	it, ok := obj.(*Rect)
	if !ok {
		return false
	}
	return rc.X == it.X && rc.Y == it.Y && rc.Width == it.Width && rc.Height == it.Height
}
func (rc *Rect) Size() *Size {
	return NewSize(float64(rc.Width), float64(rc.Height))
}
func (rc *Rect) Tl() *Vector2 {
	return NewVector2(float64(rc.X), float64(rc.Y))
}
func (rc *Rect) Br() *Vector2 {
	return NewVector2(float64(rc.X+rc.Width), float64(rc.Y+rc.Height))
}

func (rc *Rect) Intersect(dst *Rect) *Rect {
	src_tl := rc.Tl()
	src_br := rc.Br()

	dst_tl := dst.Tl()
	dst_br := dst.Br()

	if src_tl.X < dst_tl.X {
		src_tl.X = dst_tl.X
	}
	if src_tl.Y < dst_tl.Y {
		src_tl.Y = dst_tl.Y
	}
	if src_br.X > dst_br.X {
		src_br.X = dst_br.X
	}
	if src_br.Y > dst_br.Y {
		src_br.Y = dst_br.Y
	}

	return NewRect3(src_tl, src_br)
}
func (rc *Rect) String() string {
	return fmt.Sprintf("%v%v%v%v%v%v%v%v%v", "{", rc.X, ", ", rc.Y, ", ", rc.Width, "x", rc.Height, "}")
}
