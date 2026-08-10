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
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/goplus/spx/v3/internal/cmd/buildctl/shared"
	"github.com/goplus/spx/v3/internal/release"
)

type engineDownloadEnv struct {
	repoRoot              string
	version               string
	platform              string
	arch                  string
	goBinDir              string
	templateDir           string
	cacheDir              string
	urlPrefix             string
	runtimeAssetURLPrefix string
	runtimePackAsset      string
	assetDir              string
	verifyManifest        bool
	allowMissingManifest  bool
	manifest              *release.RuntimeManifest
}

var engineDownloadFetcher = fetchURLToFile
var engineDownloadResolveEnv = resolveEngineDownloadEnv
var engineDownloadHTTPClient = &http.Client{Timeout: 30 * time.Minute}

func downloadEngineAssets(cfg engineDownloadConfig, repoRoot string) error {
	env, err := engineDownloadResolveEnv(repoRoot, cfg.platform)
	if err != nil {
		return err
	}
	if cfg.assetDir != "" {
		env.assetDir = cfg.assetDir
		if !filepath.IsAbs(env.assetDir) {
			env.assetDir = filepath.Join(repoRoot, env.assetDir)
		}
		env.assetDir = filepath.Clean(env.assetDir)
		info, err := os.Stat(env.assetDir)
		if err != nil {
			return fmt.Errorf("open engine asset directory: %w", err)
		}
		if !info.IsDir() {
			return fmt.Errorf("engine asset path is not a directory: %s", env.assetDir)
		}
		env.allowMissingManifest = cfg.sameRunArtifacts
	}
	if env.verifyManifest {
		if err := loadEngineAssetManifest(&env); err != nil {
			return err
		}
	}

	if cfg.runtime {
		if err := downloadHostRuntimeAssets(env); err != nil {
			return err
		}
		if cfg.skipRuntimePack {
			return nil
		}
		if err := downloadRuntimePack(env); err != nil {
			return fmt.Errorf("download runtime asset bundle: %w", err)
		}
		return nil
	}

	return downloadPlatformAssets(env, cfg.mode, false)
}

func resolveEngineDownloadEnv(repoRoot, platform string) (engineDownloadEnv, error) {
	buildEnv, err := shared.ResolveBuildEnvironment(repoRoot, platform)
	if err != nil {
		return engineDownloadEnv{}, err
	}
	lock := release.DefaultRuntimeLock()
	if buildEnv.Version != lock.RuntimeVersion {
		return engineDownloadEnv{}, fmt.Errorf("resolved runtime version %q does not match runtime lock %q", buildEnv.Version, lock.RuntimeVersion)
	}

	env := engineDownloadEnv{
		repoRoot:              buildEnv.RepoRoot,
		version:               buildEnv.Version,
		platform:              buildEnv.Platform,
		arch:                  buildEnv.Arch,
		goBinDir:              filepath.Join(buildEnv.GoPath, "bin"),
		templateDir:           buildEnv.TemplateDir,
		cacheDir:              filepath.Join(repoRoot, "internal", "cmd", "buildctl", "bin"),
		urlPrefix:             lock.RuntimeAssetDownloadURL(""),
		runtimeAssetURLPrefix: lock.RuntimeAssetDownloadURL(""),
		runtimePackAsset:      release.RuntimeAssetZipName,
		verifyManifest:        true,
	}

	if err := os.MkdirAll(env.goBinDir, 0o755); err != nil {
		return engineDownloadEnv{}, err
	}
	if err := os.MkdirAll(env.templateDir, 0o755); err != nil {
		return engineDownloadEnv{}, err
	}
	if err := os.MkdirAll(env.cacheDir, 0o755); err != nil {
		return engineDownloadEnv{}, err
	}
	return env, nil
}

func downloadHostRuntimeAssets(env engineDownloadEnv) error {
	if err := downloadPlatformAssets(env, "", false); err != nil {
		return err
	}
	return downloadPlatformAssets(env, "editor", true)
}

