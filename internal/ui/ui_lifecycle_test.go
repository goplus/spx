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

package ui

import (
	"testing"

	"github.com/goplus/spbase/mathf"
	coreproject "github.com/goplus/spx/v4/internal/core/project"
	"github.com/goplus/spx/v4/internal/coroutine"
	"github.com/goplus/spx/v4/internal/engine"
	gdx "github.com/goplus/spx/v4/pkg/spx/pkg/engine"
)

type uiLifecycleBinding struct {
	parent gdx.Object
	path   string
}

type uiLifecycleMgr struct {
	*uiRenderSpy
	next     gdx.Object
	prefabs  map[gdx.Object]string
	bindings map[gdx.Object]uiLifecycleBinding
}

type uiLifecyclePlatform struct{ gdx.IPlatformMgr }

func TestUIPrefabsBindChildrenBeforeFirstUse(t *testing.T) {
	for _, test := range []struct {
		name   string
		create func() gdx.IUiNode
	}{
		{"UiAsk", func() gdx.IUiNode { return engine.CreateEngineUIForType[UiAsk]("") }},
		{"UiDebug", func() gdx.IUiNode { return engine.CreateEngineUIForType[UiDebug]("") }},
		{"UiMeasure", func() gdx.IUiNode { return engine.CreateEngineUIForType[UiMeasure]("") }},
		{"UiQuote", func() gdx.IUiNode { return engine.CreateEngineUIForType[UiQuote]("") }},
		{"UiSay", func() gdx.IUiNode { return engine.CreateEngineUIForType[UiSay]("") }},
	} {
		t.Run(test.name, func(t *testing.T) {
			mgr := setupUILifecycle(t)
			node := test.create()
			if got, want := mgr.prefabs[node.GetId()], "res://engine/ui/"+test.name+".tscn"; got != want {
				t.Fatalf("loaded prefab = %q, want %q", got, want)
			}
			if gdx.UiNodes()[node.GetId()] != node {
				t.Fatal("constructor did not register the live UI instance")
			}
			var children map[string]*UiNode
			switch panel := node.(type) {
			case *UiAsk:
				children = map[string]*UiNode{
					"MF/Frame/AskBody": panel.askBody, "MF/Frame/AskBody/LabelAsk": panel.askLabel,
					"M/Input": panel.input, "M/Input/Check": panel.checkBtn,
				}
			case *UiDebug:
				children = map[string]*UiNode{"Label": panel.input}
			case *UiMeasure:
				children = map[string]*UiNode{"C": panel.container, "C/Line": panel.imageLine, "LC": panel.labelContainer, "LC/Label": panel.labelValue}
			case *UiQuote:
				children = map[string]*UiNode{
					"C": panel.container, "C/ImageL": panel.imageL, "C/ImageR": panel.imageR,
					"C/LabelDes": panel.labelDes, "C/LabelMsg": panel.labelMsg,
				}
			case *UiSay:
				children = map[string]*UiNode{
					"VL": panel.left.vbox, "VL/BG/Label": panel.left.label,
					"VR": panel.right.vbox, "VR/BG/Label": panel.right.label,
					"VLThink": panel.leftThink.vbox, "VLThink/BG/MC/Label": panel.leftThink.label,
					"VRThink": panel.rightThink.vbox, "VRThink/BG/MC/Label": panel.rightThink.label,
				}
				// No cloud may flash at the origin before its first layout.
				for _, variant := range []sayNodes{panel.left, panel.right, panel.leftThink, panel.rightThink} {
					visible, set := mgr.visible[variant.vbox.GetId()]
					if !set || visible {
						t.Fatal("new bubble did not explicitly hide every variant")
					}
				}
			}
			if len(mgr.bindings) != len(children) {
				t.Fatalf("bound %d nodes, want %d", len(mgr.bindings), len(children))
			}
			for path, child := range children {
				if child == nil {
					t.Fatalf("child %q was not bound", path)
				}
				if got, want := mgr.bindings[child.GetId()], (uiLifecycleBinding{node.GetId(), path}); got != want {
					t.Errorf("child binding = %+v, want %+v", got, want)
				}
				if gdx.UiNodes()[child.GetId()] != child || child.OnUiClickEvent == nil {
					t.Errorf("child %q is not fully initialized", path)
				}
			}
		})
	}
}

