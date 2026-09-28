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

package project

import (
	"context"
	"os"
	"os/exec"
	"reflect"
	"testing"
	"time"
)

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
