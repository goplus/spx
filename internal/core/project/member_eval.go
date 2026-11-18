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
	"fmt"
	"reflect"
)

type resolvedMember struct {
	field        reflect.Value
	method       reflect.Value
	autoProperty bool
}

func ResolveMemberValueEval(target reflect.Value, name string, from int) func() any {
	member := resolveMember(target, name, from)
	if member.field.IsValid() {
		return func() any {
			return member.field.Interface()
		}
	}
	if member.method.IsValid() {
		m := member.method
		return func() any {
			return callMethodValue(m)
		}
	}

	return nil
}

func ResolveMemberStringEval(target reflect.Value, name string, from int) func() string {
	eval := ResolveMemberTextEval(target, name, from)
	if eval == nil {
		return nil
	}
	return func() string {
		text, _ := eval()
		return text
	}
}

// ResolveMemberTextEval returns the rendered text and source value of a member.
// The source value lets callers preserve type-specific formatting decisions.
func ResolveMemberTextEval(target reflect.Value, name string, from int) func() (string, any) {
	member := resolveMember(target, name, from)
	if member.field.IsValid() {
		return func() (string, any) {
			value := member.field.Interface()
			return fmt.Sprint(value), value
		}
	}
	if member.method.IsValid() {
		return makeMethodTextAccessor(member.method, member.autoProperty)
	}

	return nil
}

func resolveMember(target reflect.Value, name string, from int) resolvedMember {
	ref := getValueRef(target, name, from)
	if ref.IsValid() {
		return resolvedMember{field: ref}
	}

	targetForMethod := target
	if target.Kind() != reflect.Ptr && target.CanAddr() {
		targetForMethod = target.Addr()
	}

	aliasName := aliasNameOf(name, true)

	m := targetForMethod.MethodByName(name)
	if methodHasAutoProperty(m) {
		return resolvedMember{method: m}
	}

	if !m.IsValid() && aliasName != "" && aliasName != name {
		mAlias := targetForMethod.MethodByName(aliasName)
		if methodHasAutoProperty(mAlias) {
			return resolvedMember{method: mAlias, autoProperty: true}
		}
	}

	return resolvedMember{}
}

func getValueRef(target reflect.Value, name string, from int) reflect.Value {
	if valPtr := findPromotedFieldPtr(target, name, from); valPtr != nil {
		return reflect.ValueOf(valPtr).Elem()
	}
	return reflect.Value{}
}

// aliasNameOf mimics gogen's aliasNameOf logic:
// For methods, lowercase names are mapped to uppercase (e.g., "add" -> "Add").
func aliasNameOf(name string, isMethod bool) string {
	if isMethod && name != "" {
		if c := name[0]; c >= 'a' && c <= 'z' {
			return string(rune(c)+('A'-'a')) + name[1:]
		}
	}
	return ""
}

// methodHasAutoProperty checks if a method value is a valid auto-property getter.
func methodHasAutoProperty(m reflect.Value) bool {
	if !m.IsValid() {
		return false
	}
	mType := m.Type()
	return mType.NumIn() == 0 && mType.NumOut() == 1
}

func makeMethodTextAccessor(m reflect.Value, autoProperty bool) func() (string, any) {
	return func() (string, any) {
		if !autoProperty {
			return fmt.Sprintf("%p", m.Interface()), nil
		}
		result := callMethodValue(m)
		if fVal, ok := result.(float64); ok {
			return fmt.Sprintf("%.2f", fVal), result
		}
		if f32Val, ok := result.(float32); ok {
			return fmt.Sprintf("%.2f", f32Val), result
		}
		return fmt.Sprint(result), result
	}
}

func callMethodValue(m reflect.Value) any {
	return m.Call(nil)[0].Interface()
}
