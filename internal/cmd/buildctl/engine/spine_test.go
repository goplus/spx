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

package engine

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/goplus/spx/v4/internal/runtimevariant"
)

func TestSpineRejectsPublishedAssets(t *testing.T) {
	for _, enabled := range []string{"0", "1"} {
		t.Run("SPX_SPINE="+enabled, func(t *testing.T) {
			root := t.TempDir()
			goPath := filepath.Join(root, "spine")
			t.Setenv("SPX_SPINE", enabled)
			t.Setenv("GOPATH", goPath)
			t.Setenv("SPX_SPINE_GOPATH", goPath)
			t.Setenv("GOBIN", "")
			want := "SPX_SPINE=1"
			if enabled == "0" {
				mustWriteFile(t, filepath.Join(goPath, "bin", runtimevariant.SpineMarker), []byte("reserved"))
				want = "reserved for Spine"
			}
			for name, call := range map[string]func() error{
				"runtime":    func() error { return DownloadEngineAssets(DownloadConfig{Runtime: true}, root) },
				"web":        func() error { return DownloadEngineAssets(DownloadConfig{Platform: "web"}, root) },
				"editor":     func() error { return PrepareHostEditorAsset(root, "") },
				"linux-pack": func() error { return PrepareLinuxRuntimePackAssets(root, "release-assets") },
			} {
				// The release pack API is Linux/amd64-only and rejects other hosts first.
				if name == "linux-pack" && enabled == "0" && (runtime.GOOS != "linux" || runtime.GOARCH != "amd64") {
					continue
				}
				if err := call(); err == nil || !strings.Contains(err.Error(), want) {
					t.Errorf("%s should reject default download: %v", name, err)
				}
			}
			if enabled == "1" {
				if _, err := os.Stat(goPath); !os.IsNotExist(err) {
					t.Fatalf("rejected download created a workspace: %v", err)
				}
			} else {
				entries, err := os.ReadDir(filepath.Join(goPath, "bin"))
				if err != nil || len(entries) != 1 || entries[0].Name() != runtimevariant.SpineMarker {
					t.Fatalf("rejected download changed reserved workspace: %v, %v", entries, err)
				}
			}
			if _, err := os.Stat(filepath.Join(root, "internal")); !os.IsNotExist(err) {
				t.Fatalf("rejected download created cache: %v", err)
			}
		})
	}
}

func TestSpineBuildGuardsBeforePreparingTools(t *testing.T) {
	for _, enabled := range []string{"0", "1"} {
		t.Run("SPX_SPINE="+enabled, func(t *testing.T) {
			root := t.TempDir()
			goPath := filepath.Join(root, "spine")
			t.Setenv("GOPATH", goPath)
			t.Setenv("GOBIN", "")
			t.Setenv("SPX_SPINE", enabled)
			t.Setenv("SPX_SPINE_GOPATH", goPath)
			want := "not empty"
			if enabled == "0" {
				mustWriteFile(t, filepath.Join(goPath, "bin", runtimevariant.SpineMarker), nil)
				want = "reserved for Spine"
			} else {
				mustWriteFile(t, filepath.Join(goPath, "bin", "default-runtime"), nil)
			}
			if enabled == "0" {
				if err := BuildEngine(BuildConfig{Target: "editor"}, root); err == nil || !strings.Contains(err.Error(), want) {
					t.Fatalf("direct engine build bypassed isolation: %v", err)
				}
			}
			buildEnv := buildEnvironment{TemplateDir: filepath.Join(root, "templates")}
			if _, _, err := prepareEngineBuildEnvironment(buildEnv); err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("build preparation bypassed isolation: %v", err)
			}
			if _, err := os.Stat(buildEnv.TemplateDir); !os.IsNotExist(err) {
				t.Fatalf("created templates before rejecting workspace: %v", err)
			}
		})
	}
}
