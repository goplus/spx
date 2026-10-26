/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
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
	"reflect"
	"sync"

	coreproject "github.com/goplus/spx/v4/internal/core/project"
)

const (
	spriteOwnerField          = 1
	spritePropertyFieldsStart = spriteOwnerField + 1
)

// spriteLayout records the generated struct contract once for loading, lookup,
// and cloning. Handwritten sprites may put SpriteImpl in a different direct
// field; only sprites initialized from project assets require an owner slot.
type spriteLayout struct {
	typ        reflect.Type
	baseField  int
	userFields []int
	initFields []spriteInitField
	err        error
	cloneErr   error // InitFrom signatures restrict cloning, not loading or lookup.
}

type spriteInitField struct {
	field  int
	method int
}

var (
	spriteImplType = reflect.TypeFor[SpriteImpl]()
	spriteLayouts  sync.Map // reflect.Type -> immutable *spriteLayout
)

func (layout *spriteLayout) validateOwner(gamer reflect.Type) error {
	if spriteOwnerField >= layout.typ.NumField() {
		return fmt.Errorf("sprite %s is missing owner field", layout.typ)
	}
	ownerType := layout.typ.Field(spriteOwnerField).Type
	gamerPtr := reflect.PointerTo(gamer)
	if !gamerPtr.AssignableTo(ownerType) {
		return fmt.Errorf("sprite %s owner field type %s cannot hold %s", layout.typ, ownerType, gamerPtr)
	}
	return nil
}

func (layout *spriteLayout) base(v reflect.Value) *SpriteImpl {
	return coreproject.WritableField(v.Field(layout.baseField)).Addr().Interface().(*SpriteImpl)
}

func spriteOf(sprite Sprite) *SpriteImpl {
	v := reflect.ValueOf(sprite)
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct || !v.CanAddr() {
		return nil
	}
	layout, err := inspectSpriteLayout(v.Type())
	if err != nil {
		return nil
	}
	return layout.base(v)
}

func inspectSpriteLayout(typ reflect.Type) (*spriteLayout, error) {
	if typ == nil || typ.Kind() != reflect.Struct {
		return nil, fmt.Errorf("sprite type %v must be a struct", typ)
	}
	if cached, ok := spriteLayouts.Load(typ); ok {
		layout := cached.(*spriteLayout)
		return layout, layout.err
	}
	layout := &spriteLayout{typ: typ, baseField: -1}
	for i := range typ.NumField() {
		field := typ.Field(i)
		if field.Type == spriteImplType {
			if layout.baseField == -1 {
				layout.baseField = i
			}
		} else {
			layout.userFields = append(layout.userFields, i)
		}
		ptrType := reflect.PointerTo(field.Type)
		if method, ok := ptrType.MethodByName("InitFrom"); ok {
			// Preserve the existing call contract, including methods accepting
			// interfaces or variadic arguments, and ignored return values.
			inputs, variadic := method.Type.NumIn(), method.Type.IsVariadic()
			acceptsSource := inputs == 2 || (variadic && inputs == 3)
			if acceptsSource {
				argument := method.Type.In(1)
				if variadic && inputs == 2 {
					argument = argument.Elem()
				}
				acceptsSource = ptrType.AssignableTo(argument)
			}
			if !acceptsSource {
				if layout.cloneErr == nil {
					layout.cloneErr = fmt.Errorf("sprite %s field %s InitFrom must accept one %s argument", typ, field.Name, ptrType)
				}
				continue
			}
			layout.initFields = append(layout.initFields, spriteInitField{field: i, method: method.Index})
		}
	}
	if layout.baseField < 0 {
		layout.err = fmt.Errorf("sprite %s is missing SpriteImpl field", typ)
	}
	actual, _ := spriteLayouts.LoadOrStore(typ, layout)
	layout = actual.(*spriteLayout)
	return layout, layout.err
}

func inspectCloneLayout(typ reflect.Type) (*spriteLayout, error) {
	layout, err := inspectSpriteLayout(typ)
	if err == nil {
		err = layout.cloneErr
	}
	return layout, err
}
