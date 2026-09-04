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

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/goplus/spx/v3/internal/cmd/buildctl/shared"
	"github.com/goplus/spx/v3/internal/zippreflight"
)

type toolSetupNDKConfig struct {
	manualInstall bool
	ndkPath       string
}

type androidNDKEnv struct {
	archiveName string
	archive     androidNDKArchive
	downloadURL string
	sdkRoot     string
	ndkRoot     string
	cacheDir    string
	shellConfig string
}

type androidNDKArchive struct {
	archiveOS string
	hostTag   string
	size      int64
	sha256    string
}

var androidNDKResolveEnv = resolveAndroidNDKEnv
var androidNDKFetcher = fetchAndroidNDK
var androidNDKPersister = persistNDKArchive
var androidNDKRetryDelay = 2 * time.Second

const (
	maxNDKPropertiesBytes int64 = 64 << 10
	maxNDKSymlinkBytes          = 4 << 10
	androidNDKOwnerFile         = ".spx-buildctl"
)

var errInvalidNDKArchive = errors.New("invalid Android NDK archive")

// Re-inventory these r23c limits when changing androidNDKVersion.
var androidNDKZipLimits = shared.ZipLimits{
	MaxArchiveBytes:          2 << 30,
	MaxCentralDirectoryBytes: 128 << 20,
	MaxEntries:               20_000,
	MaxEntrySize:             512 << 20,
	MaxTotalSize:             6 << 30,
	MaxCompressionRatio:      200,
}

func androidNDKArchiveForOS(goos string) (androidNDKArchive, error) {
	switch goos {
	case "darwin":
		return androidNDKArchive{
			archiveOS: "darwin", hostTag: "darwin-x86_64", size: 982917530,
			sha256: "baf793127741eda36f2eabe69cdec23a70c814deb3c75df8744af35aed21e59d",
		}, nil
	case "linux":
		return androidNDKArchive{
			archiveOS: "linux", hostTag: "linux-x86_64", size: 724733960,
			sha256: "6ce94604b77d28113ecd588d425363624a5228d9662450c48d2e4053f8039242",
		}, nil
	case "windows":
		return androidNDKArchive{
			archiveOS: "windows", hostTag: "windows-x86_64", size: 788336993,
			sha256: "48a0a7b38fb1c69cae6d17b70e03aab8d6138e65a30f8d3faebeb0dc09bf6940",
		}, nil
	default:
		return androidNDKArchive{}, fmt.Errorf("unsupported host OS for Android NDK setup: %s", goos)
	}
}

func fetchAndroidNDK(url, dst string) error {
	return shared.FetchURLToFileWithLimit(url, dst, androidNDKZipLimits.MaxArchiveBytes)
}

func parseToolSetupNDKArgs(args []string) (toolSetupNDKConfig, error) {
	cfg := toolSetupNDKConfig{}
	var deprecatedSkipVerification bool

	fs := flag.NewFlagSet("tool setup-ndk", flag.ContinueOnError)
	fs.SetOutput(osStderr)
	fs.BoolVar(&cfg.manualInstall, "manual-install", false, "skip download and use an existing NDK zip")
	fs.StringVar(&cfg.ndkPath, "ndk-path", "", "path to an existing Android NDK archive zip")
	fs.BoolVar(&deprecatedSkipVerification, "skip-verification", false, "deprecated; archive validation is always enforced")
	fs.Usage = func() {
		fmt.Fprintln(osStderr, "Usage: buildctl tool setup-ndk [--manual-install --ndk-path PATH]")
	}

	if err := fs.Parse(args); err != nil {
		return toolSetupNDKConfig{}, err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return toolSetupNDKConfig{}, errUsage
	}
	if deprecatedSkipVerification {
		return toolSetupNDKConfig{}, errors.New("--skip-verification is no longer supported; archive validation is required")
	}
	if cfg.manualInstall && cfg.ndkPath == "" {
		return toolSetupNDKConfig{}, errors.New("--manual-install requires --ndk-path")
	}
	if !cfg.manualInstall && cfg.ndkPath != "" {
		return toolSetupNDKConfig{}, errors.New("--ndk-path requires --manual-install")
	}
	return cfg, nil
}

