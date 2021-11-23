package math32

import "fmt"

type Rect struct {
	X      int
	Y      int
	Width  int
	Height int
}

func NewRect(x int, y int, width int, height int) (rcvr *Rect) {
	rcvr = &Rect{}
	rcvr.X = x
	rcvr.Y = y
	rcvr.Width = width
	rcvr.Height = height
	return
}
func NewRect2() (rcvr *Rect) {
	rcvr = NewRect(0, 0, 0, 0)
	return
}
func NewRect3(p1 *Vector2, p2 *Vector2) (rcvr *Rect) {
	rcvr = &Rect{}
	rcvr.X = func() int {
		if p1.X < p2.X {
			return int(p1.X)
		} else {
			return int(p2.X)
		}
	}()
	rcvr.Y = func() int {
		if p1.Y < p2.Y {
			return int(p1.Y)
		} else {
			return int(p2.Y)
		}
	}()
	rcvr.Width = func() int {
		if p1.X > p2.X {
			return int(p1.X)
		} else {
			return int(p2.X)
		}
	}() - rcvr.X
	rcvr.Height = func() int {
		if p1.Y > p2.Y {
			return int(p1.Y)
		} else {
			return int(p2.Y)
		}
	}() - rcvr.Y
	return
}
func NewRect4(p *Vector2, s *Size) (rcvr *Rect) {
	rcvr = NewRect(int(p.X), int(p.Y), int(s.Width), int(s.Height))
	return
}
func NewRect5(vals []float64) (rcvr *Rect) {
	rcvr = &Rect{}
	rcvr.Set(vals)
	return
}
func (rcvr *Rect) Area() float64 {
	return float64(rcvr.Width * rcvr.Height)
}
func (rcvr *Rect) Br() *Vector2 {
	return NewVector2(float64(rcvr.X+rcvr.Width), float64(rcvr.Y+rcvr.Height))
}
func (rcvr *Rect) Clone() *Rect {
	return NewRect(rcvr.X, rcvr.Y, rcvr.Width, rcvr.Height)
}
func (rcvr *Rect) Contains(p *Vector2) bool {
	return float64(rcvr.X) <= p.X && p.X < float64(rcvr.X+rcvr.Width) && float64(rcvr.Y) <= p.Y && p.Y < float64(rcvr.Y+rcvr.Height)
}
func (rcvr *Rect) Empty() bool {
	return rcvr.Width <= 0 || rcvr.Height <= 0
}
func (rcvr *Rect) Equals(obj interface{}) bool {
	if rcvr == obj {
		return true
	}
	it, ok := obj.(*Rect)
	if !ok {
		return false
	}
	return rcvr.X == it.X && rcvr.Y == it.Y && rcvr.Width == it.Width && rcvr.Height == it.Height
}
func (rcvr *Rect) Set(vals []float64) {
	if vals != nil {
		rcvr.X = func() int {
			if len(vals) > 0 {
				return int(vals[0])
			} else {
				return 0
			}
		}()
		rcvr.Y = func() int {
			if len(vals) > 1 {
				return int(vals[1])
			} else {
				return 0
			}
		}()
		rcvr.Width = func() int {
			if len(vals) > 2 {
				return int(vals[2])
			} else {
				return 0
			}
		}()
		rcvr.Height = func() int {
			if len(vals) > 3 {
				return int(vals[3])
			} else {
				return 0
			}
		}()
	} else {
		rcvr.X = 0
		rcvr.Y = 0
		rcvr.Width = 0
		rcvr.Height = 0
	}
}
func (rcvr *Rect) Size() *Size {
	return NewSize(float64(rcvr.Width), float64(rcvr.Height))
}
func (rcvr *Rect) Tl() *Vector2 {
	return NewVector2(float64(rcvr.X), float64(rcvr.Y))
}
func (rcvr *Rect) String() string {
	return fmt.Sprintf("%v%v%v%v%v%v%v%v%v", "{", rcvr.X, ", ", rcvr.Y, ", ", rcvr.Width, "x", rcvr.Height, "}")
}
