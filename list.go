/*
 * Copyright (c) 2021 The XGo Authors (xgo.dev). All rights reserved.
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
	"fmt"
	"strconv"
	"strings"

	"github.com/goplus/spx/v3/internal/engine"
)

// -----------------------------------------------------------------------------
// Positions
// -----------------------------------------------------------------------------
type Pos = int

const (
	Invalid Pos = -1
	Last    Pos = -2
	All         = -3 // Pos or StopKind
	Random  Pos = -4
)

// -----------------------------------------------------------------------------
// Values
// -----------------------------------------------------------------------------
type obj = any

func toString(v obj) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func fromObj(v obj) any {
	if o, ok := v.(Value); ok {
		return o.data
	}
	return v
}

func normalizeNumericStringSigns(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}

	sign := 1
	i := 0
	for i < len(s) {
		switch s[i] {
		case '+':
			i++
		case '-':
			sign = -sign
			i++
		default:
			goto normalizeRest
		}
	}

normalizeRest:
	if i == len(s) {
		return "", false
	}
	if i == 0 {
		return s, true
	}
	if sign < 0 {
		return "-" + s[i:], true
	}
	return s[i:], true
}

func parseFloatString(s string) (float64, bool) {
	normalized, ok := normalizeNumericStringSigns(s)
	if !ok {
		return 0, false
	}
	f, err := strconv.ParseFloat(normalized, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

func parseIntString(s string) (int, bool) {
	normalized, ok := normalizeNumericStringSigns(s)
	if !ok {
		return 0, false
	}
	if i, err := strconv.Atoi(normalized); err == nil {
		return i, true
	}

	const maxInt = int(^uint(0) >> 1)
	const minInt = -maxInt - 1

	f, err := strconv.ParseFloat(normalized, 64)
	if err != nil {
		return 0, false
	}
	if f > float64(maxInt) || f < float64(minInt) {
		return 0, false
	}
	return int(f), true
}

func toIntAny(v any) (int, bool) {
	if v == nil {
		return 0, true
	}

	const maxInt = int(^uint(0) >> 1)
	const minInt = -maxInt - 1

	switch x := v.(type) {
	case int:
		return x, true
	case int8:
		return int(x), true
	case int16:
		return int(x), true
	case int32:
		return int(x), true
	case int64:
		if x > int64(maxInt) || x < int64(minInt) {
			return 0, false
		}
		return int(x), true
	case uint:
		if x > uint(maxInt) {
			return 0, false
		}
		return int(x), true
	case uint8:
		return int(x), true
	case uint16:
		return int(x), true
	case uint32:
		if uint64(x) > uint64(maxInt) {
			return 0, false
		}
		return int(x), true
	case uint64:
		if x > uint64(maxInt) {
			return 0, false
		}
		return int(x), true
	case float32:
		if x > float32(maxInt) || x < float32(minInt) {
			return 0, false
		}
		return int(x), true
	case float64:
		if x > float64(maxInt) || x < float64(minInt) {
			return 0, false
		}
		return int(x), true
	case bool:
		if x {
			return 1, true
		}
		return 0, true
	case string:
		if i, ok := parseIntString(x); ok {
			return i, true
		}
	}
	return 0, false
}

func toFloat64Any(v any) (float64, bool) {
	if v == nil {
		return 0, true
	}

	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int8:
		return float64(x), true
	case int16:
		return float64(x), true
	case int32:
		return float64(x), true
	case int64:
		return float64(x), true
	case uint:
		return float64(x), true
	case uint8:
		return float64(x), true
	case uint16:
		return float64(x), true
	case uint32:
		return float64(x), true
	case uint64:
		return float64(x), true
	case bool:
		if x {
			return 1, true
		}
		return 0, true
	case string:
		if f, ok := parseFloatString(x); ok {
			return f, true
		}
	}
	return 0, false
}

// -----------------------------------------------------------------------------
// Value
// -----------------------------------------------------------------------------
type Value struct {
	data any
}

func (p Value) Equal(v obj) bool {
	return Equal(p.data, v)
}

func (p Value) String() string {
	return toString(p.data)
}

func (p Value) Int() int {
	i, _ := toIntAny(p.data)
	return i
}

func (p Value) Float() float64 {
	f, _ := toFloat64Any(p.data)
	return f
}

func (p *Value) Set(v obj) {
	if p == nil {
		return
	}
	p.data = fromObj(v)
}

// NewValue creates a new Value initialized with the given object.
func NewValue(o obj) Value {
	return Value{data: fromObj(o)}
}

// -----------------------------------------------------------------------------
// List
// -----------------------------------------------------------------------------
type List struct {
	data []obj
}

func (p *List) Init(data ...obj) {
	p.data = data
}

func (p *List) InitFrom(src *List) {
	data := make([]obj, len(src.data))
	copy(data, src.data)
	p.data = data
}

func getListPos(i Pos, n int) int {
	if i == Last {
		return n - 1
	}
	if i == Random {
		if n == 0 {
			return 0
		}
		return int(randomInt31n(int32(n)))
	}
	return int(i)
}

func (p *List) Len() int {
	return len(p.data)
}

func (p *List) String() string {
	sep := ""
	items := make([]string, len(p.data))
	for i, item := range p.data {
		val := toString(item)
		if len(val) != 1 {
			sep = " "
		}
		items[i] = fmt.Sprint(val)
	}
	return strings.Join(items, sep)
}

// Contains returns true if the list contains the element v.
func (p *List) Contains(v obj) bool {
	for _, item := range p.data {
		if Equal(item, v) {
			return true
		}
	}
	return false
}

// Append adds the element v to the end of the list.
func (p *List) Append(v obj) {
	p.data = append(p.data, fromObj(v))
}

// Set sets the element at the specified index i to v.
func (p *List) Set(i Pos, v obj) {
	n := len(p.data)
	if i < 0 {
		i = Pos(getListPos(i, n))
		if i < 0 {
			engine.Panic("Set failed: invalid index -", i)
			return
		}
	}
	if int(i) < n {
		p.data[i] = fromObj(v)
	}
}

// Insert inserts the element v at the specified index i.
func (p *List) Insert(i Pos, v obj) {
	n := len(p.data)
	if i < 0 {
		if i == Invalid {
			return
		}
		i = Pos(getListPos(i, n+1))
	}
	val := fromObj(v)
	p.data = append(p.data, val)
	if int(i) < n {
		copy(p.data[i+1:], p.data[i:])
		p.data[i] = val
	}
}

// Delete removes the element at the specified index.
func (p *List) Delete(i Pos) {
	n := len(p.data)
	if i < 0 {
		if i == All {
			p.data = p.data[:0]
			return
		}
		i = Pos(getListPos(i, n))
	}
	if i >= 0 && int(i) < n {
		p.data = append(p.data[:i], p.data[i+1:]...)
	}
}

// At returns the Value at the specified index.
func (p *List) At(i Pos) Value {
	n := len(p.data)
	if i < 0 {
		i = Pos(getListPos(i, n))
	}
	if i < 0 || int(i) >= n {
		return Value{}
	}
	return Value{p.data[i]}
}

// IndexOf returns the zero-based position of the first occurrence of v in the list.
// Returns Invalid (-1) if v is not found.
func (p *List) IndexOf(v obj) Pos {
	for i, item := range p.data {
		if Equal(item, v) {
			return Pos(i)
		}
	}
	return Invalid
}

// Clear removes all elements from the list.
func (p *List) Clear() {
	p.data = p.data[:0]
}

// NewList creates a new List initialized with the given elements.
func NewList(l ...obj) List {
	data := make([]obj, len(l))
	for i, v := range l {
		data[i] = fromObj(v)
	}
	return List{data: data}
}