func setupAndroidNDK(cfg toolSetupNDKConfig) error {
	env, err := androidNDKResolveEnv()
	if err != nil {
		return err
	}

	if _, err := os.Lstat(env.ndkRoot); err == nil {
		if verifyErr := verifyInstalledNDK(env.ndkRoot, env.archive); verifyErr == nil {
			if err := configureAndroidNDK(env); err != nil {
				return err
			}
			fmt.Fprintf(os.Stdout, "Android NDK %s is already installed at %s\n", androidNDKVersion, env.ndkRoot)
			return nil
		} else if !isBuildctlNDKInstallation(env.ndkRoot) {
			return fmt.Errorf("refusing to replace unmanaged Android NDK installation at %s: %w", env.ndkRoot, verifyErr)
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(env.ndkRoot), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(env.cacheDir, 0o755); err != nil {
		return err
	}

	tempDir, err := os.MkdirTemp(filepath.Dir(env.ndkRoot), "."+filepath.Base(env.ndkRoot)+".install-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	archivePath, err := resolveNDKArchivePath(cfg, env, tempDir)
	if err != nil {
		return err
	}
	extractDir := filepath.Join(tempDir, "extract")
	if err := os.MkdirAll(extractDir, 0o755); err != nil {
		return err
	}
	if err := shared.ExtractZipWithOptions(archivePath, extractDir, shared.ZipExtractOptions{
		Limits:                     androidNDKZipLimits,
		MaterializeSymlinksAsFiles: true,
	}); err != nil {
		return err
	}
	if err := restoreNDKSymlinks(archivePath, extractDir); err != nil {
		return err
	}

	ndkSource, err := locateExtractedNDK(extractDir)
	if err != nil {
		return err
	}
	if err := installNDK(ndkSource, env.ndkRoot, env.archive); err != nil {
		return err
	}

	if err := configureAndroidNDK(env); err != nil {
		return err
	}

	fmt.Fprintf(os.Stdout, "Android NDK %s installed at %s\n", androidNDKVersion, env.ndkRoot)
	if env.shellConfig != "" {
		fmt.Fprintf(os.Stdout, "Environment exports appended to %s\n", env.shellConfig)
	}
	return nil
}

func resolveAndroidNDKEnv() (androidNDKEnv, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return androidNDKEnv{}, err
	}
	androidNDKRelease, err := androidNDKReleaseForVersion(androidNDKVersion)
	if err != nil {
		return androidNDKEnv{}, err
	}

	sdkRoot := os.Getenv("ANDROID_SDK_ROOT")
	if sdkRoot == "" {
		switch runtime.GOOS {
		case "darwin":
			sdkRoot = filepath.Join(home, "Library", "Android", "sdk")
		case "linux":
			sdkRoot = filepath.Join(home, "Android", "Sdk")
		case "windows":
			localAppData := os.Getenv("LOCALAPPDATA")
			if localAppData == "" {
				localAppData = filepath.Join(home, "AppData", "Local")
			}
			sdkRoot = filepath.Join(localAppData, "Android", "Sdk")
		default:
			return androidNDKEnv{}, fmt.Errorf("unsupported host OS for Android NDK setup: %s", runtime.GOOS)
		}
	}

	archive, err := androidNDKArchiveForOS(runtime.GOOS)
	if err != nil {
		return androidNDKEnv{}, err
	}

	archiveName := fmt.Sprintf("android-ndk-%s-%s.zip", androidNDKRelease, archive.archiveOS)
	return androidNDKEnv{
		archiveName: archiveName,
		archive:     archive,
		downloadURL: "https://dl.google.com/android/repository/" + archiveName,
		sdkRoot:     sdkRoot,
		ndkRoot:     filepath.Join(sdkRoot, "ndk", androidNDKVersion),
		cacheDir:    filepath.Join(home, ".android_ndk_cache"),
		shellConfig: detectShellConfigFile(home),
	}, nil
}

func resolveNDKArchivePath(cfg toolSetupNDKConfig, env androidNDKEnv, tempDir string) (string, error) {
	if err := env.archive.validate(); err != nil {
		return "", err
	}
	snapshot := filepath.Join(tempDir, "android-ndk-source.zip")
	if cfg.manualInstall {
		return androidNDKPersister(cfg.ndkPath, snapshot, env.archive)
	}

	cachedArchive := filepath.Join(env.cacheDir, env.archiveName)
	if info, err := os.Lstat(cachedArchive); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return "", fmt.Errorf("NDK cache is not a regular non-symlink file: %s", cachedArchive)
		}
		path, err := androidNDKPersister(cachedArchive, snapshot, env.archive)
		if err == nil {
			return path, nil
		}
		if !errors.Is(err, errInvalidNDKArchive) {
			return "", err
		}
	} else if !os.IsNotExist(err) {
		return "", err
	}

	tempArchive := filepath.Join(tempDir, env.archiveName)
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if err := androidNDKFetcher(env.downloadURL, tempArchive); err != nil {
			lastErr = err
		} else if _, err := androidNDKPersister(tempArchive, cachedArchive, env.archive); err != nil {
			if !errors.Is(err, errInvalidNDKArchive) {
				return "", fmt.Errorf("persist Android NDK archive: %w", err)
			}
			lastErr = err
		} else {
			return tempArchive, nil
		}
		if attempt < 2 && androidNDKRetryDelay > 0 {
			time.Sleep(androidNDKRetryDelay)
		}
	}
	if lastErr == nil {
		lastErr = errors.New("failed to download Android NDK archive")
	}
	return "", lastErr
}

