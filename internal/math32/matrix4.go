package math32

type Matrix4 struct {
	M00 float64 `json:"e00"`
	M01 float64 `json:"e01"`
	M02 float64 `json:"e02"`
	M03 float64 `json:"e03"`
	M10 float64 `json:"e10"`
	M11 float64 `json:"e11"`
	M12 float64 `json:"e12"`
	M13 float64 `json:"e13"`
	M20 float64 `json:"e20"`
	M21 float64 `json:"e21"`
	M22 float64 `json:"e22"`
	M23 float64 `json:"e23"`
	M30 float64 `json:"e30"`
	M31 float64 `json:"e31"`
	M32 float64 `json:"e32"`
	M33 float64 `json:"e33"`
}

func (pself *Matrix4) Mul__0(value Vector3) *Vector3 {
	return &Vector3{
		X: pself.M00*value.X + pself.M01*value.Y + pself.M02*value.Z + pself.M03,
		Y: pself.M10*value.X + pself.M11*value.Y + pself.M12*value.Z + pself.M13,
		Z: pself.M20*value.X + pself.M21*value.Y + pself.M22*value.Z + pself.M23,
	}
}

func (pself *Matrix4) Mul__1(value *Matrix4) *Matrix4 {
	tmp := NewMatrix4Zero()
	tmp.M00 = pself.M00*value.M00 + pself.M01*value.M10 + pself.M02*value.M20 + pself.M03*value.M30
	tmp.M01 = pself.M00*value.M01 + pself.M01*value.M11 + pself.M02*value.M21 + pself.M03*value.M31
	tmp.M02 = pself.M00*value.M02 + pself.M01*value.M12 + pself.M02*value.M22 + pself.M03*value.M32
	tmp.M03 = pself.M00*value.M03 + pself.M01*value.M13 + pself.M02*value.M23 + pself.M03*value.M33
	tmp.M10 = pself.M10*value.M00 + pself.M11*value.M10 + pself.M12*value.M20 + pself.M13*value.M30
	tmp.M11 = pself.M10*value.M01 + pself.M11*value.M11 + pself.M12*value.M21 + pself.M13*value.M31
	tmp.M12 = pself.M10*value.M02 + pself.M11*value.M12 + pself.M12*value.M22 + pself.M13*value.M32
	tmp.M13 = pself.M10*value.M03 + pself.M11*value.M13 + pself.M12*value.M23 + pself.M13*value.M33
	tmp.M20 = pself.M20*value.M00 + pself.M21*value.M10 + pself.M22*value.M20 + pself.M23*value.M30
	tmp.M21 = pself.M20*value.M01 + pself.M21*value.M11 + pself.M22*value.M21 + pself.M23*value.M31
	tmp.M22 = pself.M20*value.M02 + pself.M21*value.M12 + pself.M22*value.M22 + pself.M23*value.M32
	tmp.M23 = pself.M20*value.M03 + pself.M21*value.M13 + pself.M22*value.M23 + pself.M23*value.M33
	tmp.M30 = pself.M30*value.M00 + pself.M31*value.M10 + pself.M32*value.M20 + pself.M33*value.M30
	tmp.M31 = pself.M30*value.M01 + pself.M31*value.M11 + pself.M32*value.M21 + pself.M33*value.M31
	tmp.M32 = pself.M30*value.M02 + pself.M31*value.M12 + pself.M32*value.M22 + pself.M33*value.M32
	tmp.M33 = pself.M30*value.M03 + pself.M31*value.M13 + pself.M32*value.M23 + pself.M33*value.M33
	return tmp
}

func NewMatrix4Indentity() *Matrix4 {
	return &Matrix4{1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1}
}

func NewMatrix4Zero() *Matrix4 {
	return &Matrix4{
		0, 0, 0, 0,
		0, 0, 0, 0,
		0, 0, 0, 0,
		0, 0, 0, 0,
	}
}
