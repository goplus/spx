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

func TestMatchRisingEdge(t *testing.T) {
	values := []bool{false, false, true, true, false, true}
	index := 0
	edge := MatchRisingEdge(func() bool {
		value := values[index]
		index++
		return value
	})

	var fired []int
	for i := range values {
		if edge(nil) {
			fired = append(fired, i)
		}
	}

	want := []int{2, 5}
	if len(fired) != len(want) || fired[0] != want[0] || fired[1] != want[1] {
		t.Fatalf("fired = %v, want %v", fired, want)
	}
	if index != len(values) {
		t.Fatalf("test evaluated %d times, want %d", index, len(values))
	}
}

func TestMatchRisingEdgeInitiallyTrue(t *testing.T) {
	edge := MatchRisingEdge(func() bool { return true })
	if !edge(nil) {
		t.Fatal("initial true condition did not fire")
	}
	if edge(nil) {
		t.Fatal("condition fired again while remaining true")
	}
}
