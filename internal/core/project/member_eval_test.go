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
	"reflect"
	"regexp"
	"testing"
)

type memberEvalFixture struct {
	Score int
}

func (f *memberEvalFixture) Value() float64 { return 1.234 }

// EmbeddedBase is embedded (anonymously) inside the fixtures below.
// It must be exported so the anonymous field itself is exported, matching
// the real-world pattern (e.g. spx.SpriteImpl, *Game).
type EmbeddedBase struct {
	Level int
}

// unexportedEmbedBase is an unexported anonymous field type — its fields must
// NOT be reachable via findPromotedFieldPtr to prevent internal state leaks.
type unexportedEmbedBase struct {
	Secret int
}

// embeddedFixture embeds EmbeddedBase by value.
type embeddedFixture struct {
	EmbeddedBase
	Name string
}

// embeddedPtrFixture embeds *EmbeddedBase as a pointer so we can test the
// nil-pointer edge case.
type embeddedPtrFixture struct {
	*EmbeddedBase
	Name string
}

// unexportedEmbedFixture embeds an unexported anonymous type — its inner
// fields must not be reachable.
type unexportedEmbedFixture struct {
	unexportedEmbedBase // unexported anonymous field
	Name                string
}

func TestResolveMemberStringEvalField(t *testing.T) {
	fixture := memberEvalFixture{Score: 7}
	eval := ResolveMemberStringEval(reflect.ValueOf(&fixture).Elem(), "Score", 0)
	if eval == nil {
		t.Fatal("expected eval function for field")
	}
	if got := eval(); got != "7" {
		t.Fatalf("unexpected field value: %s", got)
	}
}

func TestResolveMemberStringEvalAliasMethod(t *testing.T) {
	fixture := memberEvalFixture{}
	eval := ResolveMemberStringEval(reflect.ValueOf(&fixture).Elem(), "value", 0)
	if eval == nil {
		t.Fatal("expected eval function for aliased method")
	}
	if got := eval(); got != "1.23" {
		t.Fatalf("unexpected alias method value: %s", got)
	}
}

func TestResolveMemberStringEvalOriginalMethodReturnsPointer(t *testing.T) {
	fixture := memberEvalFixture{}
	eval := ResolveMemberStringEval(reflect.ValueOf(&fixture).Elem(), "Value", 0)
	if eval == nil {
		t.Fatal("expected eval function for original method")
	}
	if got := eval(); !regexp.MustCompile(`^0x[0-9a-f]+$`).MatchString(got) {
		t.Fatalf("unexpected original method format: %s", got)
	}
}

// TestResolveMemberStringEvalPromotedField verifies that fields promoted from
// an embedded struct are resolved correctly.
func TestResolveMemberStringEvalPromotedField(t *testing.T) {
	fixture := embeddedFixture{
		EmbeddedBase: EmbeddedBase{Level: 42},
		Name:         "test",
	}
	eval := ResolveMemberStringEval(reflect.ValueOf(&fixture).Elem(), "Level", 0)
	if eval == nil {
		t.Fatal("expected eval function for promoted field")
	}
	if got := eval(); got != "42" {
		t.Fatalf("promoted field value = %q, want %q", got, "42")
	}
}

// TestResolveMemberStringEvalPromotedFieldFromIndex verifies that a promoted
// field is still found when `from` skips the embedded struct itself (the
// typical sprite case where from=2).
func TestResolveMemberStringEvalPromotedFieldFromIndex(t *testing.T) {
	fixture := embeddedFixture{
		EmbeddedBase: EmbeddedBase{Level: 99},
		Name:         "test",
	}
	// from=1 skips embeddedBase at index 0; Level must still be found via promotion.
	eval := ResolveMemberStringEval(reflect.ValueOf(&fixture).Elem(), "Level", 1)
	if eval == nil {
		t.Fatal("expected eval function for promoted field even when `from` skips it")
	}
	if got := eval(); got != "99" {
		t.Fatalf("promoted field value = %q, want %q", got, "99")
	}
}

