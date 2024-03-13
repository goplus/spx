/*
 * Copyright (c) 2021 The GoPlus Authors (goplus.org). All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package spx

import (
	"errors"
	"math/rand"
	"os"
	"time"

	"golang.org/x/image/colornames"
)

// -----------------------------------------------------------------------------

var (
	errInvalidColorFormat     = errors.New("invalid color format")
	errUnsupportedColorFormat = errors.New("unsupported color format")
)

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

// -----------------------------------------------------------------------------
func RGB(r, g, b uint8) Color {
	return Color{R: r, G: g, B: b, A: 0xff}
}

func RGBA(r, g, b, a uint8) Color {
	return Color{R: r, G: g, B: b, A: a}
}

// parseColor, s can be int, float64 or string
func parseColor(s interface{}) (Color, error) {
	if s == nil {
		return Color{}, errors.New("color is nil")
	}
	if c, ok := s.(int); ok {
		return Color{uint8(c >> 16), uint8((c >> 8) & 0xff), uint8(c & 0xff), 0xff}, nil
	}
	if f, ok := s.(float64); ok {
		c := int(f)
		return Color{uint8(c >> 16), uint8((c >> 8) & 0xff), uint8(c & 0xff), 0xff}, nil
	}
	ss, ok := s.(string)
	if !ok {
		return Color{}, errUnsupportedColorFormat
	}
	c, ok := colornames.Map[ss]
	if ok {
		return c, nil
	}
	return parseHexColor(ss)
}

func parseHexColor(s string) (Color, error) {
	if s == "" || s[0] != '#' {
		return Color{}, errInvalidColorFormat
	}
	var err error
	hexToByte := func(b byte) byte {
		switch {
		case b >= '0' && b <= '9':
			return b - '0'
		case b >= 'a' && b <= 'f':
			return b - 'a' + 10
		case b >= 'A' && b <= 'F':
			return b - 'A' + 10
		default:
			err = errInvalidColorFormat
			return 0
		}
	}
	c := Color{A: 0xff}
	switch len(s) {
	case 7:
		c.R = hexToByte(s[1])<<4 + hexToByte(s[2])
		c.G = hexToByte(s[3])<<4 + hexToByte(s[4])
		c.B = hexToByte(s[5])<<4 + hexToByte(s[6])
	case 4:
		c.R = hexToByte(s[1]) * 17
		c.G = hexToByte(s[2]) * 17
		c.B = hexToByte(s[3]) * 17
	default:
		err = errInvalidColorFormat
	}
	return c, err
}

// -----------------------------------------------------------------------------

func Exit__0(code int) {
	os.Exit(code)
}

func Exit__1() {
	os.Exit(0)
}

// -----------------------------------------------------------------------------
