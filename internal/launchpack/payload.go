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

package launchpack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/goplus/spx/v3/internal/projectbundle"
	"github.com/goplus/spx/v3/internal/projectpolicy"
	"github.com/goplus/spx/v3/internal/runtimebundle"
	"github.com/goplus/spx/v3/internal/runtimepayload"
)

func buildSourceBridge(ctx context.Context, cfg Config, bridgeName string, streams IO) (string, func(), error) {
	if err := cfg.validateGraphInputs(); err != nil {
		return "", nil, err
	}
	if err := cfg.verifyGraph(ctx, "before source bridge build"); err != nil {
		return "", nil, err
	}
	if cfg.BridgePackage == "" {
		return "", nil, fmt.Errorf("launchpack: bridge package is required")
	}
	workDir, err := os.MkdirTemp("", "spx-launchpack-bridge-")
	if err != nil {
		return "", nil, fmt.Errorf("launchpack: create source bridge work directory: %w", err)
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
	command.Dir = cfg.WorkDir
	command.Env = sourceBridgeEnv(cfg, streams.Env)
	command.Stdin = streams.Stdin
	command.Stdout = streams.Stdout
	command.Stderr = streams.Stderr
	if err := command.Run(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("launchpack: build source interpreter bridge: %w", err)
	}
	if err := cfg.verifyGraph(ctx, "after source bridge build"); err != nil {
		cleanup()
		return "", nil, err
	}
	if err := validatePinnedFile("source interpreter bridge", bridgePath); err != nil {
		cleanup()
		return "", nil, err
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
	return append(args, "-o", bridgePath, cfg.BridgePackage)
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
		return "", fmt.Errorf("launchpack: host platform %s/%s is not supported", goos, goarch)
	}
	return "gdspx-" + goos + "-" + goarch + extension, nil
}

func buildLauncher(ctx context.Context, cfg Config, assets Assets, configSnapshot projectpolicy.PortableConfigSnapshot, streams IO) (string, string, error) {
	projectConfig, err := prepareProjectBundleConfig(cfg, configSnapshot)
	if err != nil {
		return "", "", err
	}
	var payloadDigest, manifestDigest string
	err = compileLauncher(ctx, cfg, streams, func(workDir string, dst io.Writer) (string, string, error) {
		var buildErr error
		payloadDigest, manifestDigest, buildErr = writeLauncherPayload(workDir, dst, cfg, assets, projectConfig, streams)
		return payloadDigest, manifestDigest, buildErr
	})
	return payloadDigest, manifestDigest, err
}

func writeLauncherPayload(workDir string, dst io.Writer, cfg Config, assets Assets, projectConfig projectbundle.Config, streams IO) (payloadDigest, manifestDigest string, err error) {
	projectPath := filepath.Join(workDir, "project.zip")
	projectFile, err := os.OpenFile(projectPath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", "", fmt.Errorf("launchpack: create project archive: %w", err)
	}
	defer func() {
		if closeErr := projectFile.Close(); err == nil && closeErr != nil {
			payloadDigest, manifestDigest, err = "", "", fmt.Errorf("launchpack: close project archive: %w", closeErr)
		}
	}()
	projectDigest, err := projectbundle.WriteArchive(projectFile, projectConfig)
	if err != nil {
		return "", "", fmt.Errorf("launchpack: collect project: %w", err)
	}
	if err := projectFile.Sync(); err != nil {
		return "", "", fmt.Errorf("launchpack: sync project archive: %w", err)
	}
	projectInfo, err := projectFile.Stat()
	if err != nil {
		return "", "", fmt.Errorf("launchpack: stat project archive: %w", err)
	}
	projectBundle, err := runtimepayload.ComponentBundleReaderAt(projectFile, projectInfo.Size(), runtimebundle.NamespaceProject)
	if err != nil {
		return "", "", fmt.Errorf("launchpack: verify generated project bundle: %w", err)
	}

	engine, err := openPinnedFile("Engine", assets.EnginePath)
	if err != nil {
		return "", "", err
	}
	defer func() {
		if closeErr := engine.file.Close(); err == nil && closeErr != nil {
			payloadDigest, manifestDigest, err = "", "", fmt.Errorf("launchpack: close Engine: %w", closeErr)
		}
	}()
	pack, err := openPinnedFile("runtime PCK", assets.PackPath)
	if err != nil {
		return "", "", err
	}
	defer func() {
		if closeErr := pack.file.Close(); err == nil && closeErr != nil {
			payloadDigest, manifestDigest, err = "", "", fmt.Errorf("launchpack: close runtime PCK: %w", closeErr)
		}
	}()
	bridge, err := openPinnedFile("interpreter bridge", assets.BridgePath)
	if err != nil {
		return "", "", err
	}
	defer func() {
		if closeErr := bridge.file.Close(); err == nil && closeErr != nil {
			payloadDigest, manifestDigest, err = "", "", fmt.Errorf("launchpack: close interpreter bridge: %w", closeErr)
		}
	}()

	interfaceDigest, engineDigest, packDigest, err := localEngineSourceDigests(engine.source(""), pack.source(""))
	if err != nil {
		return "", "", err
	}
	bridgeDigest, err := digestFileSource(bridge.source(""))
	if err != nil {
		return "", "", fmt.Errorf("launchpack: hash interpreter bridge: %w", err)
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
		Schema: "spx-local-bridge/v1", Mode: "source", SPXSource: cfg.Source.EffectivePath,
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
		return "", "", fmt.Errorf("launchpack: identify Engine bundle: %w", err)
	}
	if !bundleEntryHasDigest(engineBundle, engineName, engineDigest) || !bundleEntryHasDigest(engineBundle, packName, packDigest) {
		return "", "", errors.New("launchpack: Engine or runtime PCK changed while identifying payload")
	}
	bridgeSources := []runtimepayload.FileSource{
		byteSource("bridge-manifest.json", 0o644, bridgeManifest),
		bridge.source(bridgeName),
	}
	bridgeBundle, err := runtimepayload.ComponentBundleSources(bridgeSources, runtimebundle.NamespaceBridge)
	if err != nil {
		return "", "", fmt.Errorf("launchpack: identify bridge bundle: %w", err)
	}
	if !bundleEntryHasDigest(bridgeBundle, bridgeName, bridgeDigest) {
		return "", "", errors.New("launchpack: interpreter bridge changed while identifying payload")
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
			SelectedPath: cfg.Source.SelectedPath, SelectedVersion: cfg.Source.SelectedVersion,
			EffectivePath: cfg.Source.EffectivePath, EffectiveVersion: cfg.Source.EffectiveVersion,
			Main: cfg.Source.Main, SourceMode: cfg.Source.SourceMode,
		},
		Target: runtimepayload.Target{GOOS: runtime.GOOS, GOARCH: runtime.GOARCH},
		Engine: runtimepayload.Engine{
			RuntimeVersion: assets.Lock.RuntimeVersion, RuntimeABI: assets.Lock.RuntimeABI,
			EngineInterfaceDigest: interfaceDigest, Executable: engineName, Pack: packName,
			BundleDigest: engineBundle.Digest,
		},
		Bridge: runtimepayload.Bridge{File: bridgeName, BundleDigest: bridgeBundle.Digest},
		Project: runtimepayload.Project{
			PackDirectory: cfg.PackDir, BundleDigest: projectBundle.Digest,
			ArchiveSHA256: projectDigest.String(),
		},
	}, payloadSources)
	if err != nil {
		return "", "", fmt.Errorf("launchpack: build embedded payload: %w", err)
	}
	for _, source := range []*pinnedFile{engine, pack, bridge} {
		if err := source.verify(); err != nil {
			return "", "", err
		}
	}
	if traceEnabled(cfg.BuildFlags) && streams.Stderr != nil {
		_, _ = fmt.Fprintf(streams.Stderr, "launchpack: project=%s payload=%s engine=%s bridge=%s\n", projectDigest, payloadDigest, engineBundle.Digest, bridgeBundle.Digest)
	}
	return payloadDigest, manifestDigest, nil
}
