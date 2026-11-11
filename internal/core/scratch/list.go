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

func (p *List) Init(values ...any) {
	p.data = values
}

func (p *List) InitFrom(sourceList *List) {
	data := make([]any, len(sourceList.data))
	copy(data, sourceList.data)
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

// Contains returns true if the list contains value.
func (p *List) Contains(value any) bool {
	return p.IndexOf(value) != ListInvalid
}

// Append adds value to the end of the list.
func (p *List) Append(value any) {
	p.data = append(p.data, UnwrapValue(value))
}

// Set sets the element at index to value.
func (p *List) Set(index int, value any) {
	n := len(p.data)
	index = getListPos(index, n)
	if index >= 0 && index < n {
		p.data[index] = UnwrapValue(value)
	}
}

// Insert inserts value at index.
func (p *List) Insert(index int, value any) {
	n := len(p.data)
	if index == ListInvalid {
		return
	}
	index = getListPos(index, n+1)
	val := UnwrapValue(value)
	p.data = append(p.data, val)
	if index < n {
		copy(p.data[index+1:], p.data[index:])
		p.data[index] = val
	}
}

// Delete removes the element at the specified index.
func (p *List) Delete(index int) {
	n := len(p.data)
	if index == ListAll {
		p.Clear()
		return
	}
	index = getListPos(index, n)
	if index >= 0 && index < n {
		p.data = append(p.data[:index], p.data[index+1:]...)
	}
}

// At returns the Value at the specified index.
func (p *List) At(index int) Value {
	n := len(p.data)
	index = getListPos(index, n)
	if index < 0 || index >= n {
		return Value{}
	}
	return WrapValue(p.data[index])
}

// IndexOf returns the zero-based position of the first occurrence of v in the list.
// Returns ListInvalid (-1) if v is not found.
func (p *List) IndexOf(v any) int {
	for i, item := range p.data {
		if Equal(item, v) {
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
