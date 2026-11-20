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
)

func downloadDesktopAssets(env engineDownloadEnv, editorOnly bool) error {
	binaryPlatform, ext := env.platform, ""
	switch env.platform {
	case "linux":
		binaryPlatform = "linuxbsd"
	case "windows":
		ext = ".exe"
	}
	binary := func(target, prefix string) binaryInstall {
		return binaryInstall{
			assetName: fmt.Sprintf("godot.%s.%s.%s%s", binaryPlatform, target, env.arch, ext),
			dst:       filepath.Join(env.goBinDir, prefix+env.version+ext),
		}
	}

	archive := env.platform + "-" + env.arch + ".zip"
	if editorOnly {
		editor := binary("editor", "gdspx")
		if shouldDownloadPreparedAsset(editor.dst) {
			return downloadBinariesFromZip(env, "editor-"+archive, editor)
		}
		return nil
	}

	release := binary("template_release", "gdspxrt")
	if env.platform == "macos" {
		if shouldDownloadPreparedAsset(release.dst) {
			if err := downloadBinariesFromZip(env, archive, release); err != nil {
				return err
			}
		}
		template := filepath.Join(env.templateDir, "macos.zip")
		if shouldDownloadPreparedAsset(template) {
			return fetchEngineAsset(env, "macos.zip", template)
		}
		return nil
	}

	debug := binary("template_debug", "gdspxrtdbg")
	if shouldDownloadPreparedAsset(release.dst) || shouldDownloadPreparedAsset(debug.dst) {
		if err := downloadBinariesFromZip(env, archive, release, debug); err != nil {
			return err
		}
	}
	if err := linkDesktopTemplate(env, "debug", debug.dst); err != nil {
		return err
	}
	return linkDesktopTemplate(env, "release", release.dst)
}

func linkDesktopTemplate(env engineDownloadEnv, target, binary string) error {
	if env.platform == "linux" {
		return linkOrCopyFile(binary, filepath.Join(env.templateDir, "linux_"+target+"."+env.arch))
	}
	stem := filepath.Join(env.templateDir, "windows_"+target+"_"+env.arch)
	if err := linkOrCopyFile(binary, stem+".exe"); err != nil {
		return err
	}
	return linkOrCopyFile(binary, stem+"_console.exe")
}
