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

package launchpack

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestBuildSourceBridgeChecksGraphAndCleansFailures(t *testing.T) {
	for _, tt := range []struct {
		name, want, body string
		graphFailure     int
		keep             bool
	}{
		{name: "success", body: "printf bridge > \"$out\""},
		{name: "retain work", body: "printf bridge > \"$out\"", keep: true},
		{name: "command fails", body: "exit 3", want: "build source interpreter bridge"},
		{name: "no output", body: "exit 0", want: "lstat source interpreter bridge"},
		{name: "graph before", body: "printf bridge > \"$out\"", graphFailure: 1, want: "before source bridge build"},
		{name: "graph after", body: "printf bridge > \"$out\"", graphFailure: 2, want: "after source bridge build"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validLaunchpackConfig(t)
			launcherGoFixture(t, &cfg, tt.body)
			temp := t.TempDir()
			t.Setenv("TMPDIR", temp)
			var log bytes.Buffer
			cfg.IO.Stderr = &log
			if tt.keep {
				cfg.BuildFlags = []string{"-work"}
			}
			checks := 0
			sentinel := errors.New("graph changed")
			cfg.VerifyGraph = func(context.Context) error {
				checks++
				if checks == tt.graphFailure {
					return sentinel
				}
				return nil
			}
			path, cleanup, err := BuildSourceBridge(context.Background(), cfg)
			if tt.want != "" {
				if err == nil || !strings.Contains(err.Error(), tt.want) || cleanup != nil || path != "" {
					t.Fatalf("BuildSourceBridge = %q, %v", path, err)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				assertRuntimeFile(t, path, "bridge")
				cleanup()
				if tt.keep {
					if !strings.Contains(log.String(), "SPXBRIDGEWORK="+filepath.Dir(path)) {
						t.Fatalf("work log = %q", log.String())
					}
					assertRuntimeFile(t, path, "bridge")
				} else if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Fatalf("bridge not removed: %v", err)
				}
			}
			entries, err := os.ReadDir(temp)
			if err != nil {
				t.Fatal(err)
			}
			if !tt.keep && len(entries) != 0 {
				t.Fatalf("temporary directory leaked: %v", entries)
			}
		})
	}
	cfg := validLaunchpackConfig(t)
	cfg.BridgePackage = ""
	if _, _, err := BuildSourceBridge(context.Background(), cfg); err == nil || !strings.Contains(err.Error(), "bridge package") {
		t.Fatalf("missing bridge package: %v", err)
	}
	for _, goos := range []string{"linux", "darwin", "windows"} {
		name, err := bridgeFileName(goos, "arm64")
		if err != nil || !strings.Contains(name, goos+"-arm64") {
			t.Fatalf("bridgeFileName(%q) = %q, %v", goos, name, err)
		}
	}
	if _, err := bridgeFileName("plan9", "amd64"); err == nil {
		t.Fatal("unsupported host accepted")
	}
}

func TestBuildSourceBridgeUnsupportedPlatform(t *testing.T) {
	switch runtime.GOOS {
	case "darwin", "linux", "windows":
		t.Skip("source bridge builds are supported on this host")
	}
	dir := t.TempDir()
	cfg := Config{WorkDir: dir, GoCommand: filepath.Join(dir, "go"), BridgePackage: "./bridge"}
	writeProjectTestFile(t, cfg.GoCommand, "")
	path, cleanup, err := BuildSourceBridge(t.Context(), cfg)
	want := "host platform " + runtime.GOOS + "/" + runtime.GOARCH + " is not supported"
	if err == nil || !strings.Contains(err.Error(), want) || path != "" || cleanup != nil {
		t.Fatalf("BuildSourceBridge returned path %q, cleanup %t, error %v; want %q", path, cleanup != nil, err, want)
	}
}

func TestGoBuildCommand(t *testing.T) {
	base := []string{
		"PATH=/tools", "CGO_CFLAGS=-I/untrusted", "CGO_LDFLAGS=-L/untrusted", "CGO_ENABLED=0",
		"CGO_CPPFLAGS=-DOTHER", "GOOS=other", "GOWORK=other", "GOFLAGS=-tags=other", "CGO_WITHOUT_EQUALS",
	}
	original := slices.Clone(base)
	cfg := Config{GoCommand: "go", GoWork: "off", IO: IO{Env: base}}
	for _, tt := range []struct {
		mode      string
		inherited []string
		cgo       string
	}{
		{"exe", []string{"PATH=/tools", "CGO_CFLAGS=-I/untrusted", "CGO_LDFLAGS=-L/untrusted", "CGO_CPPFLAGS=-DOTHER", "CGO_WITHOUT_EQUALS"}, "0"},
		{"c-shared", []string{"PATH=/tools", "CGO_WITHOUT_EQUALS"}, "1"},
	} {
		t.Run(tt.mode, func(t *testing.T) {
			cmd := cfg.goBuildCommand(context.Background(), tt.mode, "out", "./target")
			args := []string{"go", "build", "-buildmode=" + tt.mode}
			if runtime.GOOS == "windows" && tt.mode == "c-shared" {
				args = append(args, "-ldflags=-extldflags=-Wl,--allow-multiple-definition")
			}
			if want := append(args, "-o", "out", "./target"); !slices.Equal(cmd.Args, want) {
				t.Errorf("build arguments = %q, want %q", cmd.Args, want)
			}
			want := append(tt.inherited, "GOFLAGS=", "GOWORK=off", "GOOS="+runtime.GOOS, "GOARCH="+runtime.GOARCH, "CGO_ENABLED="+tt.cgo)
			if !slices.Equal(cmd.Env, want) || !slices.Equal(base, original) {
				t.Fatalf("build environment = %q; original = %q", cmd.Env, base)
			}
		})
	}
}
