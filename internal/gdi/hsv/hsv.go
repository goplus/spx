package hsv

import "math"

// FromRGB converts RGB values into HSV ones in which
// H = 0 - 360, S = 0 - 100 and V = 0 - 100
//
func FromRGB(r, g, b uint8) (uint16, uint8, uint8) {
	var h, s, v float64
	R := float64(r) / 255
	G := float64(g) / 255
	B := float64(b) / 255

	minVal := min3f(R, G, B)
	maxVal := max3f(R, G, B)
	delta := maxVal - minVal

	v = maxVal

	if delta == 0 {
		h = 0
		s = 0
	} else {
		d := maxVal - minVal
		s = delta / maxVal
		switch maxVal {
		case R:
			if G < B {
				h = (G-B)/d + 6
			} else {
				h = (G-B)/d + 0
			}
		case G:
			h = (B-R)/d + 2
		case B:
			h = (R-G)/d + 4
		}
		h /= 6
	}
	return uint16(h * 360), uint8(s * 100), uint8(v * 100)
}

// ToRGB converts HSV values into RGB.
//
func ToRGB(H uint16, S, V uint8) (R uint8, G uint8, B uint8) {
	h := float64(H) / 360.0
	s := float64(S) / 100.0
	v := float64(V) / 100.0

	if s == 0 {
		R = uint8(v * 255)
		G = uint8(v * 255)
		B = uint8(v * 255)
	} else {
		h *= 6.0

		if h == 6 {
			h = 0
		}

		i := math.Floor(h)

		a := v * (1 - s)
		b := v * (1 - s*(h-i))
		c := v * (1 - s*(1-(h-i)))

		var red, green, blue float64

		switch i {
		case 0:
			red = v
			green = c
			blue = a
		case 1:
			red = b
			green = v
			blue = a
		case 2:
			red = a
			green = v
			blue = c
		case 3:
			red = a
			green = b
			blue = v
		case 4:
			red = c
			green = a
			blue = b
		default:
			red = v
			green = a
			blue = b
		}
		R = uint8(red * 255)
		G = uint8(green * 255)
		B = uint8(blue * 255)
	}
	return
}

/*
// ToRGBf converts HSV values into RGB.
//
func ToRGBf(hue, saturation, value float64) (red, green, blue float64) {
	i := math.Min(5, math.Floor(hue*6))
	f := hue*6 - i
	p := value * (1 - saturation)
	q := value * (1 - f*saturation)
	t := value * (1 - (1-f)*saturation)
	switch i {
	case 0:
		red = value
		green = t
		blue = p
	case 1:
		red = q
		green = value
		blue = p
	case 2:
		red = p
		green = value
		blue = t
	case 3:
		red = p
		green = q
		blue = value
	case 4:
		red = t
		green = p
		blue = value
	case 5:
		red = value
		green = p
		blue = q
	}
	return
}

// FromRGBf converts RGB values into HSV.
//
func FromRGBf(red, green, blue float64) (hue, saturation, value float64) {
	max := max3f(red, green, blue)
	min := min3f(red, green, blue)
	delta := max - min
	if max != 0 {
		saturation = delta / max
	}
	value = max
	if delta == 0 {
		hue = 0
	} else {
		switch max {
		case red:
			hue = (green - blue) / delta / 6
			if green < blue {
				hue++
			}
		case green:
			hue = (blue-red)/delta/6 + 1.0/3
		case blue:
			hue = (red-green)/delta/6 + 2.0/3
		}
	}
	return
}
*/

func min3f(a, b, c float64) float64 {
	if b < a {
		a = b
	}
	if c < a {
		a = c
	}
	return a
}

func max3f(a, b, c float64) float64 {
	if b > a {
		a = b
	}
	if c > a {
		a = c
	}
	return a
}
