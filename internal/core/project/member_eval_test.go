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
