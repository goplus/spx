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

package spx

import (
	"math"
	"reflect"
	"slices"
	"testing"

	"github.com/goplus/spbase/mathf"
	coreproject "github.com/goplus/spx/v3/internal/core/project"
	"github.com/goplus/spx/v3/internal/ui"
)

func TestListMonitorItems(t *testing.T) {
	list := NewList("苹果", 1.23456789, true, nil, NewValue("[b]text[/b]\nnext"))
	for _, value := range []any{list, &list, list.data} {
		want := []string{"苹果", "1.23456789", "true", "", "[b]text[/b]\nnext"}
		if got := listMonitorItems(value); !slices.Equal(got, want) {
			t.Errorf("listMonitorItems(%T) = %q, want %q", value, got, want)
		}
	}
	for _, value := range []any{nil, (*List)(nil), []string(nil), NewList(), 42} {
		if got := listMonitorItems(value); len(got) != 0 {
			t.Errorf("listMonitorItems(%T) = %q, want empty", value, got)
		}
	}
	array := [2]int{1, 2}
	for _, value := range []any{array, &array, []int{1, 2}} {
		if got := listMonitorItems(value); !slices.Equal(got, []string{"1", "2"}) {
			t.Errorf("listMonitorItems(%T) = %q", value, got)
		}
	}
}

type ListMonitorGame struct {
	Game
	items  List
	values []string
	Monkey listMonitorSprite
}

type listMonitorSprite struct {
	SpriteImpl
	*ListMonitorGame
	items List
}

func (g *ListMonitorGame) Current() *List { return &g.items }

func TestListMonitorEvalTracksChanges(t *testing.T) {
	g := ListMonitorGame{items: NewList("first")}
	g.Monkey.ListMonitorGame = &g
	g.Monkey.items = NewList("sprite")
	root := reflect.ValueOf(&g).Elem()
	for _, name := range []string{"items", "getVar:items", "current", "Current"} {
		t.Run(name, func(t *testing.T) {
			g.items = NewList("first")
			eval := buildMonitorEval(root, "", name, ui.MonitorAppearanceList)
			if eval == nil {
				t.Fatal("list binding failed")
			}
			before := eval().Items
			g.items.Append("second")
			g.items.Set(0, "changed")
			if got := eval().Items; !slices.Equal(got, []string{"changed", "second"}) {
				t.Fatalf("after append/set: %q", got)
			}
			if !slices.Equal(before, []string{"first"}) {
				t.Fatalf("snapshot changed: %q", before)
			}
			g.items.Delete(0)
			if got := eval().Items; !slices.Equal(got, []string{"second"}) {
				t.Fatalf("after delete: %q", got)
			}
			g.items = NewList("replacement")
			if got := eval().Items; !slices.Equal(got, []string{"replacement"}) {
				t.Fatalf("after replacement: %q", got)
			}
			g.items.Clear()
			if len(eval().Items) != 0 {
				t.Fatal("clear not reflected")
			}
		})
	}
	spriteEval := buildMonitorEval(root, "Monkey", "getVar:items", ui.MonitorAppearanceList)
	if spriteEval == nil || !slices.Equal(spriteEval().Items, []string{"sprite"}) {
		t.Fatal("sprite-local list binding failed")
	}
	globalEval := buildMonitorEval(root, "Monkey", "values", ui.MonitorAppearanceList)
	g.values = []string{"shared"}
	if globalEval == nil || !slices.Equal(globalEval().Items, g.values) {
		t.Fatal("promoted global slice binding failed")
	}
	g.values = append(g.values, "grown")
	if !slices.Equal(globalEval().Items, g.values) {
		t.Fatal("slice growth not reflected")
	}
	for _, binding := range [][2]string{{"", "getVar:"}, {"", "missing"}, {"missing", "items"}} {
		if buildMonitorEval(root, binding[0], binding[1], ui.MonitorAppearanceList) != nil {
			t.Errorf("unexpected binding for %v", binding)
		}
	}
}

func TestListMonitorConfiguration(t *testing.T) {
	for _, mode := range []any{"list", float64(4)} {
		if got := parseMonitorAppearance(coreproject.StageShape{"mode": mode}); got != ui.MonitorAppearanceList {
			t.Errorf("mode %v: %v", mode, got)
		}
	}
	for _, tt := range []struct {
		width, height any
		want          mathf.Vec2
	}{
		{nil, nil, mathf.NewVec2(100, 200)},
		{0.0, 0.0, mathf.NewVec2(100, 200)},
		{180.0, 240.0, mathf.NewVec2(180, 240)},
		{1.0, 1.0, mathf.NewVec2(100, 60)},
		{math.NaN(), math.Inf(1), mathf.NewVec2(100, 200)},
	} {
		if got := parseListMonitorDimensions(coreproject.StageShape{"width": tt.width, "height": tt.height}); got != tt.want {
			t.Errorf("dimensions(%v, %v) = %v, want %v", tt.width, tt.height, got, tt.want)
		}
	}
}

func TestListMonitorVisibility(t *testing.T) {
	g := &Game{}
	global := &Monitor{target: "", val: "getVar:items"}
	local := &Monitor{target: "Monkey", val: "items"}
	g.shapeMgr.addShape(global)
	g.shapeMgr.addShape(local)
	g.ShowVar("items")
	if !global.Visible() || local.Visible() || !global.isDirty {
		t.Fatal("ShowVar did not select the global list")
	}
	g.HideVar("items")
	if global.Visible() {
		t.Fatal("HideVar did not hide the list")
	}
	if !g.setStageMonitor("Monkey", "getVar:items", true) || !local.Visible() || global.Visible() {
		t.Fatal("sprite list visibility mismatch")
	}
}