func downloadPlatformAssets(env engineDownloadEnv, mode string, editor bool) error {
	switch env.platform {
	case "android":
		return downloadAndroidAssets(env)
	case "ios":
		return downloadIOSAssets(env)
	case "web":
		if mode == "" {
			mode = "normal"
		}
		return downloadWebAssets(env, mode)
	case "linux", "windows", "macos":
		return downloadDesktopAssets(env, editor)
	default:
		return fmt.Errorf("unsupported platform for engine download: %s", env.platform)
	}
}

func downloadRuntimePack(env engineDownloadEnv) error {
	versionedPack := filepath.Join(env.goBinDir, fmt.Sprintf("gdspxrt%s.pck", env.version))
	zipName := env.runtimePackAsset
	if zipName == "" {
		zipName = release.RuntimeAssetZipName
	}
	urlPrefix := env.runtimeAssetURLPrefix
	if urlPrefix == "" {
		urlPrefix = env.urlPrefix
	}
	url := urlPrefix + zipName
	zipPath := filepath.Join(env.cacheDir, zipName)
	if err := fetchEngineAsset(env, zipName, url, zipPath); err != nil {
		return err
	}
	defer os.Remove(zipPath)

	extractDir, err := os.MkdirTemp(env.cacheDir, "runtime-pck-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(extractDir)

	if err := extractZip(zipPath, extractDir); err != nil {
		return err
	}
	entries, err := os.ReadDir(extractDir)
	if err != nil {
		return err
	}
	destinations := map[string]string{
		"gdspxrt.pck":         versionedPack,
		"runtime.gdextension": filepath.Join(env.goBinDir, "runtime.gdextension"),
	}
	seen := make(map[string]struct{}, len(destinations))
	for _, entry := range entries {
		_, ok := destinations[entry.Name()]
		if !ok || entry.IsDir() {
			return fmt.Errorf("runtime asset bundle contains unsupported entry %q", entry.Name())
		}
		src := filepath.Join(extractDir, entry.Name())
		if info, err := os.Stat(src); err != nil {
			return err
		} else if !info.Mode().IsRegular() {
			return fmt.Errorf("runtime asset bundle entry %q is not a regular file", entry.Name())
		}
		seen[entry.Name()] = struct{}{}
	}
	for name := range destinations {
		if _, ok := seen[name]; !ok {
			return fmt.Errorf("runtime asset bundle is missing %s", name)
		}
	}
	for _, name := range []string{"runtime.gdextension", "gdspxrt.pck"} {
		if err := copyEngineAssetAtomically(filepath.Join(extractDir, name), destinations[name]); err != nil {
			return err
		}
	}
	return nil
}

func downloadAndroidAssets(env engineDownloadEnv) error {
	url := env.urlPrefix + "android.zip"
	zipPath := filepath.Join(env.cacheDir, "android.zip")
	if err := fetchEngineAsset(env, "android.zip", url, zipPath); err != nil {
		return err
	}
	defer os.Remove(zipPath)

	extractDir, err := os.MkdirTemp(env.cacheDir, "android-assets-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(extractDir)

	if err := extractZip(zipPath, extractDir); err != nil {
		return err
	}

	requiredFiles := []string{"android_debug.apk", "android_release.apk", "android_source.zip"}
	for _, name := range requiredFiles {
		src := filepath.Join(extractDir, name)
		info, err := os.Stat(src)
		if err != nil {
			return fmt.Errorf("Android asset bundle is missing %s: %w", name, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("Android asset bundle entry %s is not a regular file", name)
		}
	}
	for _, name := range requiredFiles {
		src := filepath.Join(extractDir, name)
		if err := copyEngineAssetAtomically(src, filepath.Join(env.templateDir, name)); err != nil {
			return err
		}
	}
	return nil
}

func downloadIOSAssets(env engineDownloadEnv) error {
	return fetchEngineAsset(env, "ios.zip", env.urlPrefix+"ios.zip", filepath.Join(env.templateDir, "ios.zip"))
}

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

func downloadDesktopAssets(env engineDownloadEnv, editor bool) error {
	platformName := env.platform
	postfix := ""
	if env.platform == "linux" {
		platformName = "linuxbsd"
	}
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
	case "linux":
		debugBinary := filepath.Join(env.goBinDir, fmt.Sprintf("gdspxrtdbg%s", env.version))
		if shouldDownloadPreparedAsset(templateBinary) || shouldDownloadPreparedAsset(debugBinary) {
			if err := downloadBinariesFromZip(env, zipName, []binaryInstall{
				{releaseBinaryName, templateBinary},
				{fmt.Sprintf("godot.%s.template_debug.%s", platformName, env.arch), debugBinary},
			}); err != nil {
				return err
			}
		}
		if err := linkOrCopyFile(debugBinary, filepath.Join(env.templateDir, "linux_debug."+env.arch)); err != nil {
			return err
		}
		if err := linkOrCopyFile(templateBinary, filepath.Join(env.templateDir, "linux_release."+env.arch)); err != nil {
			return err
		}
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

func shouldDownloadPreparedAsset(path string) bool {
	return shouldRefreshPreparedAssets() || !shared.FileExists(path)
}

func shouldRefreshPreparedAssets() bool {
	if value, ok := envFlagValue("SPX_PREPARE_FORCE_REFRESH"); ok {
		return flagValueEnabled(value)
	}
	// GitHub Actions may restore a fallback cache entry via restore-keys before
	// prepare runs, so CI refreshes assets while local prepares reuse GOPATH/bin.
	return envFlagEnabled("GITHUB_ACTIONS")
}

func envFlagEnabled(name string) bool {
	value, _ := envFlagValue(name)
	return flagValueEnabled(value)
}

func envFlagValue(name string) (string, bool) {
	value, ok := os.LookupEnv(name)
	if !ok {
		return "", false
	}
	return strings.TrimSpace(value), true
}

func flagValueEnabled(value string) bool {
	switch strings.ToLower(value) {
	case "", "0", "false", "no", "off":
		return false
	default:
		return true
	}
}

func downloadBinaryFromZip(env engineDownloadEnv, zipName, assetName, dst string) error {
	return downloadBinariesFromZip(env, zipName, []binaryInstall{{assetName, dst}})
}

type binaryInstall struct {
	assetName string
	dst       string
}

func downloadBinariesFromZip(env engineDownloadEnv, zipName string, installs []binaryInstall) error {
	zipPath := filepath.Join(env.cacheDir, zipName)
	if err := fetchEngineAsset(env, zipName, env.urlPrefix+zipName, zipPath); err != nil {
		return err
	}
	defer os.Remove(zipPath)

	extractDir, err := os.MkdirTemp(env.cacheDir, "engine-zip-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(extractDir)

	if err := extractZip(zipPath, extractDir); err != nil {
		return err
	}
	for _, install := range installs {
		info, err := os.Stat(filepath.Join(extractDir, install.assetName))
		if err != nil {
			return fmt.Errorf("engine archive %s is missing %s: %w", zipName, install.assetName, err)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("engine archive entry %s is not a regular file", install.assetName)
		}
	}
	for _, install := range installs {
		if err := copyEngineAssetAtomically(filepath.Join(extractDir, install.assetName), install.dst); err != nil {
			return err
		}
	}
	return nil
}

func fetchEngineAsset(env engineDownloadEnv, name, url, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), filepath.Base(dst)+".verified-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Remove(tmpPath); err != nil {
		return err
	}
	defer os.Remove(tmpPath)

	if env.assetDir == "" {
		if err := engineDownloadFetcher(url, tmpPath); err != nil {
			return err
		}
	} else {
		src, err := findLocalEngineAsset(env.assetDir, name)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stdout, "Installing local engine asset %s -> %s\n", src, dst)
		if err := copyEngineAssetAtomically(src, tmpPath); err != nil {
			return err
		}
	}

	if env.manifest != nil {
		if err := env.manifest.VerifyAsset(name, tmpPath); err != nil {
			return err
		}
	}
	return replaceDownloadedFile(tmpPath, dst)
}

func loadEngineAssetManifest(env *engineDownloadEnv) error {
	lock, err := release.RuntimeLockForVersion(env.version)
	if err != nil {
		return fmt.Errorf("resolve runtime lock for %s: %w", env.version, err)
	}
	manifestPath := filepath.Join(env.cacheDir, lock.Manifest)
	if env.assetDir == "" {
		if err := engineDownloadFetcher(env.urlPrefix+lock.Manifest, manifestPath); err != nil {
			return fmt.Errorf("download runtime manifest: %w", err)
		}
		defer os.Remove(manifestPath)
	} else {
		src, err := findLocalEngineAsset(env.assetDir, lock.Manifest)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) && env.allowMissingManifest {
				fmt.Fprintf(os.Stdout, "Runtime manifest is not present in same-run artifacts; final release assembly will verify them.\n")
				return nil
			}
			return err
		}
		manifestPath = src
	}

	manifest, err := release.LoadRuntimeManifest(manifestPath)
	if err != nil {
		return err
	}
	if err := manifest.ValidateForLock(lock); err != nil {
		return err
	}
	env.manifest = &manifest
	return nil
}

