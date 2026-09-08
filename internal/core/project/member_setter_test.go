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
	"testing"
)

func TestMemberNumberSetter(t *testing.T) {
	type fixture struct {
		number   float64
		integer  int8
		unsigned uint8
		text     string
		dynamic  any
		flag     bool
		values   []int
	}
	f := fixture{}
	root := reflect.ValueOf(&f).Elem()
	for _, name := range []string{"number", "integer", "unsigned", "text", "dynamic"} {
		set := ResolveMemberNumberSetter(root, name, 0)
		if set == nil || !set(12.5) {
			t.Fatalf("cannot set %s", name)
		}
		if set(math.NaN()) || set(math.Inf(1)) {
			t.Fatalf("accepted non-finite %s", name)
		}
	}
	if f.number != 12.5 || f.integer != 12 || f.unsigned != 12 || f.text != "12.5" || f.dynamic != 12.5 {
		t.Fatalf("unexpected values %+v", f)
	}
	if ResolveMemberNumberSetter(root, "integer", 0)(128) || ResolveMemberNumberSetter(root, "unsigned", 0)(-1) {
		t.Fatal("overflow accepted")
	}
	for _, name := range []string{"missing", "flag", "values"} {
		if ResolveMemberNumberSetter(root, name, 0) != nil {
			t.Fatalf("accepted %s", name)
		}
	}
	reporter := memberEvalFixture{}
	if ResolveMemberNumberSetter(reflect.ValueOf(&reporter).Elem(), "value", 0) != nil {
		t.Fatal("reporter is not writable")
	}
}

func TestMemberNumberSetterPromotedField(t *testing.T) {
	f := embeddedPtrFixture{EmbeddedBase: &EmbeddedBase{Level: 1}}
	set := ResolveMemberNumberSetter(reflect.ValueOf(&f).Elem(), "Level", 1)
	if set == nil || !set(42) || f.Level != 42 {
		t.Fatal("promoted field did not update")
	}
	f.EmbeddedBase = nil
	if ResolveMemberNumberSetter(reflect.ValueOf(&f).Elem(), "Level", 1) != nil {
		t.Fatal("nil embedded pointer accepted")
	}
}
