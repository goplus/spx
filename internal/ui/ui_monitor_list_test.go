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

package ui

import (
	"slices"
	"testing"

	"github.com/goplus/spbase/mathf"
)

func TestListMonitorRender(t *testing.T) {
	panel, bound := newBoundMonitor(t)
	sink := newMonitorRenderSpy()
	color := mathf.NewColorRGBAi(255, 102, 26, 255)
	size := mathf.NewVec2(100, 200)
	style := MonitorStyle{Appearance: MonitorAppearanceList, Label: "items", Color: color, Dimensions: size}
	panel.render(sink, style, MonitorValue{Items: nil})
	if sink.listCalls != 1 || sink.text[bound["ScratchList"]] != "items" || sink.color[bound["ScratchList"]] != color || sink.size[bound["ScratchList"]] != size {
		t.Fatal("initial empty list render missing")
	}
	for _, view := range panel.views {
		if sink.visible[view.root.GetId()] != (view.root.GetId() == bound["ScratchList"]) {
			t.Fatal("wrong active monitor view")
		}
	}
	items := []string{"one", "two"}
	panel.render(sink, style, MonitorValue{Items: items})
	panel.render(sink, style, MonitorValue{Items: items})
	if sink.listCalls != 2 {
		t.Fatal("unchanged list sent across the bridge")
	}
	items[0] = "changed"
	panel.render(sink, style, MonitorValue{Items: items})
	if sink.listCalls != 3 || !slices.Equal(sink.items[bound["ScratchList"]], items) {
		t.Fatal("in-place update missed")
	}
	panel.render(sink, style, MonitorValue{Items: nil})
	if sink.listCalls != 4 || len(sink.items[bound["ScratchList"]]) != 0 {
		t.Fatal("clearing the list missed")
	}
}

func TestMonitorSwitchesBetweenScalarAndList(t *testing.T) {
	panel, _ := newBoundMonitor(t)
	sink := newMonitorRenderSpy()
	color := mathf.Color{}
	size := mathf.NewVec2(100, 200)
	style := MonitorStyle{Appearance: MonitorAppearanceList, Label: "items", Color: color, Dimensions: size}
	panel.render(sink, MonitorStyle{Appearance: MonitorAppearanceScratch, Label: "score", Color: color}, MonitorValue{Text: "1"})
	panel.render(sink, style, MonitorValue{Items: nil})
	panel.render(sink, style, MonitorValue{Items: []string{}})
	if sink.listCalls != 1 || panel.active != MonitorAppearanceList {
		t.Fatal("empty list must activate once, independently of its nil representation")
	}
	panel.render(sink, MonitorStyle{Appearance: MonitorAppearanceDefaultLarge, Label: "score", Color: color}, MonitorValue{Text: "2"})
	panel.render(sink, style, MonitorValue{Items: nil})
	if sink.listCalls != 2 || panel.active != MonitorAppearanceList {
		t.Fatal("reactivating a list must restore its payload")
	}
}
