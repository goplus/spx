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
	"strings"
	"testing"
)

func TestSpineRejectsPublishedAssets(t *testing.T) {
	t.Setenv("SPX_SPINE", "1")
	for name, call := range map[string]func() error{
		"runtime":    func() error { return DownloadEngineAssets(DownloadConfig{Runtime: true}, t.TempDir()) },
		"web":        func() error { return DownloadEngineAssets(DownloadConfig{Platform: "web"}, t.TempDir()) },
		"editor":     func() error { return PrepareHostEditorAsset(t.TempDir(), "") },
		"linux-pack": func() error { return PrepareLinuxRuntimePackAssets(t.TempDir(), "release-assets") },
	} {
		if err := call(); err == nil || !strings.Contains(err.Error(), "SPX_SPINE=1") {
			t.Errorf("%s should reject default download: %v", name, err)
		}
	}
}
