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

package ispx

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestFuseGeneratedListCalls(t *testing.T) {
	source := []byte(`package main

import spx "github.com/goplus/spx/v4"

type Game struct {
	particles spx.List
	elements spx.List
	swap spx.Value
	notAList customList
}

type Sprite struct {
	*Game
}

type customList struct{}

func (customList) At(int) any { return nil }
func (customList) Contains(any) bool { return false }

func (this *Game) update(index int, value any) {
	_ = spx.Equal(this.particles.At(index), 11)
	_ = this.elements.Contains(this.particles.At(index + 1))
	this.swap.Set(this.particles.At(index))
	this.particles.Set(index, this.particles.At(index + 1))
	_ = this.particles.At(index).Int()
	_ = this.particles.At(index).Float()
	_ = this.particles.At(index).String()
	_ = spx.Equal(this.particles.At(index), value)
	_ = this.notAList.Contains(this.notAList.At(index))
}

func (this *Sprite) inherited(index int) bool {
	return this.elements.Contains(this.particles.At(index))
}
`)

	optimized, err := fuseGeneratedListCalls(source)
	if err != nil {
		t.Fatal(err)
	}
	text := string(optimized)
	for _, expected := range []string{
		"this.particles.EqualAt(index, 11)",
		"this.elements.ContainsAt(&this.particles, index + 1)",
		"this.elements.ContainsAt(&this.particles, index)",
		"this.swap.SetAt(&this.particles, index)",
		"this.particles.SetAt(index, &this.particles, index + 1)",
		"this.particles.IntAt(index)",
		"this.particles.FloatAt(index)",
		"this.particles.StringAt(index)",
	} {
		if !strings.Contains(text, expected) {
			t.Errorf("optimized source is missing %q:\n%s", expected, text)
		}
	}
	for _, unchanged := range []string{
		"spx.Equal(this.particles.At(index), value)",
		"this.notAList.Contains(this.notAList.At(index))",
	} {
		if !strings.Contains(text, unchanged) {
			t.Errorf("unsafe expression %q was changed:\n%s", unchanged, text)
		}
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "main.go", optimized, parser.AllErrors); err != nil {
		t.Fatalf("optimized source is invalid Go: %v\n%s", err, text)
	}
}

func TestFuseGeneratedListCallsWithoutSPXImportIsNoOp(t *testing.T) {
	source := []byte("package main\n\nfunc main() {}\n")
	optimized, err := fuseGeneratedListCalls(source)
	if err != nil {
		t.Fatal(err)
	}
	if string(optimized) != string(source) {
		t.Fatalf("source without SPX import changed:\n%s", optimized)
	}
}
