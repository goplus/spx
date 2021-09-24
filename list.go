/*
 Copyright 2021 The GoPlus Authors (goplus.org)

 Licensed under the Apache License, Version 2.0 (the "License");
 you may not use this file except in compliance with the License.
 You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

 Unless required by applicable law or agreed to in writing, software
 distributed under the License is distributed on an "AS IS" BASIS,
 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 See the License for the specific language governing permissions and
 limitations under the License.
*/

package spx

import (
	"fmt"
	"log"
	"math/rand"
	"strings"
)

type specialObj = int

const (
	Invalid = specialObj(-1)
	Last    = specialObj(-2)
	All     = specialObj(-3)
	Random  = specialObj(-4)

	Mouse = specialObj(-5)
	Edge  = specialObj(-6)
)

// -------------------------------------------------------------------------------------

type obj = interface{}

func toString(v obj) string {
	if v == nil {
		return ""
	}
	return fmt.Sprint(v)
}

func fromObj(v obj) interface{} {
	if o, ok := v.(Value); ok {
		return o.data
	}
	return v
}

// -------------------------------------------------------------------------------------

type Value struct {
	data interface{}
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
	default:
		panic("todo: spx.Value.Int()")
	}
}

func (p Value) Float() float64 {
	panic("todo: spx.Value.Float()")
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
	for i, item := range src.data {
		data[i] = item
	}
	p.data = data
}

func getListPos(i, n int) int {
	if i == Last {
		return n - 1
	}
	if i == Random {
		if n == 0 {
			return 0
		}
		return int(rand.Int31n(int32(n)))
	}
	return i
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

func (p *List) Set(i int, v obj) {
	n := len(p.data)
	if i < 0 {
		i = getListPos(i, n)
		if i < 0 {
			log.Fatal("Set failed: invalid index -", i)
			return
		}
	}
	if i < n {
		p.data[i] = fromObj(v)
	}
}

func (p *List) Insert(i int, v obj) {
	n := len(p.data)
	if i < 0 {
		if i == Invalid {
			return
		}
		i = getListPos(i, n+1)
	}
	val := fromObj(v)
	p.data = append(p.data, val)
	if i < n {
		copy(p.data[i+1:], p.data[i:])
		p.data[i] = val
	}
}

func (p *List) Delete(i int) {
	n := len(p.data)
	if i < 0 {
		if i == All {
			p.data = p.data[:0]
			return
		}
		i = getListPos(i, n)
	}
	if i >= 0 && i < n {
		p.data = append(p.data[:i], p.data[i+1:]...)
	}
}

func (p *List) At(i int) Value {
	n := len(p.data)
	if i < 0 {
		i = getListPos(i, n)
	}
	if i < 0 || i >= n {
		return Value{}
	}
	return Value{p.data[i]}
}
