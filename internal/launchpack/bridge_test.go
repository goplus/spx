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
	if _, _, err := buildSourceBridge(context.Background(), cfg, "bridge.so", IO{}); err == nil || !strings.Contains(err.Error(), "bridge package") {
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
	args := sourceBridgeBuildArgsForGOOS(cfg, "bridge.dll", "windows")
	if !slices.Contains(args, "-ldflags=-extldflags=-Wl,--allow-multiple-definition") {
		t.Fatalf("Windows linker flags = %q", args)
	}
}

func TestSourceBridgeEnvironmentDropsInheritedCGOFlags(t *testing.T) {
	base := []string{"PATH=/tools", "CGO_CFLAGS=-I/untrusted", "CGO_LDFLAGS=-L/untrusted", "CGO_ENABLED=0", "CGO_CPPFLAGS=-DOTHER", "GOOS=other", "GOWORK=other", "GOFLAGS=-tags=other", "CGO_WITHOUT_EQUALS"}
	original := append([]string(nil), base...)
	got := sourceBridgeEnv(Config{GoWork: "off"}, base)
	want := []string{"PATH=/tools", "CGO_WITHOUT_EQUALS", "GOFLAGS=", "GOWORK=off", "GOOS=" + runtime.GOOS, "GOARCH=" + runtime.GOARCH, "CGO_ENABLED=1"}
	if !slices.Equal(got, want) || !slices.Equal(base, original) {
		t.Fatalf("bridge environment = %q; original = %q", got, base)
	}
}
