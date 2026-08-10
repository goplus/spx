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

import "errors"

var ErrUsage = errors.New("usage")

type BuildEnvironment = buildEnvironment

type ScriptRunner interface {
	RunScript(relativePath string, args ...string) error
	RunCommand(workdir string, name string, args ...string) error
	RepoRootDir() string
}

type WorkflowRunner interface {
	ScriptRunner
	ListDemoDirs() ([]string, error)
	StopWebServers() error
}

type CommandRunner struct {
	RepoRoot string
}

func (r CommandRunner) RepoRootDir() string {
	return r.RepoRoot
}

func FindRepoRoot() (string, error) {
	return findRepoRoot()
}

func FileExists(path string) bool {
	return fileExists(path)
}

func DirExists(path string) bool {
	return dirExists(path)
}

func ResolveBuildEnvironment(repoRoot string, requestedPlatform string) (BuildEnvironment, error) {
	return resolveBuildEnvironment(repoRoot, requestedPlatform)
}

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

func CurrentEnvMap() map[string]string {
	return currentEnvMap()
}

func EnvMapToSlice(env map[string]string) []string {
	return envMapToSlice(env)
}

func RunCommandOutput(name string, args ...string) ([]byte, error) {
	return runCommandOutput(name, args...)
}

func RunStreamingCommand(workdir, name string, args ...string) error {
	return runStreamingCommand(workdir, name, args...)
}

func EnsureGoPath() (string, error) {
	return ensureGoPath()
}

func DefaultRuntimeVersion() (string, error) {
	return defaultRuntimeVersion()
}

func CopyFile(src, dst string) error {
	return copyFile(src, dst)
}

func CopyDir(src, dst string) error {
	return copyDir(src, dst)
}

func WriteNamedZip(dst string, namedFiles map[string]string) error {
	return writeNamedZip(dst, namedFiles)
}

func ZipDirectory(srcDir, dstZip string) error {
	return zipDirectory(srcDir, dstZip)
}

func ExtractZip(srcZip, dstDir string) error {
	return extractZip(srcZip, dstDir)
}

func FetchURLToFile(url, dst string) error {
	return fetchURLToFile(url, dst)
}