// TestResolveMemberStringEvalNilEmbeddedPointer verifies that a nil embedded
// pointer is skipped gracefully without panicking.
func TestResolveMemberStringEvalNilEmbeddedPointer(t *testing.T) {
	fixture := embeddedPtrFixture{
		EmbeddedBase: nil, // nil pointer — must not panic
		Name:         "test",
	}
	// Level lives inside *embeddedBase which is nil; expect no result.
	eval := ResolveMemberStringEval(reflect.ValueOf(&fixture).Elem(), "Level", 0)
	if eval != nil {
		t.Fatal("expected nil eval for field inside nil embedded pointer")
	}
}

// TestResolveMemberStringEvalNilEmbeddedInterface verifies that a nil embedded
// interface is skipped gracefully without panicking.
func TestResolveMemberStringEvalNilEmbeddedInterface(t *testing.T) {
	type SomeInterface interface{ Foo() }
	type ifaceFixture struct {
		SomeInterface // nil interface — must not panic
		Name          string
	}
	fixture := ifaceFixture{Name: "test"}
	eval := ResolveMemberStringEval(reflect.ValueOf(&fixture).Elem(), "Level", 0)
	if eval != nil {
		t.Fatal("expected nil eval when embedded interface is nil")
	}
}

// TestResolveMemberStringEvalUnexportedEmbeddedFieldNotReachable verifies that
// fields inside an unexported anonymous (embedded) struct are NOT reachable.
// This guards against internal framework state being exposed via user-supplied
// names (e.g. a crafted project JSON val string).
func TestResolveMemberStringEvalUnexportedEmbeddedFieldNotReachable(t *testing.T) {
	fixture := unexportedEmbedFixture{
		unexportedEmbedBase: unexportedEmbedBase{Secret: 42},
		Name:                "test",
	}
	eval := ResolveMemberStringEval(reflect.ValueOf(&fixture).Elem(), "Secret", 0)
	if eval != nil {
		t.Fatal("field inside unexported embedded struct must not be reachable")
	}
}

// TestResolveMemberStringEvalPromotedFieldNotShadowed verifies that a
// same-named field at the outer level takes priority over the promoted one.
func TestResolveMemberStringEvalPromotedFieldNotShadowed(t *testing.T) {
	type outerFixture struct {
		EmbeddedBase
		Level int // shadows EmbeddedBase.Level
	}
	fixture := outerFixture{
		EmbeddedBase: EmbeddedBase{Level: 1},
		Level:        2,
	}
	eval := ResolveMemberStringEval(reflect.ValueOf(&fixture).Elem(), "Level", 0)
	if eval == nil {
		t.Fatal("expected eval function for outer field")
	}
	if got := eval(); got != "2" {
		t.Fatalf("expected outer field to shadow promoted field, got %q", got)
	}
}

func TestResolveMemberValueEvalField(t *testing.T) {
	fixture := memberEvalFixture{Score: 7}
	eval := ResolveMemberValueEval(reflect.ValueOf(&fixture).Elem(), "Score", 0)
	if eval == nil {
		t.Fatal("expected eval function for field")
	}
	if got := eval(); got != 7 {
		t.Fatalf("unexpected field value: %v", got)
	}
}

func TestResolveMemberValueEvalAliasMethod(t *testing.T) {
	fixture := memberEvalFixture{}
	eval := ResolveMemberValueEval(reflect.ValueOf(&fixture).Elem(), "value", 0)
	if eval == nil {
		t.Fatal("expected eval function for aliased method")
	}
	if got := eval(); got != 1.234 {
		t.Fatalf("unexpected alias method value: %v", got)
	}
}

func TestResolveMemberValueEvalOriginalMethod(t *testing.T) {
	fixture := memberEvalFixture{}
	eval := ResolveMemberValueEval(reflect.ValueOf(&fixture).Elem(), "Value", 0)
	if eval == nil {
		t.Fatal("expected eval function for original method")
	}
	if got := eval(); got != 1.234 {
		t.Fatalf("unexpected original method value: %v", got)
	}
}
