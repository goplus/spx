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

package scratch

import (
	"math/rand"
	"strings"
)

// Special list positions retain their SPX values and types.
const (
	ListInvalid int = -1
	ListLast    int = -2
	ListAll         = -3
	ListRandom  int = -4
)

var listRandomInt31n = rand.Int31n

// SetListRandomSource binds list indexing to the runtime's random source.
// Call it only during initialization, before any lists are used.
func SetListRandomSource(source func(int32) int32) {
	listRandomInt31n = source
}

// List stores values using 0-based indexes.
type List struct {
	data []any
}

// NewList copies items into a new list, unwrapping one layer of Value.
func NewList(items ...any) List {
	data := make([]any, len(items))
	for i, v := range items {
		data[i] = UnwrapValue(v)
	}
	return List{data: data}
}

func (p *List) Init(data ...any) {
	p.data = data
}

func (p *List) InitFrom(src *List) {
	data := make([]any, len(src.data))
	copy(data, src.data)
	p.data = data
}

func (p *List) Len() int {
	return len(p.data)
}

// ListItems returns a string snapshot without retaining the list's storage.
func ListItems(list List) []string {
	items := make([]string, len(list.data))
	for i, item := range list.data {
		items[i] = ToString(item)
	}
	return items
}

func (p *List) String() string {
	items := ListItems(*p)
	sep := ""
	for _, item := range items {
		if len(item) != 1 {
			sep = " "
			break
		}
	}
	return strings.Join(items, sep)
}

// Contains returns true if the list contains the element v.
func (p *List) Contains(v any) bool {
	return p.IndexOf(v) != ListInvalid
}

// ContainsAt is equivalent to p.Contains(source.At(i)), but avoids creating
// and passing an intermediate Value through the interpreter.
func (p *List) ContainsAt(source *List, i int) bool {
	// Keep the Value wrapper that At would return. Init may intentionally retain
	// a nested Value, and the unfused expression unwraps only the outer layer.
	return p.Contains(WrapValue(source.rawAt(i)))
}

// Append adds the element v to the end of the list.
func (p *List) Append(v any) {
	p.data = append(p.data, UnwrapValue(v))
}

// Set sets the element at the specified index i to v.
func (p *List) Set(i int, v any) {
	n := len(p.data)
	i = getListPos(i, n)
	if i >= 0 && i < n {
		p.data[i] = UnwrapValue(v)
	}
}

// SetAt is equivalent to p.Set(i, source.At(sourceIndex)). Read the source
// before writing the destination so aliased lists keep normal Go evaluation
// order.
func (p *List) SetAt(i int, source *List, sourceIndex int) {
	value := source.rawAt(sourceIndex)
	n := len(p.data)
	i = getListPos(i, n)
	if i >= 0 && i < n {
		p.data[i] = value
	}
}

// Insert inserts the element v at the specified index i.
func (p *List) Insert(i int, v any) {
	n := len(p.data)
	if i == ListInvalid {
		return
	}
	i = getListPos(i, n+1)
	val := UnwrapValue(v)
	p.data = append(p.data, val)
	if i < n {
		copy(p.data[i+1:], p.data[i:])
		p.data[i] = val
	}
}

// Delete removes the element at the specified index.
func (p *List) Delete(i int) {
	n := len(p.data)
	if i == ListAll {
		p.Clear()
		return
	}
	i = getListPos(i, n)
	if i >= 0 && i < n {
		p.data = append(p.data[:i], p.data[i+1:]...)
	}
}

// At returns the Value at the specified index.
func (p *List) At(i int) Value {
	return WrapValue(p.rawAt(i))
}

// rawAt applies the normal list-position rules without wrapping the result.
// Fused calls use it to avoid immediately creating and unpacking a Value.
func (p *List) rawAt(i int) any {
	n := len(p.data)
	i = getListPos(i, n)
	if i < 0 || i >= n {
		return nil
	}
	return p.data[i]
}

// IntAt is equivalent to p.At(i).Int().
func (p *List) IntAt(i int) int {
	value, _ := toInt(p.rawAt(i))
	return value
}

// FloatAt is equivalent to p.At(i).Float().
func (p *List) FloatAt(i int) float64 {
	value, _ := toFloat64(p.rawAt(i))
	return value
}

// StringAt is equivalent to p.At(i).String().
func (p *List) StringAt(i int) string {
	return ToString(p.rawAt(i))
}

// EqualAt is equivalent to Equal(p.At(i), v).
func (p *List) EqualAt(i int, v any) bool {
	// Preserve Equal(At(i), v)'s single-layer unwrapping for nested Values.
	return Equal(WrapValue(p.rawAt(i)), v)
}

// IndexOf returns the zero-based position of the first occurrence of v in the list.
// Returns ListInvalid (-1) if v is not found.
func (p *List) IndexOf(v any) int {
	needle := normalizeEqualValue(v)
	for i, item := range p.data {
		if equalValues(normalizeEqualValue(item), needle) {
			return i
		}
	}
	return ListInvalid
}

// Clear removes all elements from the list.
func (p *List) Clear() {
	p.data = p.data[:0]
}

func getListPos(i int, n int) int {
	if i == ListLast {
		return n - 1
	}
	if i == ListRandom {
		if n == 0 {
			return 0
		}
		return int(listRandomInt31n(int32(n)))
	}
	return i
}