func findLocalEngineAsset(assetDir, name string) (string, error) {
	if filepath.Base(name) != name || name == "." || name == ".." {
		return "", fmt.Errorf("invalid engine asset name: %q", name)
	}

	direct := filepath.Join(assetDir, name)
	if info, err := os.Stat(direct); err == nil && info.Mode().IsRegular() {
		return direct, nil
	}

	var matches []string
	err := filepath.WalkDir(assetDir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type().IsRegular() && entry.Name() == name {
			matches = append(matches, path)
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("scan engine asset directory: %w", err)
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("engine asset %q not found under %s: %w", name, assetDir, fs.ErrNotExist)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("engine asset %q is ambiguous under %s: %v", name, assetDir, matches)
	}
}

func copyEngineAssetAtomically(src, dst string) (err error) {
	input, err := os.Open(src)
	if err != nil {
		return err
	}
	defer input.Close()
	info, err := input.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("engine asset is not a regular file: %s", src)
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	output, err := os.CreateTemp(filepath.Dir(dst), filepath.Base(dst)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := output.Name()
	defer func() {
		if output != nil {
			_ = output.Close()
		}
		if err != nil {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := io.Copy(output, input); err != nil {
		return err
	}
	if err := output.Chmod(info.Mode().Perm()); err != nil {
		return err
	}
	if err := output.Close(); err != nil {
		return err
	}
	output = nil
	return replaceDownloadedFile(tmpPath, dst)
}

func extractZip(srcZip, dstDir string) error {
	reader, err := zip.OpenReader(srcZip)
	if err != nil {
		return err
	}
	defer reader.Close()

	for _, file := range reader.File {
		targetPath, err := resolveZipExtractPath(dstDir, file.Name)
		if err != nil {
			return err
		}
		if file.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, file.Mode()); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		if err := extractZipFile(file, targetPath); err != nil {
			return err
		}
	}
	return nil
}

func resolveZipExtractPath(dstDir, name string) (string, error) {
	cleanBase := filepath.Clean(dstDir)
	targetPath := filepath.Clean(filepath.Join(cleanBase, name))
	basePrefix := cleanBase
	if !strings.HasSuffix(basePrefix, string(os.PathSeparator)) {
		basePrefix += string(os.PathSeparator)
	}
	targetPrefix := targetPath
	if !strings.HasSuffix(targetPrefix, string(os.PathSeparator)) {
		targetPrefix += string(os.PathSeparator)
	}
	if targetPath != cleanBase && !strings.HasPrefix(targetPrefix, basePrefix) {
		return "", fmt.Errorf("illegal path in archive entry: %s", name)
	}
	return targetPath, nil
}

func extractZipFile(file *zip.File, dst string) error {
	reader, err := file.Open()
	if err != nil {
		return err
	}
	defer reader.Close()

	output, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, file.Mode())
	if err != nil {
		return err
	}
	defer output.Close()

	_, err = io.Copy(output, reader)
	return err
}

func fetchURLToFile(url, dst string) (err error) {
	fmt.Fprintf(os.Stdout, "Downloading %s -> %s\n", url, dst)

	resp, err := engineDownloadHTTPClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s failed: %s", url, resp.Status)
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(dst), filepath.Base(dst)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := file.Name()
	defer func() {
		if file != nil {
			_ = file.Close()
		}
		if err != nil {
			_ = os.Remove(tmpPath)
		}
	}()

	if resp.ContentLength <= 0 {
		if _, err := io.Copy(file, resp.Body); err != nil {
			return err
		}
	} else {
		var downloaded int64
		lastReport := time.Now().Add(-time.Second)
		buffer := make([]byte, 128*1024)

		for {
			n, readErr := resp.Body.Read(buffer)
			if n > 0 {
				if _, err := file.Write(buffer[:n]); err != nil {
					return err
				}
				downloaded += int64(n)
				if time.Since(lastReport) >= 500*time.Millisecond || downloaded == resp.ContentLength {
					fmt.Fprintf(os.Stdout, "  %.1f%% (%s/%s)\r", float64(downloaded)*100/float64(resp.ContentLength), formatDownloadSize(downloaded), formatDownloadSize(resp.ContentLength))
					lastReport = time.Now()
				}
			}
			if readErr == io.EOF {
				break
			}
			if readErr != nil {
				return readErr
			}
		}

		fmt.Fprintf(os.Stdout, "  100.0%% (%s/%s)\n", formatDownloadSize(resp.ContentLength), formatDownloadSize(resp.ContentLength))
	}

	if err := file.Close(); err != nil {
		return err
	}
	file = nil

	return replaceDownloadedFile(tmpPath, dst)
}

func replaceDownloadedFile(src, dst string) error {
	return os.Rename(src, dst)
}

func linkOrCopyFile(src, dst string) error {
	if filepath.Clean(src) == filepath.Clean(dst) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := removeIfExists(dst); err != nil {
		return err
	}
	if err := os.Link(src, dst); err == nil {
		return nil
	} else if !errors.Is(err, fs.ErrExist) && !isLinkFallbackError(err) {
		return err
	}
	return shared.CopyFile(src, dst)
}

func removeIfExists(path string) error {
	if err := os.Remove(path); err == nil || errors.Is(err, fs.ErrNotExist) {
		return nil
	} else {
		return err
	}
}

func isLinkFallbackError(err error) bool {
	switch {
	case errors.Is(err, fs.ErrExist):
		return true
	case errors.Is(err, fs.ErrPermission):
		return true
	case errors.Is(err, syscall.EXDEV):
		return true
	case errors.Is(err, syscall.ENOTSUP):
		return true
	case errors.Is(err, syscall.EPERM):
		return true
	case errors.Is(err, syscall.EACCES):
		return true
	default:
		return false
	}
}

func formatDownloadSize(size int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)

	switch {
	case size >= gb:
		return fmt.Sprintf("%.1fGB", float64(size)/float64(gb))
	case size >= mb:
		return fmt.Sprintf("%.1fMB", float64(size)/float64(mb))
	case size >= kb:
		return fmt.Sprintf("%.1fKB", float64(size)/float64(kb))
	default:
		return fmt.Sprintf("%dB", size)
	}
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