func persistNDKArchive(src, dst string, archive androidNDKArchive) (_ string, err error) {
	if err := archive.validate(); err != nil {
		return "", err
	}
	input, info, err := openNDKArchive(src)
	if err != nil {
		return "", err
	}
	defer input.Close()
	if info.Size() != archive.size {
		return "", invalidNDKArchivef("size %d does not match locked size %d", info.Size(), archive.size)
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", err
	}
	output, err := os.CreateTemp(filepath.Dir(dst), filepath.Base(dst)+".tmp-*")
	if err != nil {
		return "", err
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
	copied, err := io.Copy(output, io.LimitReader(input, archive.size+1))
	if err != nil {
		return "", err
	}
	if copied != archive.size {
		return "", invalidNDKArchivef("size %d does not match locked size %d", copied, archive.size)
	}
	if err := output.Sync(); err != nil {
		return "", err
	}
	if err := output.Close(); err != nil {
		return "", err
	}
	output = nil
	if err := verifyNDKArchive(tmpPath, archive); err != nil {
		return "", err
	}
	if err := shared.ReplaceFile(tmpPath, dst); err != nil {
		return "", err
	}
	return dst, nil
}

func verifyNDKArchive(path string, archive androidNDKArchive) error {
	if err := archive.validate(); err != nil {
		return err
	}
	file, info, err := openNDKArchive(path)
	if err != nil {
		return err
	}
	defer file.Close()
	if info.Size() != archive.size {
		return invalidNDKArchivef("size %d does not match locked size %d", info.Size(), archive.size)
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, io.NewSectionReader(file, 0, info.Size())); err != nil {
		return err
	}
	if digest := hex.EncodeToString(hash.Sum(nil)); digest != archive.sha256 {
		return invalidNDKArchivef("SHA-256 %s does not match locked digest %s", digest, archive.sha256)
	}
	if err := zippreflight.Check(file, info.Size(), zippreflight.Limits{
		MaxArchiveBytes:          androidNDKZipLimits.MaxArchiveBytes,
		MaxCentralDirectoryBytes: androidNDKZipLimits.MaxCentralDirectoryBytes,
		MaxEntries:               androidNDKZipLimits.MaxEntries,
	}); err != nil {
		return invalidNDKArchivef("preflight: %v", err)
	}
	root, err := expectedNDKRoot()
	if err != nil {
		return err
	}
	zipReader, err := zip.NewReader(file, info.Size())
	if err != nil {
		return invalidNDKArchivef("parse ZIP: %v", err)
	}
	entries := make(map[string]*zip.File, len(zipReader.File))
	for _, entry := range zipReader.File {
		if entry.Name != root+"/" && !strings.HasPrefix(entry.Name, root+"/") {
			return invalidNDKArchivef("entry %q is outside %q", entry.Name, root)
		}
		entries[entry.Name] = entry
	}
	tools, err := requiredNDKTools(archive.hostTag)
	if err != nil {
		return err
	}
	for _, name := range append(tools, requiredNDKFiles(archive.hostTag)...) {
		entry, ok := entries[root+"/"+name]
		if !ok || entry.UncompressedSize64 == 0 {
			return invalidNDKArchivef("missing required file %s", name)
		}
	}
	propertiesName := root + "/source.properties"
	properties, ok := entries[propertiesName]
	if !ok {
		return invalidNDKArchivef("missing %s", propertiesName)
	}
	if err := verifyNDKProperties(properties); err != nil {
		return invalidNDKArchivef("%v", err)
	}
	return nil
}

func openNDKArchive(path string) (*os.File, os.FileInfo, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, nil, err
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("NDK archive is not a regular non-symlink file: %s", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, nil, err
	}
	if !os.SameFile(before, info) {
		file.Close()
		return nil, nil, fmt.Errorf("NDK archive changed while opening: %s", path)
	}
	return file, info, nil
}

func invalidNDKArchivef(format string, args ...any) error {
	return fmt.Errorf("%w: %s", errInvalidNDKArchive, fmt.Sprintf(format, args...))
}

func (archive androidNDKArchive) validate() error {
	if archive.archiveOS == "" || archive.hostTag == "" || archive.size <= 0 {
		return fmt.Errorf("invalid Android NDK archive specification")
	}
	if !strings.HasPrefix(archive.hostTag, archive.archiveOS+"-") {
		return fmt.Errorf("Android NDK host tag %q does not match archive OS %q", archive.hostTag, archive.archiveOS)
	}
	digest, err := hex.DecodeString(archive.sha256)
	if err != nil || len(digest) != sha256.Size || archive.sha256 != strings.ToLower(archive.sha256) {
		return fmt.Errorf("invalid Android NDK archive SHA-256")
	}
	return nil
}

func requiredNDKTools(hostTag string) ([]string, error) {
	bin := "toolchains/llvm/prebuilt/" + hostTag + "/bin/"
	if strings.HasPrefix(hostTag, "windows-") {
		return []string{bin + "clang.exe", bin + "clang++.exe"}, nil
	}
	if strings.HasPrefix(hostTag, "darwin-") || strings.HasPrefix(hostTag, "linux-") {
		return []string{bin + "clang", bin + "clang++", bin + "clang-12"}, nil
	}
	return nil, fmt.Errorf("unsupported Android NDK host tag %q", hostTag)
}

func requiredNDKFiles(hostTag string) []string {
	prebuilt := "toolchains/llvm/prebuilt/" + hostTag + "/"
	return []string{
		"build/cmake/android.toolchain.cmake",
		"meta/abis.json",
		prebuilt + "sysroot/usr/include/android/api-level.h",
		prebuilt + "sysroot/usr/lib/aarch64-linux-android/libc++_shared.so",
	}
}

func expectedNDKRoot() (string, error) {
	release, err := androidNDKReleaseForVersion(androidNDKVersion)
	if err != nil {
		return "", err
	}
	return "android-ndk-" + release, nil
}

func verifyNDKProperties(entry *zip.File) error {
	if entry.FileInfo().Mode().Type() != 0 {
		return fmt.Errorf("NDK source.properties is not a regular file")
	}
	if entry.UncompressedSize64 > uint64(maxNDKPropertiesBytes) {
		return fmt.Errorf("NDK source.properties exceeds %d bytes", maxNDKPropertiesBytes)
	}
	reader, err := entry.Open()
	if err != nil {
		return err
	}
	data, readErr := io.ReadAll(io.LimitReader(reader, maxNDKPropertiesBytes+1))
	closeErr := reader.Close()
	if readErr != nil {
		return readErr
	}
	if closeErr != nil {
		return closeErr
	}
	if int64(len(data)) > maxNDKPropertiesBytes {
		return fmt.Errorf("NDK source.properties exceeds %d bytes", maxNDKPropertiesBytes)
	}
	return verifyNDKRevision(data)
}

func verifyNDKRevision(data []byte) error {
	revision := ""
	found := false
	for _, line := range strings.Split(string(data), "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok || strings.TrimSpace(key) != "Pkg.Revision" {
			continue
		}
		if found {
			return fmt.Errorf("NDK source.properties contains duplicate Pkg.Revision")
		}
		found = true
		revision = strings.TrimSpace(value)
	}
	if revision != androidNDKVersion {
		return fmt.Errorf("NDK revision %q does not match locked version %q", revision, androidNDKVersion)
	}
	return nil
}

