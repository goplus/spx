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

package tool

import "github.com/goplus/spx/v2/internal/cmd/buildctl/shared"

type InstallConfig struct {
	Web bool
	Opt bool
}

type SetupNDKConfig struct {
	ManualInstall    bool
	NDKPath          string
	SkipVerification bool
}

type SetupSConsConfig struct{}
type SetupJDKConfig struct{}
type SetupEMSDKConfig struct{}

type AndroidNDKEnv struct {
	ArchiveName string
	DownloadURL string
	SDKRoot     string
	NDKRoot     string
	CacheDir    string
	ShellConfig string
}

type EMSDKEnvironment struct {
	RootDir string
	RepoDir string
}

func Run(args []string) error                      { return runTool(args) }
func ParseToolCleanAssetsArgs(args []string) error { return parseToolCleanAssetsArgs(args) }
func ParseToolInstallArgs(args []string) (InstallConfig, error) {
	cfg, err := parseToolInstallArgs(args)
	return InstallConfig{Web: cfg.web, Opt: cfg.opt}, err
}
func ParseToolSetupNDKArgs(args []string) (SetupNDKConfig, error) {
	cfg, err := parseToolSetupNDKArgs(args)
	return SetupNDKConfig{
		ManualInstall:    cfg.manualInstall,
		NDKPath:          cfg.ndkPath,
		SkipVerification: cfg.skipVerification,
	}, err
}
func ParseToolSetupSConsArgs(args []string) (SetupSConsConfig, error) {
	_, err := parseToolSetupSConsArgs(args)
	return SetupSConsConfig{}, err
}
func ParseToolSetupJDKArgs(args []string) (SetupJDKConfig, error) {
	_, err := parseToolSetupJDKArgs(args)
	return SetupJDKConfig{}, err
}
func ParseToolSetupEMSDKArgs(args []string) (SetupEMSDKConfig, error) {
	_, err := parseToolSetupEMSDKArgs(args)
	return SetupEMSDKConfig{}, err
}
func InstallTools(cfg InstallConfig, runner shared.ScriptRunner) error {
	return installTools(toolInstallConfig{web: cfg.Web, opt: cfg.Opt}, scriptRunnerAdapter{inner: runner})
}
func CleanInstalledAssets() error { return cleanInstalledAssets() }
func SetupAndroidNDK(cfg SetupNDKConfig) error {
	return setupAndroidNDK(toolSetupNDKConfig{
		manualInstall:    cfg.ManualInstall,
		ndkPath:          cfg.NDKPath,
		skipVerification: cfg.SkipVerification,
	})
}
func SetupSCons() error                                  { return setupSCons() }
func SetupJDK() error                                    { return setupJDK() }
func SetupEMSDK() error                                  { return setupEMSDK() }
func ResolveJDKShellExports() (map[string]string, error) { return resolveJDKShellExports() }
func ResolveEMSDKEnvironment() (EMSDKEnvironment, error) {
	env, err := resolveEMSDKEnvironment()
	return EMSDKEnvironment{RootDir: env.rootDir, RepoDir: env.repoDir}, err
}
func ResolveEMSDKVerificationEnvironment(env EMSDKEnvironment) (map[string]string, string, error) {
	return resolveEMSDKVerificationEnvironment(emsdkEnvironment{rootDir: env.RootDir, repoDir: env.RepoDir})
}
func ResolveEMSDKShellExports() (map[string]string, error) { return resolveEMSDKShellExports() }
func VerifyEMSDK(env EMSDKEnvironment) error {
	return verifyEMSDK(emsdkEnvironment{rootDir: env.RootDir, repoDir: env.RepoDir})
}
func EmscriptenCPPExecutableName() string                   { return emscriptenCPPExecutableName() }
func ParseJavaMajorVersion(output string) (int, bool)       { return parseJavaMajorVersion(output) }
func PrependToPath(pathValue string, dirs ...string) string { return prependToPath(pathValue, dirs...) }
func SelectEMSDKExports(before, after map[string]string) map[string]string {
	return selectEMSDKExports(before, after)
}
func UpdateNDKShellConfig(env AndroidNDKEnv) error {
	return updateNDKShellConfig(androidNDKEnv{
		archiveName: env.ArchiveName,
		downloadURL: env.DownloadURL,
		sdkRoot:     env.SDKRoot,
		ndkRoot:     env.NDKRoot,
		cacheDir:    env.CacheDir,
		shellConfig: env.ShellConfig,
	})
}
