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
	"context"
	"math"
	"os"
	"os/exec"
	"reflect"
	"regexp"
	"testing"
	"time"
)

type memberEvalFixture struct {
	Score int
}

func (f *memberEvalFixture) Value() float64  { return 1.234 }
func (f *memberEvalFixture) Single() float32 { return 2.5 }
func (f *memberEvalFixture) Text() string    { return "level complete" }

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
	for _, tt := range []struct {
		name string
		want string
	}{
		{name: "value", want: "1.23"},
		{name: "single", want: "2.50"},
		{name: "text", want: "level complete"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			eval := ResolveMemberStringEval(reflect.ValueOf(&fixture).Elem(), tt.name, 0)
			if eval == nil {
				t.Fatal("expected eval function for aliased method")
			}
			if got := eval(); got != tt.want {
				t.Fatalf("alias method value = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestResolveMemberStringEvalOriginalMethod(t *testing.T) {
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
	if eval := ResolveMemberValueEval(reflect.ValueOf(&fixture).Elem(), "missing", 0); eval != nil {
		t.Fatal("missing field resolved to a value evaluator")
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

type CyclicMemberFixture struct {
	*CyclicMemberFixture
	*CycleMemberLeaf
	Label string
}

func TestResolveMemberEvalEmbeddedRootIgnoresTopLevelFrom(t *testing.T) {
	fixture := &CyclicMemberFixture{Label: "root"}
	fixture.CyclicMemberFixture = fixture
	if eval := ResolveMemberValueEval(reflect.ValueOf(fixture), "Label", 3); eval == nil || eval() != "root" {
		t.Fatal("from must only restrict direct fields, including when an embedded pointer revisits the root")
	}
}

func (*CyclicMemberFixture) Value() int { return 42 }

type CycleMemberLeaf struct{ Score int }
type CycleMemberBranch struct{ *CycleMemberLeaf }
type CycleMemberLeft struct{ *CycleMemberBranch }
type CycleMemberRight struct{ *CycleMemberBranch }

func TestResolveMemberEvalCyclicEmbeddings(t *testing.T) {
	if os.Getenv("SPX_TEST_MEMBER_CYCLES") != "1" {
		executable, err := os.Executable()
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, executable, "-test.run=^TestResolveMemberEvalCyclicEmbeddings$", "-test.count=1")
		cmd.Env = append(os.Environ(), "SPX_TEST_MEMBER_CYCLES=1")
		output, err := cmd.CombinedOutput()
		if ctx.Err() != nil {
			t.Fatalf("cyclic member lookup did not terminate: %v\n%s", ctx.Err(), output)
		}
		if err != nil {
			t.Fatalf("cyclic member lookup: %v\n%s", err, output)
		}
		return
	}

	self := &CyclicMemberFixture{}
	self.CyclicMemberFixture = self
	first, second := &CyclicMemberFixture{}, &CyclicMemberFixture{CycleMemberLeaf: &CycleMemberLeaf{Score: 17}}
	first.CyclicMemberFixture, second.CyclicMemberFixture = second, first
	for _, fixture := range []*CyclicMemberFixture{self, first} {
		target := reflect.ValueOf(fixture)
		if eval := ResolveMemberValueEval(target, "missing", 0); eval != nil {
			t.Fatal("unexpected value evaluator for missing member")
		}
		if eval := ResolveMemberStringEval(target, "missing", 0); eval != nil {
			t.Fatal("unexpected string evaluator for missing member")
		}
		if eval := ResolveMemberValueEval(target, "value", 0); eval == nil || eval() != 42 {
			t.Fatal("cycle hid value getter")
		}
		if eval := ResolveMemberStringEval(target, "value", 0); eval == nil || eval() != "42" {
			t.Fatal("cycle hid string getter")
		}
	}
	if eval := ResolveMemberValueEval(reflect.ValueOf(first), "Score", 0); eval == nil || eval() != 17 {
		t.Fatal("cycle hid field on distinct instance of the same embedded type")
	}
}

func TestResolveMemberEvalPreservesEmbeddedInstanceOrder(t *testing.T) {
	left, right := &CycleMemberBranch{}, &CycleMemberBranch{CycleMemberLeaf: &CycleMemberLeaf{Score: 9}}
	fixture := struct {
		CycleMemberLeft
		CycleMemberRight
	}{
		CycleMemberLeft{left}, CycleMemberRight{right},
	}
	target := reflect.ValueOf(&fixture)
	if eval := ResolveMemberValueEval(target, "Score", 0); eval == nil || eval() != 9 {
		t.Fatal("missing field from second instance of the same embedded type")
	}
	left.CycleMemberLeaf = &CycleMemberLeaf{Score: 7}
	if eval := ResolveMemberValueEval(target, "Score", 0); eval == nil || eval() != 7 {
		t.Fatal("first field at equal depth must win")
	}
	shallow := struct {
		CycleMemberLeft
		*CycleMemberLeaf
	}{
		CycleMemberLeft{right}, &CycleMemberLeaf{Score: 3},
	}
	if eval := ResolveMemberValueEval(reflect.ValueOf(&shallow), "Score", 0); eval == nil || eval() != 3 {
		t.Fatal("shallower field must win even in a later branch")
	}
}

func TestMemberNumberSetter(t *testing.T) {
	type fixture struct {
		number   float64
		single   float32
		integer  int8
		unsigned uint8
		text     string
		dynamic  any
		flag     bool
		values   []int
		reporter interface{ Value() float64 }
	}
	reporter := &memberEvalFixture{}
	f := fixture{flag: true, values: []int{1, 2}, reporter: reporter}
	root := reflect.ValueOf(&f).Elem()
	for _, name := range []string{"number", "single", "integer", "unsigned", "text", "dynamic"} {
		set := ResolveMemberNumberSetter(root, name, 0)
		if set == nil || !set(12.5) {
			t.Fatalf("cannot set %s", name)
		}
		for _, value := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
			if set(value) {
				t.Errorf("accepted %g for %s", value, name)
			}
		}
	}
	if f.number != 12.5 || f.single != 12.5 || f.integer != 12 || f.unsigned != 12 || f.text != "12.5" || f.dynamic != 12.5 {
		t.Fatalf("unexpected values %+v", f)
	}
	for _, tt := range []struct {
		name  string
		value float64
	}{
		{name: "integer", value: 128},
		{name: "integer", value: -129},
		{name: "unsigned", value: 256},
		{name: "unsigned", value: -1},
		{name: "single", value: math.MaxFloat64},
		{name: "single", value: -math.MaxFloat64},
	} {
		set := ResolveMemberNumberSetter(root, tt.name, 0)
		eval := ResolveMemberValueEval(root, tt.name, 0)
		before := eval()
		if set(tt.value) {
			t.Errorf("%s accepted out-of-range value %g", tt.name, tt.value)
		}
		if got := eval(); got != before {
			t.Errorf("rejected %s update changed value to %v, want %v", tt.name, got, before)
		}
	}
	for _, name := range []string{"missing", "flag", "values", "reporter"} {
		if ResolveMemberNumberSetter(root, name, 0) != nil {
			t.Fatalf("accepted %s", name)
		}
	}
	if ResolveMemberNumberSetter(reflect.ValueOf(42), "number", 0) != nil {
		t.Fatal("non-struct target accepted")
	}
	if f.reporter != reporter {
		t.Fatal("non-empty interface changed")
	}
	if ResolveMemberNumberSetter(reflect.ValueOf(reporter).Elem(), "value", 0) != nil {
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
