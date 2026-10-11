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

import "path/filepath"

func downloadAndroidAssets(env engineDownloadEnv) error {
	return downloadBinariesFromZip(env, "android.zip", []binaryInstall{
		{"android_debug.apk", filepath.Join(env.templateDir, "android_debug.apk")},
		{"android_release.apk", filepath.Join(env.templateDir, "android_release.apk")},
		{"android_source.zip", filepath.Join(env.templateDir, "android_source.zip")},
	})
}

func downloadIOSAssets(env engineDownloadEnv) error {
	return fetchEngineAsset(env, "ios.zip", env.urlPrefix+"ios.zip", filepath.Join(env.templateDir, "ios.zip"))
}
