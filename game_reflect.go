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

	coreproject "github.com/goplus/spx/v2/internal/core/project"
)

func instance(gamer reflect.Value) *Game {
	fld := gamer.FieldByName("Game")
	if !fld.IsValid() {
		panic("type doesn't have field spx.Game")
	}
	return fld.Addr().Interface().(*Game)
}

func getFieldPtrOrAlloc(g *Game, v reflect.Value, i int) (name string, val any) {
	return coreproject.FieldPtrOrAlloc(v, i, coreproject.FieldAllocConfig{
		IsPointerSpriteType: func(typ reflect.Type) bool {
			return typ.Implements(tySprite)
		},
		ResolveInterfaceSpriteType: func(fieldName string) (reflect.Type, bool) {
			typ, ok := g.typs[fieldName]
			return typ, ok
		},
	})
}

func findFieldPtr(v reflect.Value, name string, from int) any {
	return coreproject.FindFieldPtr(v, name, from)
}

// findFieldRefCaseInsensitive finds a field reference by name with case-insensitive matching.
func findFieldRefCaseInsensitive(v reflect.Value, name string, from int) any {
	return coreproject.FindFieldRefCaseInsensitive(v, name, from)
}

func findObjPtr(v reflect.Value, name string, from int) any {
	return coreproject.FindObjectPtr(v, name, from)
}
