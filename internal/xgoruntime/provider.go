/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
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

package xgoruntime

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/goplus/spx/v3/internal/interpruntime"
	"github.com/goplus/spx/v3/internal/processsupervisor"
	"github.com/goplus/spx/v3/internal/projectbundle"
	"github.com/goplus/spx/v3/internal/projectpolicy"
	"github.com/goplus/spx/v3/internal/release"
	"github.com/goplus/spx/v3/internal/runtimebundle"
	"github.com/goplus/spx/v3/internal/runtimepayload"
	"github.com/goplus/spx/v3/x/xgolauncher"
)

const activeEnvironment = "SPX_XGO_RUNTIME_ACTIVE"

// IO contains the inherited provider streams and environment. A nil stream is
// passed through to the Engine/Go command in the same way as os/exec.
type IO struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	Env    []string
}

// Execute performs one already-parsed provider request without exiting the
// process. Command entry points translate the returned status through
// xgolauncher.Exit after all cleanup has completed.
func Execute(ctx context.Context, cfg Config, streams IO) (xgolauncher.ProcessStatus, error) {
	if ctx == nil {
		return xgolauncher.ProcessStatus{}, errors.New("xgoruntime: nil context")
	}
	if err := validateSPXRequest(cfg); err != nil {
		return xgolauncher.ProcessStatus{}, err
	}
	if err := cfg.validateGraphInputs(); err != nil {
		return xgolauncher.ProcessStatus{}, err
	}
	if hasEnvironment(streams.Env, activeEnvironment) {
		return xgolauncher.ProcessStatus{}, errors.New("xgoruntime: recursive runtime-provider invocation rejected")
	}
	configSnapshot, err := projectpolicy.SnapshotPortableConfig(cfg.ProjectDir)
	if err != nil {
		return xgolauncher.ProcessStatus{}, fmt.Errorf("xgoruntime: %w", err)
	}
	if err := VerifyDeclaration(cfg.Declaration); err != nil {
		return xgolauncher.ProcessStatus{}, err
	}
	if err := verifyProviderPackage(ctx, cfg, streams.Env); err != nil {
		return xgolauncher.ProcessStatus{}, err
	}
	if !sourceMode(cfg.ProviderOrigin) {
		return xgolauncher.ProcessStatus{}, fmt.Errorf("xgoruntime: published bridge mode is unavailable for %s@%s; use a main/workspace module or local replace until immutable bridge manifests are published", cfg.ProviderOrigin.Selected.Path, cfg.ProviderOrigin.Selected.Version)
	}
	assets, err := resolveLocalAssets(ctx, cfg, streams)
	if err != nil {
		return xgolauncher.ProcessStatus{}, err
	}
	if assets.Cleanup != nil {
		defer assets.Cleanup()
	}
	switch cfg.Action {
	case ActionRun:
		return runProject(ctx, cfg, assets, configSnapshot, streams)
	case ActionBuild:
		if err := buildLauncher(ctx, cfg, assets, configSnapshot, streams); err != nil {
			return xgolauncher.ProcessStatus{}, err
		}
		return xgolauncher.ProcessStatus{}, nil
	default:
		return xgolauncher.ProcessStatus{}, fmt.Errorf("xgoruntime: unsupported action %q", cfg.Action)
	}
}

// validateSPXRequest applies provider-domain requirements after the generic
// XGo protocol has been parsed. Pack metadata is optional on the wire because
// other runtime providers may not use it; SPX requires it to define AssetDir.
func validateSPXRequest(cfg Config) error {
	if cfg.Project.PackDirectory == "" || cfg.Project.PackIndexFile == "" {
		return errors.New("xgoruntime: SPX provider requires pack metadata")
	}
	if cfg.Project.PackDirectory == "." {
		return errors.New("xgoruntime: SPX provider requires a dedicated pack directory below the project root")
	}
	return nil
}

type localAssets struct {
	EnginePath string
	PackPath   string
	BridgePath string
	Lock       release.RuntimeLock
	Cleanup    func()
}

