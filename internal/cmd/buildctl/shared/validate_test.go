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

package shared

import (
	"bytes"
	"errors"
	"flag"
	"strings"
	"testing"
)

func TestParseNoArgs(t *testing.T) {
	var output bytes.Buffer
	if err := ParseNoArgs("doctor", "Usage: buildctl doctor", nil, &output); err != nil {
		t.Fatalf("ParseNoArgs returned error: %v", err)
	}
	if err := ParseNoArgs("doctor", "Usage: buildctl doctor", []string{"extra"}, &output); !errors.Is(err, ErrUsage) {
		t.Fatalf("ParseNoArgs extra argument error = %v, want ErrUsage", err)
	}
	if !strings.Contains(output.String(), "Usage: buildctl doctor") {
		t.Fatalf("ParseNoArgs did not print usage: %q", output.String())
	}
	if err := ParseNoArgs("doctor", "Usage: buildctl doctor", []string{"--help"}, &output); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("ParseNoArgs --help error = %v, want flag.ErrHelp", err)
	}
}

func TestValidateOptionalPlatform(t *testing.T) {
	for _, platform := range []string{"", "android", "ios", "web", "linux", "windows", "macos"} {
		t.Run("accept/"+platform, func(t *testing.T) {
			if err := ValidateOptionalPlatform(platform); err != nil {
				t.Fatalf("ValidateOptionalPlatform(%q) returned error: %v", platform, err)
			}
		})
	}
	if err := ValidateOptionalPlatform("plan9"); err == nil || !strings.Contains(err.Error(), "unsupported platform: plan9") {
		t.Fatalf("ValidateOptionalPlatform(plan9) error = %v, want unsupported-platform error", err)
	}
}

func TestResolveWebModeContract(t *testing.T) {
	want := map[string]WebModeSpec{
		"normal":      {ReleaseTemplate: "web.zip", CacheSuffix: "webpack.zip", ExportCommand: "exportweb", ExportZip: "spx_web.zip"},
		"worker":      {ReleaseTemplate: "web-worker.zip", CacheSuffix: "webworker.zip", ExportCommand: "exportwebworker", ExportZip: "spx_web_worker.zip", Threaded: true},
		"minigame":    {ReleaseTemplate: "web-minigame.zip", CacheSuffix: "webminigame.zip", ExportCommand: "exportminigame", ExportZip: "spx_web_minigame.zip"},
		"miniprogram": {ReleaseTemplate: "web-miniprogram.zip", CacheSuffix: "webminiprogram.zip", ExportCommand: "exportminiprogram", ExportZip: "spx_web_miniprogram.zip"},
	}
	for mode, expected := range want {
		t.Run(mode, func(t *testing.T) {
			got, err := ResolveWebMode(mode)
			if err != nil {
				t.Fatalf("ResolveWebMode(%q) returned error: %v", mode, err)
			}
			if got != expected {
				t.Fatalf("ResolveWebMode(%q) = %#v, want %#v", mode, got, expected)
			}
			if err := ValidateWebMode(mode); err != nil {
				t.Fatalf("ValidateWebMode(%q) returned error: %v", mode, err)
			}
		})
	}
	for _, mode := range []string{"", "unknown"} {
		t.Run("reject/"+mode, func(t *testing.T) {
			if _, err := ResolveWebMode(mode); err == nil || !strings.Contains(err.Error(), "unsupported web-mode") {
				t.Fatalf("ResolveWebMode(%q) error = %v, want unsupported-web-mode error", mode, err)
			}
			if err := ValidateWebMode(mode); err == nil {
				t.Fatalf("ValidateWebMode(%q) succeeded", mode)
			}
		})
	}
}
