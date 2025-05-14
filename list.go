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
	"fmt"
	"log"
	"math/rand"
	"reflect"
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
		log.Panicln("todo: spx.Value.Int()", reflect.TypeOf(v))
		return 0
	}
}

func (p Value) Float() float64 {
	switch v := p.data.(type) {
	case float64:
		return v
	case nil:
		return 0
	default:
		log.Panicln("todo: spx.Value.Float()", reflect.TypeOf(v))
		return 0
	}
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

func (p *List) Contains(v obj) bool {
	val := fromObj(v)
	for _, item := range p.data {
		if item == val {
			return true
		}
	}
	return false
}

func (p *List) Append(v obj) {
	p.data = append(p.data, fromObj(v))
}

func (p *List) Set(i Pos, v obj) {
	n := len(p.data)
	if i < 0 {
		i = Pos(getListPos(i, n))
		if i < 0 {
			log.Panicln("Set failed: invalid index -", i)
			return
		}
	}
	if int(i) < n {
		p.data[i] = fromObj(v)
	}
}

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
