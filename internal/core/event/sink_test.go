package event

import "testing"

func TestNewSink(t *testing.T) {
	handler := "handler"
	sink := NewSink("owner", handler)
	if sink.Owner != "owner" || sink.Handler != handler || sink.Cond != nil {
		t.Fatalf("NewSink = %+v", sink)
	}
}

func TestSinkMatchers(t *testing.T) {
	if !MatchOwner("sprite")("sprite") || MatchOwner("sprite")("other") {
		t.Fatal("MatchOwner mismatch")
	}
	if !MatchOwnerOrNil("sprite")(nil) || !MatchOwnerOrNil("sprite")("sprite") || MatchOwnerOrNil("sprite")("other") {
		t.Fatal("MatchOwnerOrNil mismatch")
	}
	if !MatchValue("msg")("msg") || MatchValue("msg")("other") {
		t.Fatal("MatchValue mismatch")
	}
	if !MatchAnyOf([]int{1, 2, 3})(2) || MatchAnyOf([]int{1, 2, 3})(4) {
		t.Fatal("MatchAnyOf mismatch")
	}
	if !MatchApproxFloat(1.5, 0.01)(1.5001) || MatchApproxFloat(1.5, 0.01)(1.6) {
		t.Fatal("MatchApproxFloat mismatch")
	}
}