func sourceMode(origin ModuleOrigin) bool {
	if origin.Main {
		return true
	}
	if origin.Replace != nil {
		// A replacement with no version is a filesystem source identity. A
		// versioned replacement is still a released module identity and must not
		// borrow arbitrary installed local assets.
		return origin.Replace.Version == ""
	}
	return origin.Selected.Version == ""
}

func resolveLocalAssets(ctx context.Context, cfg Config, streams IO) (localAssets, error) {
	return resolveLocalAssetsWith(ctx, cfg, streams, defaultRuntimeAssetDependencies())
}

func resolveLocalAssetsWith(ctx context.Context, cfg Config, streams IO, dependencies runtimeAssetDependencies) (localAssets, error) {
	lock := release.DefaultRuntimeLock()
	bridgeName, err := bridgeFileName(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return localAssets{}, err
	}
	assets, err := acquireRuntimeAssetsWith(ctx, cfg, streams, lock, dependencies)
	if err != nil {
		return localAssets{}, err
	}
	bridgePath, cleanup, err := buildSourceBridge(ctx, cfg, bridgeName, streams)
	if err != nil {
		if assets.Cleanup != nil {
			assets.Cleanup()
		}
		return localAssets{}, err
	}
	assets.BridgePath = bridgePath
	runtimeCleanup := assets.Cleanup
	assets.Cleanup = func() {
		cleanup()
		if runtimeCleanup != nil {
			runtimeCleanup()
		}
	}
	return assets, nil
}

func buildSourceBridge(ctx context.Context, cfg Config, bridgeName string, streams IO) (string, func(), error) {
	if err := cfg.validateGraphInputs(); err != nil {
		return "", nil, err
	}
	workDir, err := os.MkdirTemp("", "spx-provider-bridge-")
	if err != nil {
		return "", nil, fmt.Errorf("xgoruntime: create source bridge work directory: %w", err)
	}
	keepWork := hasBuildFlag(cfg.BuildFlags, "work")
	cleanup := func() { _ = os.RemoveAll(workDir) }
	if keepWork {
		cleanup = func() {}
		if streams.Stderr != nil {
			_, _ = fmt.Fprintf(streams.Stderr, "SPXBRIDGEWORK=%s\n", workDir)
		}
	}
	bridgePath := filepath.Join(workDir, bridgeName)
	args := sourceBridgeBuildArgs(cfg, bridgePath)
	command := exec.CommandContext(ctx, cfg.GoCommand, args...)
	command.Dir = cfg.GraphWorkDir
	command.Env = sourceBridgeEnv(cfg, streams.Env)
	command.Stdin = streams.Stdin
	command.Stdout = streams.Stdout
	command.Stderr = streams.Stderr
	if err := command.Run(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("xgoruntime: build source interpreter bridge: %w", err)
	}
	if err := validatePinnedFile("source interpreter bridge", bridgePath); err != nil {
		cleanup()
		return "", nil, err
	}
	if err := verifyBuiltProviderOrigin(ctx, bridgePath, cfg.ProviderOrigin, cfg, streams.Env); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("xgoruntime: verify source interpreter bridge provenance: %w", err)
	}
	return bridgePath, cleanup, nil
}

func sourceBridgeBuildArgs(cfg Config, bridgePath string) []string {
	return sourceBridgeBuildArgsForGOOS(cfg, bridgePath, runtime.GOOS)
}

func sourceBridgeBuildArgsForGOOS(cfg Config, bridgePath, goos string) []string {
	args := append([]string{"build"}, cfg.GraphFlags...)
	args = append(args, normalizedGoBuildFlags(cfg.BuildFlags)...)
	args = append(args, "-buildmode=c-shared")
	if goos == "windows" {
		args = append(args, "-ldflags=-extldflags=-Wl,--allow-multiple-definition")
	}
	return append(args, "-o", bridgePath, cfg.ProviderOrigin.Selected.Path+"/cmd/ispxnative")
}

