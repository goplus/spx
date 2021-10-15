package spx

import (
	"math/rand"
	"os"
	"time"
)

// -----------------------------------------------------------------------------

func init() {
	rand.Seed(time.Now().UnixNano())
}

func Rand__0(from, to int) float64 {
	if to < from {
		to = from
	}
	return float64(from + rand.Intn(to-from+1))
}

func Rand__1(from, to float64) float64 {
	if to < from {
		to = from
	}
	return rand.Float64()*(to-from) + from
}

// Iround returns an integer value, while math.Round returns a float value.
func Iround(v float64) int {
	if v >= 0 {
		return int(v + 0.5)
	}
	return int(v - 0.5)
}

func RGB(r, g, b uint8) Color {
	return Color{R: r, G: g, B: b, A: 0xff}
}

func RGBA(r, g, b, a uint8) Color {
	return Color{R: r, G: g, B: b, A: a}
}

func Exit(code ...int) {
	v := 0
	if code != nil {
		v = code[0]
	}
	os.Exit(v)
}

// -----------------------------------------------------------------------------
