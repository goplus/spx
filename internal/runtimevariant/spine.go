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

// Package runtimevariant selects optional local runtime builds without falling
// back to published or embedded default-feature artifacts.
package runtimevariant

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const SpineMarker = ".spx-spine"

func SpineEnabled(env []string) (bool, error) {
	value := ""
	found := false
	for _, entry := range env {
		key, current, ok := strings.Cut(entry, "=")
		if !ok || key != "SPX_SPINE" {
			continue
		}
		if found {
			return false, fmt.Errorf("duplicate SPX_SPINE")
		}
		value, found = current, true
	}
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "0", "false", "no", "off":
		return false, nil
	case "1", "true", "yes", "on":
		return true, nil
	default:
		return false, fmt.Errorf("invalid SPX_SPINE %q: use 0 or 1", value)
	}
}

// LocalSpineBin validates that a requested optional runtime is selected only
// from the workspace prepared by buildctl. bin may be supplied by the CLI;
// otherwise it is resolved from the explicit environment GOPATH.
func LocalSpineBin(env []string, bin string) (string, bool, error) {
	enabled, err := SpineEnabled(env)
	if err != nil {
		return "", false, err
	}
	var goPath string
	for _, entry := range env {
		if key, value, ok := strings.Cut(entry, "="); ok && key == "GOPATH" {
			goPath = value
		}
	}
	if bin == "" && goPath != "" && len(filepath.SplitList(goPath)) == 1 {
		bin = filepath.Join(goPath, "bin")
	}
	marked := false
	if bin != "" {
		info, err := os.Stat(filepath.Join(bin, SpineMarker))
		marked = err == nil && info.Mode().IsRegular()
	}
	if !enabled {
		if marked {
			return "", false, fmt.Errorf("this GOPATH is reserved for Spine; set SPX_SPINE=1 or use your normal GOPATH")
		}
		return "", false, nil
	}
	if goPath == "" || !filepath.IsAbs(goPath) || len(filepath.SplitList(goPath)) != 1 || filepath.Clean(bin) != filepath.Join(filepath.Clean(goPath), "bin") || !marked {
		return "", true, fmt.Errorf("Spine runtime requires a dedicated GOPATH prepared with SPX_SPINE=1 make dev; default published and embedded runtimes are not supported")
	}
	return bin, true, nil
}