func bridgeFileName(goos, goarch string) (string, error) {
	extension := ""
	switch goos {
	case "darwin":
		extension = ".dylib"
	case "linux":
		extension = ".so"
	case "windows":
		extension = ".dll"
	default:
		return "", fmt.Errorf("xgoruntime: host platform %s/%s is not supported", goos, goarch)
	}
	return "gdspx-" + goos + "-" + goarch + extension, nil
}

func runProject(ctx context.Context, cfg Config, assets localAssets, configSnapshot projectpolicy.PortableConfigSnapshot, streams IO) (xgolauncher.ProcessStatus, error) {
	sessionDir, err := os.MkdirTemp("", "spx-provider-run-")
	if err != nil {
		return xgolauncher.ProcessStatus{}, fmt.Errorf("xgoruntime: create run session: %w", err)
	}
	keepWork := hasBuildFlag(cfg.BuildFlags, "work")
	if keepWork {
		if streams.Stderr != nil {
			_, _ = fmt.Fprintf(streams.Stderr, "SPXWORK=%s\n", sessionDir)
		}
	} else {
		defer os.RemoveAll(sessionDir)
	}
	configDir, configIdentity, err := materializePortableConfigSnapshot(sessionDir, cfg.ProjectDir, configSnapshot)
	if err != nil {
		return xgolauncher.ProcessStatus{}, err
	}
	roots := interpruntime.Roots{
		ProjectDir: cfg.ProjectDir,
		AssetDir:   filepath.Join(cfg.ProjectDir, filepath.FromSlash(cfg.Project.PackDirectory)),
		SessionDir: sessionDir,
	}
	if err := interpruntime.PrepareSession(interpruntime.SessionConfig{Roots: roots, BridgePath: assets.BridgePath}); err != nil {
		return xgolauncher.ProcessStatus{}, fmt.Errorf("xgoruntime: prepare interpreted session: %w", err)
	}
	command, err := interpruntime.PrepareCommand(ctx, interpruntime.CommandConfig{
		Roots: roots, Executable: assets.EnginePath, Args: cfg.ApplicationArgs,
		Env:   append(sanitizeEnvironment(streams.Env), activeEnvironment+"=1"),
		Stdin: streams.Stdin, Stdout: streams.Stdout, Stderr: streams.Stderr,
		PathPolicy: interpruntime.RejectPath,
	})
	if err != nil {
		return xgolauncher.ProcessStatus{}, fmt.Errorf("xgoruntime: prepare Engine: %w", err)
	}
	command.Env = append(command.Env,
		interpruntime.PortableConfigDirEnv+"="+configDir,
		interpruntime.PortableConfigIdentityEnv+"="+configIdentity,
	)
	status, err := processsupervisor.Run(ctx, command)
	if err != nil {
		return xgolauncher.ProcessStatus{}, fmt.Errorf("xgoruntime: run Engine: %w", err)
	}
	return xgolauncher.ProcessStatus{Code: status.Code, Signal: status.Signal}, nil
}