func TestUiMonitorFactoryInitializesFirstRender(t *testing.T) {
	mgr := setupUILifecycle(t)

	panel := NewUiMonitor()
	if mgr.prefabs[panel.GetId()] != "res://engine/ui/UiMonitor.tscn" || gdx.UiNodes()[panel.GetId()] != panel {
		t.Fatal("monitor factory did not create and register its prefab")
	}
	if panel.active != monitorAppearanceCount {
		t.Fatal("new monitor already has an active view")
	}
	if _, changed := panel.ReadSliderChange(); changed {
		t.Fatal("new monitor reported slider input before first render")
	}
	for i, spec := range monitorViewSpecs {
		view := panel.views[i]
		for _, child := range []struct {
			path string
			node *UiNode
		}{{spec.root, view.root}, {spec.label, view.label}, {spec.value, view.value}, {spec.colorTarget, view.colorTarget}, {spec.input, view.input}} {
			if child.path == "" {
				if child.node != nil {
					t.Fatal("optional monitor node unexpectedly bound")
				}
				continue
			}
			if child.node == nil || mgr.bindings[child.node.GetId()] != (uiLifecycleBinding{panel.GetId(), child.path}) {
				t.Fatalf("monitor child %q was not bound to its prefab", child.path)
			}
		}
	}

	panel.Render(MonitorStyle{Label: "score"}, MonitorValue{Text: "42"})
	for i, view := range panel.views {
		visible, set := mgr.visible[view.root.GetId()]
		if !set || visible != (MonitorAppearance(i) == MonitorAppearanceDefault) {
			t.Fatalf("first render did not set view %d visibility", i)
		}
	}
	view := panel.views[MonitorAppearanceDefault]
	if mgr.text[view.label.GetId()] != "score" || mgr.text[view.value.GetId()] != "42" {
		t.Fatal("first monitor render lost its label or value")
	}
}

func TestUIAskBoundButtonSubmitsCurrentAnswer(t *testing.T) {
	mgr := setupUILifecycle(t)
	panel := engine.CreateEngineUIForType[UiAsk]("")
	for _, answer := range []string{"first answer", "下一题"} {
		calls := 0
		panel.Show(true, "Question", func(got string) {
			calls++
			if got != answer || mgr.visible[panel.GetId()] {
				t.Errorf("submitted %q with visible=%v", got, mgr.visible[panel.GetId()])
			}
		})
		mgr.text[panel.input.GetId()] = answer
		panel.checkBtn.V_OnUiClick()
		if calls != 1 {
			t.Fatalf("button invoked callback %d times, want once", calls)
		}
	}
}

func TestMeasureShapeKeepsLabelInLogicalUnits(t *testing.T) {
	for _, test := range []struct {
		name, text             string
		size, scale, lineWidth float64
	}{
		{"integer", "10", 10, 3, 38},
		{"rounded", "1.3", 1.26, 2, 10.52},
	} {
		t.Run(test.name, func(t *testing.T) {
			mgr := setupUILifecycle(t)
			color := mathf.NewColorRGBAi(20, 40, 60, 255)
			shape := NewMeasureShape(coreproject.MeasureShape{X: 5, Y: 7, Size: test.size, Scale: test.scale, Heading: 90, Color: color}).(*measureShape)
			panel := shape.panel
			if got := mgr.text[panel.labelValue.GetId()]; got != test.text {
				t.Fatalf("measurement label = %q, want %q", got, test.text)
			}
			assertVec2Near(t, "scaled ruler", mgr.sizes[panel.container.GetId()], mathf.NewVec2(test.lineWidth, 26))
			assertVec2Near(t, "label anchor", mgr.positions[panel.labelContainer.GetId()], mathf.NewVec2(240, 146))
			if mgr.colors[panel.container.GetId()] != color || mgr.colors[panel.labelContainer.GetId()] != color {
				t.Fatal("measurement lost its configured color")
			}
		})
	}
}

func (m *uiLifecycleMgr) CreateNode(path string) gdx.Object {
	m.next++
	m.prefabs[m.next] = path
	return m.next
}

func (m *uiLifecycleMgr) BindNode(parent gdx.Object, path string) gdx.Object {
	m.next++
	m.bindings[m.next] = uiLifecycleBinding{parent, path}
	return m.next
}

func (uiLifecyclePlatform) IsMainThread() bool { return true }

func setupUILifecycle(t *testing.T) *uiLifecycleMgr {
	t.Helper()
	mgr := &uiLifecycleMgr{
		uiRenderSpy: setupUIRendering(t), next: 100000,
		prefabs: make(map[gdx.Object]string), bindings: make(map[gdx.Object]uiLifecycleBinding),
	}
	for id := range gdx.UiNodes() {
		mgr.next = max(mgr.next, id)
	}
	gdx.UiMgr = mgr
	// Public factories cross the engine-thread boundary. The UI package has
	// no running game; install an idle scheduler for them.
	previous := gdx.PlatformMgr
	engine.SetCoroutines(coroutine.New(nil))
	gdx.PlatformMgr = uiLifecyclePlatform{}
	t.Cleanup(func() {
		engine.SetCoroutines(nil)
		gdx.PlatformMgr = previous
		for id := range mgr.prefabs {
			delete(gdx.UiNodes(), id)
		}
		for id := range mgr.bindings {
			delete(gdx.UiNodes(), id)
		}
	})
	return mgr
}
