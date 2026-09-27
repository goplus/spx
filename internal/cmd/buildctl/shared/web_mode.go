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

import "fmt"

// WebModeSpec describes the artifacts and execution mode of a Web export.
type WebModeSpec struct {
	ReleaseTemplate string
	CacheSuffix     string
	ExportCommand   string
	ExportZip       string
	Threaded        bool
}

var webModes = map[string]WebModeSpec{
	"normal":      {"web.zip", "webpack.zip", "exportweb", "spx_web.zip", false},
	"worker":      {"web-worker.zip", "webworker.zip", "exportwebworker", "spx_web_worker.zip", true},
	"minigame":    {"web-minigame.zip", "webminigame.zip", "exportminigame", "spx_web_minigame.zip", false},
	"miniprogram": {"web-miniprogram.zip", "webminiprogram.zip", "exportminiprogram", "spx_web_miniprogram.zip", false},
}

// ResolveWebMode returns a copy of the mode specification. Callers choose their
// own default before resolving; an empty mode is not valid here.
func ResolveWebMode(mode string) (WebModeSpec, error) {
	spec, ok := webModes[mode]
	if !ok {
		return WebModeSpec{}, fmt.Errorf("unsupported web-mode: %s", mode)
	}
	return spec, nil
}

func ValidateWebMode(mode string) error {
	_, err := ResolveWebMode(mode)
	return err
}
