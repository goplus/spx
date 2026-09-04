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

package runtimebundle

import (
	archivezip "archive/zip"
	"bytes"
	"errors"
	"os"
	"testing"
)

func TestPreflightMapsArchiveLimit(t *testing.T) {
	data := readPreflightZip(t,
		testZipEntry{name: "one", data: "1"},
		testZipEntry{name: "two", data: "2"},
	)
	limits := resolvedPreflightLimits(t, func(limits *Limits) {
		limits.MaxEntries = 1
	})
	if err := preflightZipArchive(bytes.NewReader(data), int64(len(data)), limits); !errors.Is(err, ErrArchiveLimit) {
		t.Fatalf("preflightZipArchive error = %v, want ErrArchiveLimit", err)
	}
}

func TestPreflightPreservesFormatError(t *testing.T) {
	data := append(readPreflightZip(t, testZipEntry{name: "one", data: "1"}), 0x7f)
	err := preflightZipArchive(bytes.NewReader(data), int64(len(data)), resolvedPreflightLimits(t, nil))
	if !errors.Is(err, archivezip.ErrFormat) {
		t.Fatalf("preflightZipArchive error = %v, want archive/zip format error", err)
	}
}

func TestVerifyZipReaderUsesPreflightLimits(t *testing.T) {
	data := readPreflightZip(t,
		testZipEntry{name: "one", data: "1"},
		testZipEntry{name: "two", data: "2"},
	)
	_, err := VerifyZipReader(bytes.NewReader(data), int64(len(data)), VerifyOptions{
		Limits: Limits{MaxEntries: 1},
	})
	if !errors.Is(err, ErrArchiveLimit) {
		t.Fatalf("VerifyZipReader error = %v, want ErrArchiveLimit", err)
	}
}

func readPreflightZip(t *testing.T, entries ...testZipEntry) []byte {
	t.Helper()
	data, err := os.ReadFile(writeTestZip(t, entries...))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func resolvedPreflightLimits(t *testing.T, update func(*Limits)) Limits {
	t.Helper()
	limits := Limits{}
	if update != nil {
		update(&limits)
	}
	resolved, err := limits.withDefaults()
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}