func verifyInstalledNDK(root string, archive androidNDKArchive) error {
	if err := archive.validate(); err != nil {
		return err
	}
	info, err := os.Lstat(root)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return fmt.Errorf("Android NDK root is not a regular directory: %s", root)
	}
	properties, err := readSmallRegularFile(filepath.Join(root, "source.properties"), maxNDKPropertiesBytes)
	if err != nil {
		return err
	}
	if err := verifyNDKRevision(properties); err != nil {
		return err
	}
	canonicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	if err := verifyInstalledNDKTools(root, canonicalRoot, archive.hostTag); err != nil {
		return err
	}
	for _, name := range requiredNDKFiles(archive.hostTag) {
		if _, err := inspectInstalledNDKFile(root, canonicalRoot, name); err != nil {
			return err
		}
	}
	return nil
}

func verifyInstalledNDKTools(root, canonicalRoot, hostTag string) error {
	tools, err := requiredNDKTools(hostTag)
	if err != nil {
		return err
	}

	isWindows := strings.HasPrefix(hostTag, "windows-")
	compilerName := ""
	var compilerSize int64
	if !isWindows {
		compilerName = "toolchains/llvm/prebuilt/" + hostTag + "/bin/clang-12"
		compiler, err := inspectInstalledNDKFile(root, canonicalRoot, compilerName)
		if err != nil {
			return err
		}
		if compiler.Mode().Perm()&0o111 == 0 {
			return fmt.Errorf("Android NDK tool is not executable: %s", compilerName)
		}
		compilerSize = compiler.Size()
	}
	for _, name := range tools {
		if name == compilerName {
			continue
		}
		info, err := inspectInstalledNDKFile(root, canonicalRoot, name)
		if err != nil {
			return err
		}
		if isWindows {
			continue
		}
		if info.Mode().Perm()&0o111 == 0 {
			return fmt.Errorf("Android NDK tool is not executable: %s", name)
		}
		if info.Size() != compilerSize {
			return fmt.Errorf("Android NDK tool %s does not resolve to the compiler", name)
		}
	}
	return nil
}

