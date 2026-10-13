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
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDecorativeUIScenesIgnoreMouse(t *testing.T) {
	for _, name := range []string{"UiDebug", "UiMeasure", "UiQuote", "UiSay"} {
		t.Run(name, func(t *testing.T) {
			parsed := parseMonitorScene(t, readUITemplateScene(t, name))
			for path, node := range parsed.nodes {
				if got := monitorSceneProperty(node, "mouse_filter"); got != "2" {
					t.Errorf("%s mouse_filter = %q, want 2 (ignore)", path, got)
				}
			}
		})
	}
}

func TestDisplayMonitorsStopMouse(t *testing.T) {
	parsed := parseMonitorScene(t, readUITemplateScene(t, "UiMonitor"))
	if got := monitorSceneProperty(parsed.nodes["."], "mouse_filter"); got != "2" {
		t.Errorf("monitor root mouse_filter = %q, want 2 (ignore)", got)
	}
	for _, root := range []string{"BG", "ValueOnly", "ScratchBG", "ScratchValueOnly"} {
		if got := monitorSceneProperty(parsed.nodes[root], "mouse_filter"); got != "0" {
			t.Errorf("%s mouse_filter = %q, want 0 (stop)", root, got)
		}
		for path, node := range parsed.nodes {
			if !strings.HasPrefix(path, root+"/") {
				continue
			}
			if got := monitorSceneProperty(node, "mouse_filter"); got != "2" {
				t.Errorf("%s mouse_filter = %q, want 2 (ignore)", path, got)
			}
		}
	}
	if got := monitorSceneProperty(parsed.nodes["ScratchSlider/V/Slider"], "mouse_filter"); got != "0" {
		t.Errorf("slider mouse_filter = %q, want 0 (stop)", got)
	}
}

func readUITemplateScene(t *testing.T, name string) string {
	t.Helper()
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller could not resolve test source path")
	}
	path := filepath.Join(filepath.Dir(sourceFile), "..", "..", "cmd", "spx", "template", "project", "engine", "ui", name+".tscn")
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		t.Fatalf("read %s scene: %v", name, err)
	}
	return string(data)
}
