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

// Value stores a value with conversion and comparison helpers.
type Value struct {
	data any
}

// NewValue creates a Value, unwrapping one layer of Value from v.
func NewValue(v any) Value {
	return WrapValue(UnwrapValue(v))
}

// WrapValue stores v unchanged, including any existing Value wrapper.
func WrapValue(v any) Value {
	return Value{data: v}
}

// Raw returns the stored value without converting its type or contents.
func (p Value) Raw() any {
	return p.data
}

func (p Value) Equal(v any) bool {
	return Equal(p.data, v)
}

func (p Value) String() string {
	return ToString(p.data)
}

func (p Value) Int() int {
	i, _ := toInt(p.data)
	return i
}

func (p Value) Float() float64 {
	f, _ := toFloat64(p.data)
	return f
}

func (p *Value) Set(v any) {
	if p == nil {
		return
	}
	p.data = UnwrapValue(v)
}

// SetAt is equivalent to p.Set(source.At(i)), but keeps the list lookup and
// assignment behind one interpreter call boundary.
func (p *Value) SetAt(source *List, i int) {
	value := source.rawAt(i)
	if p == nil {
		return
	}
	p.data = value
}
