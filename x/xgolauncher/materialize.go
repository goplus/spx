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
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/goplus/spx/v3/internal/runtimebundle"
	"github.com/goplus/spx/v3/internal/runtimepayload"
)

func materializeComponent(ctx context.Context, cache *runtimebundle.Cache, workDir string, namespace runtimebundle.Namespace, expectedDigest string, write func(io.Writer) error) (*runtimebundle.Materialized, error) {
	if ctx == nil {
		return nil, fmt.Errorf("xgolauncher: nil materialization context")
	}
	if write == nil {
		return nil, fmt.Errorf("xgolauncher: nil component writer")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	zipPath := filepath.Join(workDir, string(namespace)+".zip")
	file, err := os.OpenFile(zipPath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return nil, err
	}
	closeFile := true
	defer func() {
		if closeFile {
			_ = file.Close()
		}
	}()
	if err := write(contextWriter{ctx: ctx, dst: file}); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := file.Sync(); err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	bundle, err := runtimepayload.ComponentBundleReaderAt(file, info.Size(), namespace)
	if err != nil {
		return nil, err
	}
	if bundle.Digest != expectedDigest {
		return nil, fmt.Errorf("embedded %s identity changed after payload verification", namespace)
	}
	if err := file.Close(); err != nil {
		return nil, err
	}
	closeFile = false
	return cache.Materialize(ctx, namespace, zipPath, &bundle)
}

type contextWriter struct {
	ctx context.Context
	dst io.Writer
}

func (w contextWriter) Write(p []byte) (int, error) {
	if err := w.ctx.Err(); err != nil {
		return 0, err
	}
	return w.dst.Write(p)
}
