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
	"reflect"
	"strings"
	"unsafe"
)

func instance(gamer reflect.Value) *Game {
	fld := gamer.FieldByName("Game")
	if !fld.IsValid() {
		panic("type doesn't have field spx.Game")
	}
	return fld.Addr().Interface().(*Game)
}

func getFieldPtrOrAlloc(g *Game, v reflect.Value, i int) (name string, val any) {
	tFld := v.Type().Field(i)
	vFld := v.Field(i)
	typ := tFld.Type
	word := unsafe.Pointer(vFld.Addr().Pointer())
	ret := reflect.NewAt(typ, word).Interface()

	if vFld.Kind() == reflect.Pointer && typ.Implements(tySprite) {
		obj := reflect.New(typ.Elem())
		reflect.ValueOf(ret).Elem().Set(obj)
		ret = obj.Interface()
	}

	if vFld.Kind() == reflect.Interface && typ.Implements(tySprite) {
		if typ2, ok := g.typs[tFld.Name]; ok {
			obj := reflect.New(typ2)
			reflect.ValueOf(ret).Elem().Set(obj)
			ret = obj.Interface()
		}
	}
	return tFld.Name, ret
}

func findFieldPtr(v reflect.Value, name string, from int) any {
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	t := v.Type()
	for i, n := from, v.NumField(); i < n; i++ {
		tFld := t.Field(i)
		if tFld.Name == name {
			word := unsafe.Pointer(v.Field(i).Addr().Pointer())
			return reflect.NewAt(tFld.Type, word).Interface()
		}
	}
	return nil
}

// findFieldRefCaseInsensitive finds a field reference by name with case-insensitive matching.
func findFieldRefCaseInsensitive(v reflect.Value, name string, from int) any {
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	t := v.Type()
	nameLower := strings.ToLower(name)
	for i, n := from, v.NumField(); i < n; i++ {
		tFld := t.Field(i)
		if strings.ToLower(tFld.Name) == nameLower {
			word := unsafe.Pointer(v.Field(i).Addr().Pointer())
			return reflect.NewAt(tFld.Type, word).Interface()
		}
	}
	return nil
}

func findObjPtr(v reflect.Value, name string, from int) any {
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	t := v.Type()
	for i, n := from, v.NumField(); i < n; i++ {
		tFld := t.Field(i)
		if tFld.Name == name {
			typ := tFld.Type
			vFld := v.Field(i)
			if vFld.Kind() == reflect.Pointer {
				word := unsafe.Pointer(vFld.Pointer())
				return reflect.NewAt(typ.Elem(), word).Interface()
			}
			if vFld.Kind() == reflect.Interface {
				word := unsafe.Pointer(vFld.Addr().Pointer())
				return reflect.NewAt(tFld.Type, word).Elem().Interface()
			}
			word := unsafe.Pointer(vFld.Addr().Pointer())
			return reflect.NewAt(typ, word).Interface()
		}
	}
	return nil
}
