package math32

import "fmt"

type Matrix3 [9]float64

func (pself *Matrix3) Get(row int, col int) float64 {
	return pself[row*3+col]
}
func (pself *Matrix3) Set(row int, col int, value float64) {
	pself[row*3+col] = value
}

func (pself *Matrix3) Add__0(otherMat *Matrix3) *Matrix3 {
	for i := 0; i < 9; i++ {
		pself[i] += otherMat[i]
	}
	return pself
}

func (pself *Matrix3) Sub__0(otherMat *Matrix3) *Matrix3 {
	for i := 0; i < 9; i++ {
		pself[i] -= otherMat[i]
	}
	return pself
}

func (pself *Matrix3) Scale__0(otherMat *Matrix3) *Matrix3 {
	for i := 0; i < 9; i++ {
		pself[i] *= otherMat[i]
	}
	return pself
}

func (pself *Matrix3) Div__0(otherMat *Matrix3) *Matrix3 {
	for i := 0; i < 9; i++ {
		pself[i] /= otherMat[i]
	}
	return pself
}
func (pself *Matrix3) Add__1(value float64) *Matrix3 {
	for i := 0; i < 9; i++ {
		pself[i] += value
	}
	return pself
}

func (pself *Matrix3) Sub__1(value float64) *Matrix3 {
	for i := 0; i < 9; i++ {
		pself[i] -= value
	}
	return pself
}
func (pself *Matrix3) Scale__1(value float64) *Matrix3 {
	for i := 0; i < 9; i++ {
		pself[i] *= value
	}
	return pself
}

func (pself *Matrix3) Div__1(value float64) *Matrix3 {
	for i := 0; i < 9; i++ {
		pself[i] /= value
	}
	return pself
}

func (pself *Matrix3) Mul__0(value Vector2) Vector2 {
	return Vector2{
		X: pself[0]*value.X + pself[1]*value.Y + pself[2],
		Y: pself[3]*value.X + pself[4]*value.Y + pself[5],
	}
}
func (pself *Matrix3) Mul__1(value Matrix3) *Matrix3 {
	tmp := NewMatrix3Zero()
	tmp[0] = pself[0]*value[0] + pself[1]*value[3] + pself[2]*value[6]
	tmp[1] = pself[0]*value[1] + pself[1]*value[4] + pself[2]*value[7]
	tmp[2] = pself[0]*value[2] + pself[1]*value[5] + pself[2]*value[8]
	tmp[3] = pself[3]*value[0] + pself[4]*value[3] + pself[5]*value[6]
	tmp[4] = pself[3]*value[1] + pself[4]*value[4] + pself[5]*value[7]
	tmp[5] = pself[3]*value[2] + pself[4]*value[5] + pself[5]*value[8]
	tmp[6] = pself[6]*value[0] + pself[7]*value[3] + pself[8]*value[6]
	tmp[7] = pself[6]*value[1] + pself[7]*value[4] + pself[8]*value[7]
	tmp[8] = pself[6]*value[2] + pself[7]*value[5] + pself[8]*value[8]
	return tmp
}

func (pself *Matrix3) Clone() *Matrix3 {
	tmp := NewMatrix3Zero()
	for i := 0; i < 9; i++ {
		tmp[i] = pself[i]
	}
	return tmp
}

func (pself *Matrix3) Equals(otherMat *Matrix3) bool {
	for i := 0; i < 9; i++ {
		if pself[i] != otherMat[i] {
			return false
		}
	}
	return true
}

func (pself *Matrix3) String() string {
	return fmt.Sprintf(
		"[[%f, %f, %f], [%f, %f, %f], [%f, %f, %f]]",
		pself[0], pself[1], pself[2], pself[3], pself[4], pself[5], pself[6], pself[7], pself[8],
	)
}

func NewMatrix3Indentity() *Matrix3 {
	return &Matrix3{1, 0, 0, 0, 1, 0, 0, 0, 1}
}

func NewMatrix3Zero() *Matrix3 {
	return &Matrix3{0, 0, 0, 0, 0, 0, 0, 0, 0}
}
