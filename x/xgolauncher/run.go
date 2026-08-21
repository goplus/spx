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

package xgolauncher

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	"github.com/goplus/spx/v3/internal/interpruntime"
	"github.com/goplus/spx/v3/internal/processsupervisor"
	"github.com/goplus/spx/v3/internal/runtimebundle"
	"github.com/goplus/spx/v3/internal/runtimepayload"
)

func run(ctx context.Context, cfg Config, cacheRoot string) (ProcessStatus, error) {
	if ctx == nil {
		return ProcessStatus{}, errors.New("xgolauncher: nil context")
	}
	if err := ctx.Err(); err != nil {
		return ProcessStatus{}, err
	}
	verified, err := runtimepayload.Verify(cfg.Payload, cfg.PayloadSHA256, cfg.ManifestSHA256, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return ProcessStatus{}, err
	}
	if err := ctx.Err(); err != nil {
		return ProcessStatus{}, err
	}
	if cacheRoot == "" {
		cacheRoot = runtimebundle.DefaultCacheRoot()
	}
	cache := runtimebundle.NewCache(cacheRoot)
	if _, err := cache.Path(runtimebundle.NamespaceEngine, verified.Manifest.Engine.BundleDigest); err != nil {
		return ProcessStatus{}, fmt.Errorf("xgolauncher: validate cache root: %w", err)
	}

	workDir, err := os.MkdirTemp("", "spx-launcher-")
	if err != nil {
		return ProcessStatus{}, fmt.Errorf("xgolauncher: create launcher work directory: %w", err)
	}
	defer os.RemoveAll(workDir)

	engine, err := materializeComponent(ctx, cache, workDir, runtimebundle.NamespaceEngine, verified.Manifest.Engine.BundleDigest, func(dst io.Writer) error {
		return verified.WriteComponentZIP("engine/", dst)
	})
	if err != nil {
		return ProcessStatus{}, fmt.Errorf("xgolauncher: materialize Engine: %w", err)
	}
	defer engine.Close()
	bridge, err := materializeComponent(ctx, cache, workDir, runtimebundle.NamespaceBridge, verified.Manifest.Bridge.BundleDigest, func(dst io.Writer) error {
		return verified.WriteComponentZIP("bridge/", dst)
	})
	if err != nil {
		return ProcessStatus{}, fmt.Errorf("xgolauncher: materialize bridge: %w", err)
	}
	defer bridge.Close()
	project, err := materializeComponent(ctx, cache, workDir, runtimebundle.NamespaceProject, verified.Manifest.Project.BundleDigest, func(dst io.Writer) error {
		return verified.WriteProjectZIP(dst)
	})
	if err != nil {
		return ProcessStatus{}, fmt.Errorf("xgolauncher: materialize project: %w", err)
	}
	defer project.Close()

	if err := ctx.Err(); err != nil {
		return ProcessStatus{}, err
	}
	sessionDir, err := os.MkdirTemp("", "spx-session-")
	if err != nil {
		return ProcessStatus{}, fmt.Errorf("xgolauncher: create Engine session: %w", err)
	}
	defer os.RemoveAll(sessionDir)
	if err := ctx.Err(); err != nil {
		return ProcessStatus{}, err
	}
	roots := interpruntime.Roots{
		ProjectDir: project.Path,
		AssetDir:   filepath.Join(project.Path, filepath.FromSlash(verified.Manifest.Project.PackDirectory)),
		SessionDir: sessionDir,
	}
	bridgePath := filepath.Join(bridge.Path, verified.Manifest.Bridge.File)
	if err := interpruntime.PrepareSession(interpruntime.SessionConfig{Roots: roots, BridgePath: bridgePath}); err != nil {
		return ProcessStatus{}, fmt.Errorf("xgolauncher: prepare Engine session: %w", err)
	}
	command, err := interpruntime.PrepareCommand(ctx, interpruntime.CommandConfig{
		Roots: roots, Executable: filepath.Join(engine.Path, verified.Manifest.Engine.Executable),
		Args: cfg.Args, Env: os.Environ(), Stdin: cfg.Stdin, Stdout: cfg.Stdout, Stderr: cfg.Stderr,
		PathPolicy: interpruntime.RejectPath,
	})
	if err != nil {
		return ProcessStatus{}, fmt.Errorf("xgolauncher: prepare Engine command: %w", err)
	}
	status, err := processsupervisor.Run(ctx, command)
	if err != nil {
		return ProcessStatus{}, fmt.Errorf("xgolauncher: run Engine: %w", err)
	}
	return ProcessStatus{Code: status.Code, Signal: status.Signal}, nil
}
