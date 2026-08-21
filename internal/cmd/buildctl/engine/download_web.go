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

package engine

import (
	"fmt"
	"path/filepath"

	"github.com/goplus/spx/v3/internal/cmd/buildctl/shared"
)

func downloadWebAssets(env engineDownloadEnv, mode string) error {
	templateName, err := webModeReleaseTemplateName(mode)
	if err != nil {
		return err
	}
	cachedName, err := webModeCachedTemplatePath(env.version, mode)
	if err != nil {
		return err
	}
	cachedZip := filepath.Join(env.goBinDir, cachedName)
	if shouldDownloadPreparedAsset(cachedZip) {
		if err := fetchEngineAsset(env, templateName, env.urlPrefix+templateName, cachedZip); err != nil {
			return err
		}
	}

	for _, name := range []string{
		"web_dlink_nothreads_debug.zip",
		"web_dlink_nothreads_release.zip",
		"web_nothreads_debug.zip",
		"web_nothreads_release.zip",
		"web_dlink_debug.zip",
		"web_dlink_release.zip",
		"web_debug.zip",
		"web_release.zip",
	} {
		if err := linkOrCopyFile(cachedZip, filepath.Join(env.templateDir, name)); err != nil {
			return err
		}
	}
	return nil
}

func webModeReleaseTemplateName(mode string) (string, error) {
	if err := shared.ValidateWebMode(mode); err != nil {
		return "", err
	}
	switch mode {
	case "normal":
		return "web.zip", nil
	case "worker":
		return "web-worker.zip", nil
	case "minigame":
		return "web-minigame.zip", nil
	case "miniprogram":
		return "web-miniprogram.zip", nil
	default:
		return "", fmt.Errorf("unsupported web-mode: %s", mode)
	}
}

func webModeCachedTemplatePath(version, mode string) (string, error) {
	if err := shared.ValidateWebMode(mode); err != nil {
		return "", err
	}
	if mode == "normal" {
		return fmt.Sprintf("gdspx%s_webpack.zip", version), nil
	}
	return fmt.Sprintf("gdspx%s_web%s.zip", version, mode), nil
}
