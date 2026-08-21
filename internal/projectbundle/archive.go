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

package projectbundle

import (
	"archive/zip"
	"compress/flate"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"time"
)

var canonicalZipTime = time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)

// String returns the lowercase hexadecimal SHA-256.
func (d Digest) String() string {
	return hex.EncodeToString(d[:])
}

// WriteArchive collects cfg and writes its canonical ZIP to w.
func WriteArchive(w io.Writer, cfg Config) (Digest, error) {
	bundle, err := collect(cfg)
	if err != nil {
		return Digest{}, err
	}
	return bundle.writeZIP(w)
}

// writeZIP writes the canonical ZIP to w. It does not close w.
func (b *bundle) writeZIP(w io.Writer) (Digest, error) {
	if b == nil {
		return Digest{}, errors.New("projectbundle: nil bundle")
	}
	if w == nil {
		return Digest{}, errors.New("projectbundle: nil writer")
	}

	hasher := sha256.New()
	limited := &archiveLimitWriter{writer: w, remaining: b.limits.maxArchiveBytes}
	writer := zip.NewWriter(io.MultiWriter(limited, hasher))
	writer.RegisterCompressor(zip.Deflate, func(w io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(w, flate.BestCompression)
	})

	for _, entry := range b.entries {
		header := &zip.FileHeader{Name: entry.name, Method: zip.Deflate}
		header.SetModTime(canonicalZipTime)
		header.SetMode(0o644)
		output, err := writer.CreateHeader(header)
		if err != nil {
			_ = writer.Close()
			return Digest{}, fmt.Errorf("projectbundle: create ZIP entry %q: %w", entry.name, err)
		}
		if _, err := output.Write(entry.data); err != nil {
			_ = writer.Close()
			return Digest{}, fmt.Errorf("projectbundle: write ZIP entry %q: %w", entry.name, err)
		}
	}
	if err := writer.Close(); err != nil {
		return Digest{}, fmt.Errorf("projectbundle: close ZIP: %w", err)
	}

	var digest Digest
	copy(digest[:], hasher.Sum(nil))
	return digest, nil
}

type archiveLimitWriter struct {
	writer    io.Writer
	remaining int64
}

func (w *archiveLimitWriter) Write(data []byte) (int, error) {
	if int64(len(data)) > w.remaining {
		return 0, fmt.Errorf("%w: ZIP exceeds archive byte limit", ErrLimit)
	}
	n, err := w.writer.Write(data)
	w.remaining -= int64(n)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	return n, err
}
