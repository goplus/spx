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

func downloadDesktopAssets(env engineDownloadEnv, editor bool) error {
	platformName := env.platform
	postfix := ""
	if env.platform == "windows" {
		postfix = ".exe"
	}

	if editor {
		zipName := fmt.Sprintf("editor-%s-%s.zip", env.platform, env.arch)
		binaryName := fmt.Sprintf("godot.%s.editor.%s%s", platformName, env.arch, postfix)
		finalBinary := filepath.Join(env.goBinDir, fmt.Sprintf("gdspx%s%s", env.version, postfix))
		if !shouldDownloadPreparedAsset(finalBinary) {
			return nil
		}
		return downloadBinaryFromZip(env, zipName, binaryName, finalBinary)
	}

	zipName := fmt.Sprintf("%s-%s.zip", env.platform, env.arch)
	releaseBinaryName := fmt.Sprintf("godot.%s.template_release.%s%s", platformName, env.arch, postfix)
	templateBinary := filepath.Join(env.goBinDir, fmt.Sprintf("gdspxrt%s%s", env.version, postfix))

	switch env.platform {
	case "windows":
		debugBinary := filepath.Join(env.goBinDir, fmt.Sprintf("gdspxrtdbg%s.exe", env.version))
		if shouldDownloadPreparedAsset(templateBinary) || shouldDownloadPreparedAsset(debugBinary) {
			if err := downloadBinariesFromZip(env, zipName, []binaryInstall{
				{releaseBinaryName, templateBinary},
				{fmt.Sprintf("godot.%s.template_debug.%s%s", platformName, env.arch, postfix), debugBinary},
			}); err != nil {
				return err
			}
		}
		for _, name := range []string{"windows_debug_" + env.arch + ".exe", "windows_debug_" + env.arch + "_console.exe"} {
			if err := linkOrCopyFile(debugBinary, filepath.Join(env.templateDir, name)); err != nil {
				return err
			}
		}
		for _, name := range []string{"windows_release_" + env.arch + ".exe", "windows_release_" + env.arch + "_console.exe"} {
			if err := linkOrCopyFile(templateBinary, filepath.Join(env.templateDir, name)); err != nil {
				return err
			}
		}
	case "macos":
		if shouldDownloadPreparedAsset(templateBinary) {
			if err := downloadBinaryFromZip(env, zipName, releaseBinaryName, templateBinary); err != nil {
				return err
			}
		}
		macosZip := filepath.Join(env.templateDir, "macos.zip")
		if shouldDownloadPreparedAsset(macosZip) {
			if err := fetchEngineAsset(env, "macos.zip", env.urlPrefix+"macos.zip", macosZip); err != nil {
				return err
			}
		}
	}

	return nil
}
