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

package xgodriver

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/goplus/spx/v3/internal/envutil"
	"github.com/goplus/spx/v3/internal/interpruntime"
	"github.com/goplus/spx/v3/internal/launchpack"
	"github.com/goplus/spx/v3/internal/processsupervisor"
	"github.com/goplus/spx/v3/internal/projectpolicy"
	"github.com/goplus/spx/v3/x/xgolauncher"
)

func runProject(ctx context.Context, cfg Config, snapshot projectpolicy.PortableConfigSnapshot, streams IO) (xgolauncher.ProcessStatus, error) {
	packCfg := cfg.launchpackConfig(snapshot, streams)
	if cfg.DriverOrigin.IsLocal() {
		verifier, err := newGraphVerifier(ctx, cfg, streams.Env)
		if err != nil {
			return xgolauncher.ProcessStatus{}, fmt.Errorf("xgodriver: snapshot source graph: %w", err)
		}
		packCfg.VerifyGraph = verifier.verify
	}
	assets, err := launchpack.PrepareAssets(ctx, packCfg)
	if err != nil {
		return xgolauncher.ProcessStatus{}, fmt.Errorf("xgodriver: prepare assets: %w", err)
	}
	if assets.Cleanup != nil {
		defer assets.Cleanup()
	}
	return runEngine(ctx, cfg, assets, snapshot, streams)
}

func runEngine(ctx context.Context, cfg Config, assets launchpack.Assets, snapshot projectpolicy.PortableConfigSnapshot, streams IO) (xgolauncher.ProcessStatus, error) {
	sessionDir, err := os.MkdirTemp("", "spx-driver-run-")
	if err != nil {
		return xgolauncher.ProcessStatus{}, fmt.Errorf("xgodriver: create run session: %w", err)
	}
	if hasBuildFlag(cfg.BuildFlags, "work") {
		if streams.Stderr != nil {
			_, _ = fmt.Fprintf(streams.Stderr, "SPXWORK=%s\n", sessionDir)
		}
	} else {
		defer os.RemoveAll(sessionDir)
	}
	configDir, configIdentity, err := materializePortableConfigSnapshot(sessionDir, cfg.ProjectDir, snapshot)
	if err != nil {
		return xgolauncher.ProcessStatus{}, err
	}
	roots := interpruntime.Roots{
		ProjectDir: cfg.ProjectDir,
		AssetDir:   filepath.Join(cfg.ProjectDir, filepath.FromSlash(cfg.Project.PackDirectory)),
		SessionDir: sessionDir,
	}
	if err := assets.Verify(); err != nil {
		return xgolauncher.ProcessStatus{}, fmt.Errorf("xgodriver: verify runtime assets: %w", err)
	}
	if err := interpruntime.PrepareSession(interpruntime.SessionConfig{Roots: roots, BridgePath: assets.BridgePath}); err != nil {
		return xgolauncher.ProcessStatus{}, fmt.Errorf("xgodriver: prepare interpreted session: %w", err)
	}
	command, err := interpruntime.PrepareCommand(ctx, interpruntime.CommandConfig{
		Roots: roots, Executable: assets.EnginePath, Args: cfg.ApplicationArgs,
		Env:   append(sanitizeEnvironment(streams.Env), activeEnvironment+"=1"),
		Stdin: streams.Stdin, Stdout: streams.Stdout, Stderr: streams.Stderr,
		PathPolicy: interpruntime.RejectPath,
	})
	if err != nil {
		return xgolauncher.ProcessStatus{}, fmt.Errorf("xgodriver: prepare Engine: %w", err)
	}
	command.Env = append(command.Env,
		interpruntime.PortableConfigDirEnv+"="+configDir,
		interpruntime.PortableConfigIdentityEnv+"="+configIdentity,
	)
	status, err := processsupervisor.Run(ctx, command)
	if err != nil {
		return xgolauncher.ProcessStatus{}, fmt.Errorf("xgodriver: run Engine: %w", err)
	}
	return xgolauncher.ProcessStatus{Code: status.Code, Signal: status.Signal}, nil
}

func materializePortableConfigSnapshot(sessionDir, projectDir string, snapshot projectpolicy.PortableConfigSnapshot) (string, string, error) {
	if err := snapshot.Verify(projectDir); err != nil {
		return "", "", fmt.Errorf("xgodriver: %w", err)
	}
	identity, err := snapshot.Identity()
	if err != nil {
		return "", "", fmt.Errorf("xgodriver: identify portable config: %w", err)
	}
	configDir := filepath.Join(sessionDir, "portable-config")
	if err := os.Mkdir(configDir, 0o700); err != nil {
		return "", "", fmt.Errorf("xgodriver: create portable config directory: %w", err)
	}
	if !snapshot.Present() {
		return configDir, identity, nil
	}
	file, err := os.OpenFile(filepath.Join(configDir, ".config"), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return "", "", fmt.Errorf("xgodriver: create portable config: %w", err)
	}
	data := snapshot.Bytes()
	written, writeErr := file.Write(data)
	if writeErr == nil && written != len(data) {
		writeErr = io.ErrShortWrite
	}
	closeErr := file.Close()
	if writeErr != nil {
		return "", "", fmt.Errorf("xgodriver: write portable config: %w", writeErr)
	}
	if closeErr != nil {
		return "", "", fmt.Errorf("xgodriver: close portable config: %w", closeErr)
	}
	return configDir, identity, nil
}

func hasBuildFlag(flags []string, name string) bool {
	for _, flag := range flags {
		if flag == "-"+name || flag == "-"+name+"=true" {
			return true
		}
	}
	return false
}

func sanitizeEnvironment(env []string) []string {
	return envutil.Without(env, activeEnvironment, "GOFLAGS", "GOWORK", "GOOS", "GOARCH", "CGO_ENABLED")
}
