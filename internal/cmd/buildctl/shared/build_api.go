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

func ResolveSPXModuleSource(repoRoot string) (string, error) {
	return resolveSPXModuleSource(repoRoot)
}

func (env BuildEnvironment) ShellExports() string {
	return buildEnvironment(env).shellExports()
}

func ShellQuote(value string) string {
	return shellQuote(value)
}

func EnsureEngineSource(repoRoot string, run func(name string, args ...string) error) error {
	return ensureEngineSource(repoRoot, run)
}

func ResolveMacOSVulkanSDKRoot(homeDir string, envSDK string) (string, error) {
	return resolveMacOSVulkanSDKRoot(homeDir, envSDK)
}

func MacOSVulkanSDKShellExports(sdkRoot string) string {
	return macOSVulkanSDKShellExports(sdkRoot)
}

func ValidateWebMode(mode string) error {
	return validateWebMode(mode)
}

func ValidateOptionalPlatform(platform string) error {
	return validateOptionalPlatform(platform)
}

func CurrentBuildEnv() (map[string]string, error) {
	return currentBuildEnv()
}

func EnvMapToSlice(env map[string]string) []string {
	return envMapToSlice(env)
}
