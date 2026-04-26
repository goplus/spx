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
	"os"

	"github.com/goplus/spx/v2/internal/cmd/buildctl/shared"
	toolpkg "github.com/goplus/spx/v2/internal/cmd/buildctl/tool"
)

var osStderr = os.Stderr

var errUsage = shared.ErrUsage

type buildEnvironment = shared.BuildEnvironment

func findRepoRoot() (string, error)          { return shared.FindRepoRoot() }
func fileExists(path string) bool            { return shared.FileExists(path) }
func copyFile(src, dst string) error         { return shared.CopyFile(src, dst) }
func defaultRuntimeVersion() (string, error) { return shared.DefaultRuntimeVersion() }
func resolveBuildEnvironment(repoRoot string, requestedPlatform string) (buildEnvironment, error) {
	return shared.ResolveBuildEnvironment(repoRoot, requestedPlatform)
}
func validateWebMode(mode string) error { return shared.ValidateWebMode(mode) }
func validateOptionalPlatform(platform string) error {
	return shared.ValidateOptionalPlatform(platform)
}
func ensureEngineSource(repoRoot string, run func(name string, args ...string) error) error {
	return shared.EnsureEngineSource(repoRoot, run)
}
func resolveMacOSVulkanSDKRoot(homeDir string, envSDK string) (string, error) {
	return shared.ResolveMacOSVulkanSDKRoot(homeDir, envSDK)
}
func shellQuote(value string) string               { return shared.ShellQuote(value) }
func currentEnvMap() map[string]string             { return shared.CurrentEnvMap() }
func envMapToSlice(env map[string]string) []string { return shared.EnvMapToSlice(env) }
func prependToPath(pathValue string, dirs ...string) string {
	return shared.PrependToPath(pathValue, dirs...)
}
func runCommandOutputWithEnv(workdir string, env []string, name string, args ...string) ([]byte, error) {
	return shared.RunCommandOutputWithEnv(workdir, env, name, args...)
}
func runStreamingCommand(workdir, name string, args ...string) error {
	return shared.RunStreamingCommand(workdir, name, args...)
}

var buildEnvRunStreaming = shared.RunStreamingCommand

func setupSCons() error                                    { return toolpkg.SetupSCons() }
func setupJDK() error                                      { return toolpkg.SetupJDK() }
func setupEMSDK() error                                    { return toolpkg.SetupEMSDK() }
func resolveJDKShellExports() (map[string]string, error)   { return toolpkg.ResolveJDKShellExports() }
func resolveEMSDKShellExports() (map[string]string, error) { return toolpkg.ResolveEMSDKShellExports() }
