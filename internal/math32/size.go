package math32

import "fmt"

type Size struct {
	Width  float64
	Height float64
}

func NewSize(width float64, height float64) (rcvr *Size) {
	rcvr = &Size{}
	rcvr.Width = width
	rcvr.Height = height
	return
}
func NewSize2() (rcvr *Size) {
	rcvr = NewSize(0, 0)
	return
}
func NewSize3(p *Vector2) (rcvr *Size) {
	rcvr = &Size{}
	rcvr.Width = p.X
	rcvr.Height = p.Y
	return
}
func NewSize4(vals []float64) (rcvr *Size) {
	rcvr = &Size{}
	rcvr.Set(vals)
	return
}
func (rcvr *Size) Area() float64 {
	return rcvr.Width * rcvr.Height
}
func (rcvr *Size) Clone() *Size {
	return NewSize(rcvr.Width, rcvr.Height)
}
func (rcvr *Size) Empty() bool {
	return rcvr.Width <= 0 || rcvr.Height <= 0
}
func (rcvr *Size) Equals(obj interface{}) bool {
	if rcvr == obj {
		return true
	}
	it, ok := obj.(*Size)
	if !ok {
		return false
	}
	return rcvr.Width == it.Width && rcvr.Height == it.Height
}
func (rcvr *Size) Set(vals []float64) {
	if vals != nil {
		rcvr.Width = func() float64 {
			if len(vals) > 0 {
				return vals[0]
			} else {
				return 0
			}
		}()
		rcvr.Height = func() float64 {
			if len(vals) > 1 {
				return vals[1]
			} else {
				return 0
			}
		}()
	} else {
		rcvr.Width = 0
		rcvr.Height = 0
	}
}
func (rcvr *Size) String() string {
	return fmt.Sprint(rcvr.Width, "x", rcvr.Height)
}