func inspectInstalledNDKFile(root, canonicalRoot, name string) (os.FileInfo, error) {
	resolved, err := resolveNDKPath(canonicalRoot, filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		return nil, fmt.Errorf("resolve Android NDK file %s: %w", name, err)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return nil, fmt.Errorf("inspect Android NDK file %s: %w", name, err)
	}
	if !info.Mode().IsRegular() || info.Size() == 0 {
		return nil, fmt.Errorf("Android NDK file is not a non-empty regular file: %s", name)
	}
	return info, nil
}

func readSmallRegularFile(path string, maxBytes int64) ([]byte, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() || before.Size() > maxBytes {
		return nil, fmt.Errorf("not a bounded regular file: %s", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !os.SameFile(before, opened) {
		return nil, fmt.Errorf("file changed while opening: %s", path)
	}
	data, err := io.ReadAll(io.LimitReader(file, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("file exceeds %d bytes: %s", maxBytes, path)
	}
	return data, nil
}

func isBuildctlNDKInstallation(root string) bool {
	info, err := os.Lstat(root)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return false
	}
	data, err := readSmallRegularFile(filepath.Join(root, androidNDKOwnerFile), maxNDKPropertiesBytes)
	return err == nil && string(data) == androidNDKVersion+"\n"
}

func isPathWithinRoot(rel string) bool {
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func resolveNDKPath(canonicalRoot, path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(canonicalRoot, resolved)
	if err != nil || !isPathWithinRoot(rel) {
		return "", errors.New("path resolves outside the NDK root")
	}
	return resolved, nil
}

func locateExtractedNDK(extractDir string) (string, error) {
	root, err := expectedNDKRoot()
	if err != nil {
		return "", err
	}
	path := filepath.Join(extractDir, root)
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("find extracted Android NDK directory: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("extracted Android NDK path is not a directory: %s", path)
	}
	return path, nil
}

func restoreNDKSymlinks(archivePath, extractDir string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	type link struct {
		name   string
		target string
	}
	var links []link
	for _, entry := range reader.File {
		if entry.Mode()&fs.ModeType != fs.ModeSymlink {
			continue
		}
		input, err := entry.Open()
		if err != nil {
			return err
		}
		data, readErr := io.ReadAll(io.LimitReader(input, maxNDKSymlinkBytes+1))
		closeErr := input.Close()
		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		if len(data) == 0 || len(data) > maxNDKSymlinkBytes {
			return fmt.Errorf("invalid NDK symlink target for %s", entry.Name)
		}
		links = append(links, link{name: entry.Name, target: string(data)})
	}
	for _, link := range links {
		path := filepath.Join(extractDir, filepath.FromSlash(link.name))
		if err := os.Remove(path); err != nil {
			return err
		}
		if err := os.Symlink(filepath.FromSlash(link.target), path); err != nil {
			return err
		}
	}
	root, err := expectedNDKRoot()
	if err != nil {
		return err
	}
	canonicalRoot, err := filepath.EvalSymlinks(filepath.Join(extractDir, root))
	if err != nil {
		return err
	}
	for _, link := range links {
		path := filepath.Join(extractDir, filepath.FromSlash(link.name))
		resolved, err := resolveNDKPath(canonicalRoot, path)
		if err != nil {
			return fmt.Errorf("resolve NDK symlink %s: %w", link.name, err)
		}
		info, err := os.Stat(resolved)
		if err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("NDK symlink target is not a regular file: %s", link.name)
		}
	}
	return nil
}

func installNDK(src, dst string, archive androidNDKArchive) error {
	if err := verifyInstalledNDK(src, archive); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(src, androidNDKOwnerFile), []byte(androidNDKVersion+"\n"), 0o644); err != nil {
		return err
	}
	return replaceNDKInstallation(src, dst, archive)
}

func replaceNDKInstallation(stage, dst string, archive androidNDKArchive) error {
	if _, err := os.Lstat(dst); os.IsNotExist(err) {
		return os.Rename(stage, dst)
	} else if err != nil {
		return err
	}
	if err := verifyInstalledNDK(dst, archive); err == nil {
		return nil
	}
	if !isBuildctlNDKInstallation(dst) {
		return fmt.Errorf("refusing to replace unmanaged Android NDK installation at %s", dst)
	}

	backupDir, err := os.MkdirTemp(filepath.Dir(dst), "."+filepath.Base(dst)+".backup-*")
	if err != nil {
		return err
	}
	previous := filepath.Join(backupDir, "previous")
	if err := os.Rename(dst, previous); err != nil {
		_ = os.Remove(backupDir)
		return err
	}
	if err := os.Rename(stage, dst); err != nil {
		restoreErr := os.Rename(previous, dst)
		_ = os.Remove(backupDir)
		if restoreErr != nil {
			return errors.Join(err, fmt.Errorf("restore previous Android NDK installation: %w", restoreErr))
		}
		return err
	}
	if err := os.RemoveAll(backupDir); err != nil {
		return fmt.Errorf("remove previous Android NDK installation: %w", err)
	}
	return nil
}

func configureAndroidNDK(env androidNDKEnv) error {
	if err := updateNDKShellConfig(env); err != nil {
		return err
	}
	if err := os.Setenv("ANDROID_SDK_ROOT", env.sdkRoot); err != nil {
		return err
	}
	return os.Setenv("ANDROID_NDK_ROOT", env.ndkRoot)
}

func updateNDKShellConfig(env androidNDKEnv) error {
	if env.shellConfig == "" {
		return nil
	}

	exports := fmt.Sprintf("export ANDROID_SDK_ROOT=%s\nexport ANDROID_NDK_ROOT=%s\nexport PATH=\"$ANDROID_NDK_ROOT:$PATH\"\n", shared.ShellQuote(env.sdkRoot), shared.ShellQuote(env.ndkRoot))
	current, _ := os.ReadFile(env.shellConfig)
	if strings.Contains(string(current), exports) {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(env.shellConfig), 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(env.shellConfig, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString("\n# Android NDK (buildctl)\n" + exports)
	return err
}

func detectShellConfigFile(home string) string {
	shell := filepath.Base(os.Getenv("SHELL"))
	switch shell {
	case "zsh":
		return filepath.Join(home, ".zshrc")
	case "bash":
		bashProfile := filepath.Join(home, ".bash_profile")
		if shared.FileExists(bashProfile) {
			return bashProfile
		}
		return filepath.Join(home, ".bashrc")
	default:
		return filepath.Join(home, ".bash_profile")
	}
}
