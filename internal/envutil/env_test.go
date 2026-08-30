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

package envutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestLookupReportsDuplicatesWithoutLosingValue(t *testing.T) {
	value, found, duplicate := Lookup([]string{"A=one", "B=two", "A=three"}, "A")
	if value != "" || !found || !duplicate {
		t.Fatalf("Lookup duplicate = %q, %t, %t", value, found, duplicate)
	}
	if value, found, duplicate := Lookup([]string{"A=one"}, "A"); value != "one" || !found || duplicate {
		t.Fatalf("Lookup single = %q, %t, %t", value, found, duplicate)
	}
}

func TestHasNonEmptyChecksEveryOccurrence(t *testing.T) {
	if !HasNonEmpty([]string{"A=", "A=value"}, "A") {
		t.Fatal("HasNonEmpty missed a later non-empty value")
	}
	if HasNonEmpty([]string{"A=", "B=value"}, "A") {
		t.Fatal("HasNonEmpty accepted only empty values")
	}
}

func TestSetManyReplacesKeysAndPreservesOrder(t *testing.T) {
	got := SetMany([]string{"A=old", "malformed", "B=keep", "A=duplicate"}, Assignment{Key: "A", Value: "new"}, Assignment{Key: "C", Value: ""})
	want := []string{"malformed", "B=keep", "A=new", "C="}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SetMany = %#v, want %#v", got, want)
	}
}

func TestSetManyUsesLastDuplicateAssignment(t *testing.T) {
	got := SetMany([]string{"A=old", "B=keep"}, Assignment{Key: "A", Value: "first"}, Assignment{Key: "C", Value: "value"}, Assignment{Key: "A", Value: "last"})
	want := []string{"B=keep", "C=value", "A=last"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SetMany duplicates = %#v, want %#v", got, want)
	}
}

func TestWithoutPrefixes(t *testing.T) {
	got := WithoutPrefixes([]string{"CGO_ENABLED=1", "CGO_CFLAGS=x", "PATH=/bin"}, "CGO_")
	want := []string{"PATH=/bin"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("WithoutPrefixes = %#v, want %#v", got, want)
	}
}

func TestEnvironmentKeyComparisonMatchesPlatform(t *testing.T) {
	value, found, duplicate := Lookup([]string{"spx_flag=on"}, "SPX_FLAG")
	wantValue, wantFound := "", false
	if runtime.GOOS == "windows" {
		wantValue, wantFound = "on", true
	}
	if value != wantValue || found != wantFound || duplicate {
		t.Fatalf("Lookup case variant = %q, %t, %t; want %q, %t, false", value, found, duplicate, wantValue, wantFound)
	}

	got := SetMany([]string{"Path=/old"}, Assignment{Key: "PATH", Value: "/new"})
	want := []string{"Path=/old", "PATH=/new"}
	if runtime.GOOS == "windows" {
		want = []string{"PATH=/new"}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("SetMany case variants = %#v, want %#v", got, want)
	}

	got = WithoutPrefixes([]string{"cgo_cflags=-unsafe", "PATH=/bin"}, "CGO_")
	want = []string{"cgo_cflags=-unsafe", "PATH=/bin"}
	if runtime.GOOS == "windows" {
		want = []string{"PATH=/bin"}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("WithoutPrefixes case variants = %#v, want %#v", got, want)
	}
}

func TestHostGoEnvironmentPinsGraphAndTarget(t *testing.T) {
	got := HostGoEnvironment([]string{"PATH=/bin", "GOFLAGS=-mod=vendor", "GOWORK=/ambient", "GOOS=plan9", "GOARCH=386", "CGO_ENABLED=1", "SECRET=remove"}, "/graph/go.work", false, "SECRET")
	want := []string{"PATH=/bin", "GOFLAGS=" + NeutralGOFLAGS, "GOWORK=/graph/go.work", "GOOS=" + runtime.GOOS, "GOARCH=" + runtime.GOARCH, "CGO_ENABLED=0"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("HostGoEnvironment = %#v, want %#v", got, want)
	}
}

func TestHostGoEnvironmentOverridesPersistedGOFLAGS(t *testing.T) {
	goCommand, err := exec.LookPath("go")
	if err != nil {
		t.Skip(err)
	}
	goEnv := filepath.Join(t.TempDir(), "go.env")
	if err := os.WriteFile(goEnv, []byte("GOFLAGS=-buildvcs=false\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	base := SetMany(os.Environ(), Assignment{Key: "GOENV", Value: goEnv})
	command := exec.Command(goCommand, "env", "GOFLAGS")
	command.Env = HostGoEnvironment(base, "off", false)
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(output)); got != NeutralGOFLAGS {
		t.Fatalf("go env GOFLAGS = %q, want %q", got, NeutralGOFLAGS)
	}
}
