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

package project

import (
	"math"
	"reflect"
	"strconv"
)

// ResolveMemberNumberSetter binds writable variables, never reporter methods.
// Static Go field types are preserved; an any variable receives a float64.
func ResolveMemberNumberSetter(target reflect.Value, name string, from int) func(float64) bool {
	field := getValueRef(target, name, from)
	if !field.IsValid() || !field.CanSet() {
		return nil
	}
	switch field.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64, reflect.String:
	case reflect.Interface:
		if field.Type().NumMethod() != 0 {
			return nil
		}
	default:
		return nil
	}
	return func(value float64) bool {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return false
		}
		switch field.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			limit := math.Ldexp(1, field.Type().Bits()-1)
			if value < -limit || value >= limit {
				return false
			}
			field.SetInt(int64(value))
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			if value < 0 || value >= math.Ldexp(1, field.Type().Bits()) {
				return false
			}
			field.SetUint(uint64(value))
		case reflect.Float32, reflect.Float64:
			if field.OverflowFloat(value) {
				return false
			}
			field.SetFloat(value)
		case reflect.String:
			field.SetString(strconv.FormatFloat(value, 'f', -1, 64))
		case reflect.Interface:
			field.Set(reflect.ValueOf(value))
		}
		return true
	}
}
