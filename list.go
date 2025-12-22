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
	"math/rand"
	"reflect"
	"strconv"
	"strings"
)

type Pos = int

const (
	Invalid Pos = -1
	Last    Pos = -2
	All         = -3 // Pos or StopKind
	Random  Pos = -4
)

// -------------------------------------------------------------------------------------

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

func toFloat64Any(v any) (float64, bool) {
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
	case string:
		if f, err := strconv.ParseFloat(x, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

// -------------------------------------------------------------------------------------

type Value struct {
	data any
}

func (p Value) Equal(v obj) bool {
	return p.data == fromObj(v)
}

func (p Value) String() string {
	return toString(p.data)
}

func (p Value) Int() int {
	switch v := p.data.(type) {
	case int:
		return v
	case nil:
		return 0
	default:
		doPanic("todo: spx.Value.Int()", reflect.TypeOf(v))
		return 0
	}
}

func (p Value) Float() float64 {
	f, ok := toFloat64Any(p.data)
	if !ok {
		doPanic("spx.Value.Float() conversion failed for type:", reflect.TypeOf(p.data))
	}
	return f
}

// -------------------------------------------------------------------------------------

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
		return int(rand.Int31n(int32(n)))
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
	val := fromObj(v)
	for _, item := range p.data {
		if item == val {
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
			doPanic("Set failed: invalid index -", i)
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
	val := fromObj(v)
	for i, item := range p.data {
		if item == val {
			return Pos(i)
		}
	}
	return Invalid
}

// Clear removes all elements from the list.
func (p *List) Clear() {
	p.data = p.data[:0]
}