func materializePortableConfigSnapshot(sessionDir, projectDir string, snapshot projectpolicy.PortableConfigSnapshot) (string, string, error) {
	if err := snapshot.Verify(projectDir); err != nil {
		return "", "", fmt.Errorf("xgoruntime: %w", err)
	}
	identity, err := snapshot.Identity()
	if err != nil {
		return "", "", fmt.Errorf("xgoruntime: identify portable config: %w", err)
	}
	configDir := filepath.Join(sessionDir, "portable-config")
	if err := os.Mkdir(configDir, 0o700); err != nil {
		return "", "", fmt.Errorf("xgoruntime: create portable config directory: %w", err)
	}
	if snapshot.Present() {
		configPath := filepath.Join(configDir, ".config")
		file, err := os.OpenFile(configPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err != nil {
			return "", "", fmt.Errorf("xgoruntime: create portable config: %w", err)
		}
		data := snapshot.Bytes()
		written, writeErr := file.Write(data)
		if writeErr == nil && written != len(data) {
			writeErr = io.ErrShortWrite
		}
		closeErr := file.Close()
		if writeErr != nil {
			return "", "", fmt.Errorf("xgoruntime: write portable config: %w", writeErr)
		}
		if closeErr != nil {
			return "", "", fmt.Errorf("xgoruntime: close portable config: %w", closeErr)
		}
	}
	return configDir, identity, nil
}

func buildLauncher(ctx context.Context, cfg Config, assets localAssets, configSnapshot projectpolicy.PortableConfigSnapshot, streams IO) error {
	if err := verifyLauncherPackage(ctx, cfg, streams.Env); err != nil {
		return err
	}
	projectConfig, err := prepareProjectBundleConfig(cfg, configSnapshot)
	if err != nil {
		return err
	}
	return compileLauncher(ctx, cfg, streams, func(workDir string, dst io.Writer) (string, string, error) {
		return writeLauncherPayload(workDir, dst, cfg, assets, projectConfig, streams)
	})
}

func prepareProjectBundleConfig(cfg Config, configSnapshot projectpolicy.PortableConfigSnapshot) (projectbundle.Config, error) {
	projectFiles, err := collectProjectAllowlist(cfg)
	if err != nil {
		return projectbundle.Config{}, err
	}
	if err := configSnapshot.Verify(cfg.ProjectDir); err != nil {
		return projectbundle.Config{}, fmt.Errorf("xgoruntime: %w", err)
	}
	return projectbundle.Config{
		ProjectDir: cfg.ProjectDir, ProjectFiles: projectFiles, IncludeConfig: configSnapshot.Present(),
		ConfigBytes: configSnapshot.Bytes(),
		PackDir:     cfg.Project.PackDirectory, Output: cfg.Output, FinalOutput: cfg.FinalOutput,
	}, nil
}

func writeLauncherPayload(workDir string, dst io.Writer, cfg Config, assets localAssets, projectConfig projectbundle.Config, streams IO) (payloadDigest, manifestDigest string, err error) {
	projectPath := filepath.Join(workDir, "project.zip")
	projectFile, err := os.OpenFile(projectPath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", "", fmt.Errorf("xgoruntime: create project archive: %w", err)
	}
	defer func() {
		if closeErr := projectFile.Close(); err == nil && closeErr != nil {
			payloadDigest, manifestDigest, err = "", "", fmt.Errorf("xgoruntime: close project archive: %w", closeErr)
		}
	}()
	projectDigest, err := projectbundle.WriteArchive(projectFile, projectConfig)
	if err != nil {
		return "", "", fmt.Errorf("xgoruntime: collect project: %w", err)
	}
	if err := projectFile.Sync(); err != nil {
		return "", "", fmt.Errorf("xgoruntime: sync project archive: %w", err)
	}
	projectInfo, err := projectFile.Stat()
	if err != nil {
		return "", "", fmt.Errorf("xgoruntime: stat project archive: %w", err)
	}
	projectBundle, err := runtimepayload.ComponentBundleReaderAt(projectFile, projectInfo.Size(), runtimebundle.NamespaceProject)
	if err != nil {
		return "", "", fmt.Errorf("xgoruntime: verify generated project bundle: %w", err)
	}

	engine, err := openPinnedFile("Engine", assets.EnginePath)
	if err != nil {
		return "", "", err
	}
	defer func() {
		if closeErr := engine.file.Close(); err == nil && closeErr != nil {
			payloadDigest, manifestDigest, err = "", "", fmt.Errorf("xgoruntime: close Engine: %w", closeErr)
		}
	}()
	pack, err := openPinnedFile("runtime PCK", assets.PackPath)
	if err != nil {
		return "", "", err
	}
	defer func() {
		if closeErr := pack.file.Close(); err == nil && closeErr != nil {
			payloadDigest, manifestDigest, err = "", "", fmt.Errorf("xgoruntime: close runtime PCK: %w", closeErr)
		}
	}()
	bridge, err := openPinnedFile("interpreter bridge", assets.BridgePath)
	if err != nil {
		return "", "", err
	}
	defer func() {
		if closeErr := bridge.file.Close(); err == nil && closeErr != nil {
			payloadDigest, manifestDigest, err = "", "", fmt.Errorf("xgoruntime: close interpreter bridge: %w", closeErr)
		}
	}()

	interfaceDigest, engineDigest, packDigest, err := localEngineSourceDigests(engine.source(""), pack.source(""))
	if err != nil {
		return "", "", err
	}
	bridgeDigest, err := digestFileSource(bridge.source(""))
	if err != nil {
		return "", "", fmt.Errorf("xgoruntime: hash interpreter bridge: %w", err)
	}
	engineManifest, err := json.Marshal(struct {
		Schema                string `json:"schema"`
		Mode                  string `json:"mode"`
		RuntimeVersion        string `json:"runtime_version"`
		RuntimeABI            int    `json:"runtime_abi"`
		EngineInterfaceDigest string `json:"engine_interface_digest"`
		ExecutableSHA256      string `json:"executable_sha256"`
		PackSHA256            string `json:"pack_sha256"`
	}{
		Schema: "spx-local-engine/v1", Mode: "source", RuntimeVersion: assets.Lock.RuntimeVersion,
		RuntimeABI: assets.Lock.RuntimeABI, EngineInterfaceDigest: interfaceDigest,
		ExecutableSHA256: engineDigest, PackSHA256: packDigest,
	})
	if err != nil {
		return "", "", err
	}
	bridgeManifest, err := json.Marshal(struct {
		Schema                string `json:"schema"`
		Mode                  string `json:"mode"`
		SPXSource             string `json:"spx_source"`
		EngineInterfaceDigest string `json:"engine_interface_digest"`
		BridgeSHA256          string `json:"bridge_sha256"`
	}{
		Schema: "spx-local-bridge/v1", Mode: "source", SPXSource: cfg.ProviderOrigin.Effective().Path,
		EngineInterfaceDigest: interfaceDigest, BridgeSHA256: bridgeDigest,
	})
	if err != nil {
		return "", "", err
	}

	engineName := filepath.Base(assets.EnginePath)
	packName := filepath.Base(assets.PackPath)
	bridgeName := filepath.Base(assets.BridgePath)
	engineSources := []runtimepayload.FileSource{
		byteSource("runtime-manifest.json", 0o644, engineManifest),
		engine.source(engineName),
		pack.source(packName),
	}
	engineBundle, err := runtimepayload.ComponentBundleSources(engineSources, runtimebundle.NamespaceEngine)
	if err != nil {
		return "", "", fmt.Errorf("xgoruntime: identify Engine bundle: %w", err)
	}
	if !bundleEntryHasDigest(engineBundle, engineName, engineDigest) || !bundleEntryHasDigest(engineBundle, packName, packDigest) {
		return "", "", errors.New("xgoruntime: Engine or runtime PCK changed while identifying payload")
	}
	bridgeSources := []runtimepayload.FileSource{
		byteSource("bridge-manifest.json", 0o644, bridgeManifest),
		bridge.source(bridgeName),
	}
	bridgeBundle, err := runtimepayload.ComponentBundleSources(bridgeSources, runtimebundle.NamespaceBridge)
	if err != nil {
		return "", "", fmt.Errorf("xgoruntime: identify bridge bundle: %w", err)
	}
	if !bundleEntryHasDigest(bridgeBundle, bridgeName, bridgeDigest) {
		return "", "", errors.New("xgoruntime: interpreter bridge changed while identifying payload")
	}

	payloadSources := []runtimepayload.FileSource{
		byteSource("engine/runtime-manifest.json", 0o644, engineManifest),
		engine.source("engine/" + engineName),
		pack.source("engine/" + packName),
		byteSource("bridge/bridge-manifest.json", 0o644, bridgeManifest),
		bridge.source("bridge/" + bridgeName),
		{Name: runtimepayload.ProjectZipPath, Mode: 0o644, ReaderAt: projectFile, Size: projectInfo.Size()},
	}
	payloadDigest, manifestDigest, err = runtimepayload.BuildTo(dst, runtimepayload.BuildConfig{
		SPX: runtimepayload.SourceIdentity{
			SelectedPath: cfg.ProviderOrigin.Selected.Path, SelectedVersion: cfg.ProviderOrigin.Selected.Version,
			EffectivePath: cfg.ProviderOrigin.Effective().Path, EffectiveVersion: cfg.ProviderOrigin.Effective().Version,
			Main: cfg.ProviderOrigin.Main, SourceMode: true,
		},
		Target: runtimepayload.Target{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH},
		Engine: runtimepayload.Engine{
			RuntimeVersion: assets.Lock.RuntimeVersion, RuntimeABI: assets.Lock.RuntimeABI,
			EngineInterfaceDigest: interfaceDigest, Executable: engineName, Pack: packName,
			BundleDigest: engineBundle.Digest,
		},
		Bridge: runtimepayload.Bridge{File: bridgeName, BundleDigest: bridgeBundle.Digest},
		Project: runtimepayload.Project{
			PackDirectory: cfg.Project.PackDirectory, BundleDigest: projectBundle.Digest,
			ArchiveSHA256: projectDigest.String(),
		},
	}, payloadSources)
	if err != nil {
		return "", "", fmt.Errorf("xgoruntime: build embedded payload: %w", err)
	}
	for _, source := range []*pinnedFile{engine, pack, bridge} {
		if err := source.verify(); err != nil {
			return "", "", err
		}
	}
	if traceEnabled(cfg.BuildFlags) && streams.Stderr != nil {
		_, _ = fmt.Fprintf(streams.Stderr, "xgoruntime: project=%s payload=%s engine=%s bridge=%s\n", projectDigest, payloadDigest, engineBundle.Digest, bridgeBundle.Digest)
	}
	return payloadDigest, manifestDigest, nil
}

type pinnedFile struct {
	name string
	path string
	file *os.File
	info os.FileInfo
}

func openPinnedFile(name, filePath string) (*pinnedFile, error) {
	before, err := os.Lstat(filePath)
	if err != nil {
		return nil, fmt.Errorf("lstat %s %q: %w", name, filePath, err)
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() {
		return nil, fmt.Errorf("%s %q is not a regular non-symlink file", name, filePath)
	}
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("open %s %q: %w", name, filePath, err)
	}
	opened, statErr := file.Stat()
	if statErr != nil {
		_ = file.Close()
		return nil, fmt.Errorf("stat %s %q: %w", name, filePath, statErr)
	}
	after, err := os.Lstat(filePath)
	if err != nil || after.Mode()&os.ModeSymlink != 0 || !after.Mode().IsRegular() || !os.SameFile(before, opened) || !os.SameFile(opened, after) || before.Size() != opened.Size() || opened.Size() != after.Size() {
		_ = file.Close()
		return nil, fmt.Errorf("%s %q changed while opening", name, filePath)
	}
	return &pinnedFile{name: name, path: filePath, file: file, info: opened}, nil
}

func validatePinnedFile(name, filePath string) error {
	file, err := openPinnedFile(name, filePath)
	if err != nil {
		return err
	}
	if err := file.file.Close(); err != nil {
		return fmt.Errorf("close %s %q: %w", name, filePath, err)
	}
	return nil
}

func (f *pinnedFile) source(name string) runtimepayload.FileSource {
	return runtimepayload.FileSource{Name: name, Mode: f.info.Mode().Perm(), ReaderAt: f.file, Size: f.info.Size()}
}

func (f *pinnedFile) verify() error {
	opened, err := f.file.Stat()
	if err != nil {
		return fmt.Errorf("stat %s %q: %w", f.name, f.path, err)
	}
	after, err := os.Lstat(f.path)
	if err != nil || after.Mode()&os.ModeSymlink != 0 || !after.Mode().IsRegular() || !os.SameFile(f.info, opened) || !os.SameFile(opened, after) || opened.Size() != f.info.Size() || after.Size() != f.info.Size() {
		return fmt.Errorf("%s %q changed while reading", f.name, f.path)
	}
	return nil
}

func byteSource(name string, mode os.FileMode, data []byte) runtimepayload.FileSource {
	return runtimepayload.FileSource{Name: name, Mode: mode, ReaderAt: bytes.NewReader(data), Size: int64(len(data))}
}

func digestFileSource(source runtimepayload.FileSource) (string, error) {
	hasher := sha256.New()
	if err := copyFileSource(hasher, source); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// localEngineSourceDigests computes the stable source-mode compatibility
// identity for an installed Engine/PCK pair together with their file digests.
// Published mode must use the release manifest's canonical interface digest;
// including bridge bytes here would give the same Engine a different cache
// identity for every source bridge rebuild.
func localEngineSourceDigests(engine, pack runtimepayload.FileSource) (interfaceDigest, engineDigest, packDigest string, err error) {
	interfaceHasher := sha256.New()
	engineHasher := sha256.New()
	packHasher := sha256.New()
	_, _ = interfaceHasher.Write([]byte("spx-local-engine-interface/v1\x00"))
	if err := copyFileSource(io.MultiWriter(interfaceHasher, engineHasher), engine); err != nil {
		return "", "", "", fmt.Errorf("xgoruntime: hash Engine: %w", err)
	}
	_, _ = interfaceHasher.Write([]byte{0})
	if err := copyFileSource(io.MultiWriter(interfaceHasher, packHasher), pack); err != nil {
		return "", "", "", fmt.Errorf("xgoruntime: hash runtime PCK: %w", err)
	}
	return hex.EncodeToString(interfaceHasher.Sum(nil)), hex.EncodeToString(engineHasher.Sum(nil)), hex.EncodeToString(packHasher.Sum(nil)), nil
}

func copyFileSource(dst io.Writer, source runtimepayload.FileSource) error {
	count, err := io.Copy(dst, io.NewSectionReader(source.ReaderAt, 0, source.Size))
	if err != nil {
		return err
	}
	if count != source.Size {
		return fmt.Errorf("short read: read %d bytes, want %d: %w", count, source.Size, io.ErrUnexpectedEOF)
	}
	return nil
}

func bundleEntryHasDigest(bundle runtimebundle.Bundle, name, digest string) bool {
	for _, entry := range bundle.Entries {
		if entry.Name == name {
			return entry.SHA256 == digest
		}
	}
	return false
}

func hasBuildFlag(flags []string, name string) bool {
	needle := "-" + name + "=true"
	for _, flag := range flags {
		if flag == needle {
			return true
		}
	}
	return false
}

func traceEnabled(flags []string) bool { return hasBuildFlag(flags, "x") || hasBuildFlag(flags, "v") }

func hasEnvironment(env []string, key string) bool {
	for _, entry := range env {
		name, value, ok := strings.Cut(entry, "=")
		if ok && name == key && value != "" {
			return true
		}
	}
	return false
}

func sanitizeEnvironment(env []string) []string {
	result := make([]string, 0, len(env))
	for _, entry := range env {
		key, _, ok := strings.Cut(entry, "=")
		if ok && (key == activeEnvironment || key == "GOFLAGS" || key == "GOWORK" || key == "GOOS" || key == "GOARCH" || key == "CGO_ENABLED") {
			continue
		}
		result = append(result, entry)
	}
	return result
}

func hostGoEnv(cfg Config, base []string) []string {
	env := sanitizeEnvironment(base)
	return append(env,
		"GOFLAGS=", "GOWORK="+cfg.GoWork, "GOOS="+runtime.GOOS, "GOARCH="+runtime.GOARCH, "CGO_ENABLED=0",
	)
}

func sourceBridgeEnv(cfg Config, base []string) []string {
	env := sanitizeEnvironment(base)
	filtered := env[:0]
	for _, entry := range env {
		key, _, ok := strings.Cut(entry, "=")
		if ok && strings.HasPrefix(key, "CGO_") {
			continue
		}
		filtered = append(filtered, entry)
	}
	return append(filtered,
		"GOFLAGS=", "GOWORK="+cfg.GoWork, "GOOS="+runtime.GOOS, "GOARCH="+runtime.GOARCH, "CGO_ENABLED=1",
	)
}
