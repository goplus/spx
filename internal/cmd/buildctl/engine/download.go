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
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/goplus/spx/v2/internal/releasemeta"
)

type engineDownloadEnv struct {
	repoRoot    string
	version     string
	platform    string
	arch        string
	goBinDir    string
	templateDir string
	cacheDir    string
	urlPrefix   string
}

var EngineDownloadFetcher = fetchURLToFile
var EngineDownloadResolveEnv = resolveEngineDownloadEnv
var EngineDownloadHTTPClient = &http.Client{Timeout: 30 * time.Minute}

func downloadEngineAssets(cfg engineDownloadConfig, repoRoot string) error {
	env, err := EngineDownloadResolveEnv(repoRoot, cfg.platform)
	if err != nil {
		return err
	}

	if cfg.runtime {
		if err := downloadHostRuntimeAssets(env); err != nil {
			return err
		}
		if err := downloadRuntimePack(env); err != nil {
			fmt.Fprintf(osStderr, "warning: failed to download runtime pack: %v\n", err)
		}
		return nil
	}

	return downloadPlatformAssets(env, cfg.mode, false)
}

func resolveEngineDownloadEnv(repoRoot, platform string) (engineDownloadEnv, error) {
	buildEnv, err := resolveBuildEnvironment(repoRoot, platform)
	if err != nil {
		return engineDownloadEnv{}, err
	}

	env := engineDownloadEnv{
		repoRoot:    buildEnv.RepoRoot,
		version:     buildEnv.Version,
		platform:    buildEnv.Platform,
		arch:        buildEnv.Arch,
		goBinDir:    filepath.Join(buildEnv.GoPath, "bin"),
		templateDir: buildEnv.TemplateDir,
		cacheDir:    filepath.Join(repoRoot, "internal", "cmd", "buildctl", "bin"),
		urlPrefix:   fmt.Sprintf("https://github.com/goplus/godot/releases/download/spx%s/", buildEnv.Version),
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
	defaultPack := filepath.Join(env.goBinDir, "gdspxrt.pck")
	meta := releasemeta.CurrentReleaseMeta()
	zipName := fmt.Sprintf("gdspxrt.pck.%s.zip", meta.Pck.Version)
	url := meta.PckDownloadURL(zipName)
	zipPath := filepath.Join(env.cacheDir, zipName)
	if err := EngineDownloadFetcher(url, zipPath); err != nil {
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
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		src := filepath.Join(extractDir, entry.Name())
		dst := filepath.Join(env.goBinDir, entry.Name())
		if err := copyFile(src, dst); err != nil {
			return err
		}
	}

	if fileExists(defaultPack) {
		if err := replaceDownloadedFile(defaultPack, versionedPack); err != nil {
			return err
		}
	}
	return nil
}

func downloadAndroidAssets(env engineDownloadEnv) error {
	url := env.urlPrefix + "android.zip"
	zipPath := filepath.Join(env.cacheDir, "android.zip")
	if err := EngineDownloadFetcher(url, zipPath); err != nil {
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

	for _, name := range []string{"android_debug.apk", "android_release.apk", "android_source.zip"} {
		src := filepath.Join(extractDir, name)
		if fileExists(src) {
			if err := copyFile(src, filepath.Join(env.templateDir, name)); err != nil {
				return err
			}
		}
	}
	return nil
}

func downloadIOSAssets(env engineDownloadEnv) error {
	return EngineDownloadFetcher(env.urlPrefix+"ios.zip", filepath.Join(env.templateDir, "ios.zip"))
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
		if err := EngineDownloadFetcher(env.urlPrefix+templateName, cachedZip); err != nil {
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
	binaryName := fmt.Sprintf("godot.%s.template_release.%s%s", platformName, env.arch, postfix)
	templateBinary := filepath.Join(env.goBinDir, fmt.Sprintf("gdspxrt%s%s", env.version, postfix))
	if shouldDownloadPreparedAsset(templateBinary) {
		if err := downloadBinaryFromZip(env, zipName, binaryName, templateBinary); err != nil {
			return err
		}
	}

	switch env.platform {
	case "linux":
		for _, name := range []string{
			"linux_debug.arm32",
			"linux_debug.arm64",
			"linux_debug.x86_32",
			"linux_debug.x86_64",
			"linux_release.arm32",
			"linux_release.arm64",
			"linux_release.x86_32",
			"linux_release.x86_64",
		} {
			if err := linkOrCopyFile(templateBinary, filepath.Join(env.templateDir, name)); err != nil {
				return err
			}
		}
	case "windows":
		for _, name := range []string{
			"windows_debug_x86_32_console.exe",
			"windows_debug_x86_32.exe",
			"windows_debug_x86_64_console.exe",
			"windows_debug_x86_64.exe",
			"windows_release_x86_32_console.exe",
			"windows_release_x86_32.exe",
			"windows_release_x86_64_console.exe",
			"windows_release_x86_64.exe",
		} {
			if err := linkOrCopyFile(templateBinary, filepath.Join(env.templateDir, name)); err != nil {
				return err
			}
		}
	case "macos":
		macosZip := filepath.Join(env.templateDir, "macos.zip")
		if shouldDownloadPreparedAsset(macosZip) {
			if err := EngineDownloadFetcher(env.urlPrefix+"macos.zip", macosZip); err != nil {
				return err
			}
		}
	}

	return nil
}

func shouldDownloadPreparedAsset(path string) bool {
	return shouldRefreshPreparedAssets() || !fileExists(path)
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
	zipPath := filepath.Join(env.cacheDir, zipName)
	if err := EngineDownloadFetcher(env.urlPrefix+zipName, zipPath); err != nil {
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
	return copyFile(filepath.Join(extractDir, assetName), dst)
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

	resp, err := EngineDownloadHTTPClient.Get(url)
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
	if err := os.Rename(src, dst); err == nil {
		return nil
	} else if runtime.GOOS != "windows" || !fileExists(dst) {
		return err
	}
	if err := os.Remove(dst); err != nil {
		return err
	}
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
	return copyFile(src, dst)
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
	if err := validateWebMode(mode); err != nil {
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
	if err := validateWebMode(mode); err != nil {
		return "", err
	}
	if mode == "normal" {
		return fmt.Sprintf("gdspx%s_webpack.zip", version), nil
	}
	return fmt.Sprintf("gdspx%s_web%s.zip", version, mode), nil
}
