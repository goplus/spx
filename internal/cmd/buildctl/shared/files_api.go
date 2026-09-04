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

package shared

import "github.com/goplus/spx/v3/internal/runtimebundle"

// ZipLimits controls bounded verification and extraction of untrusted ZIPs.
type ZipLimits = runtimebundle.Limits

// ZipExtractOptions controls bounded extraction of an untrusted ZIP.
type ZipExtractOptions struct {
	Limits ZipLimits
	// MaterializeSymlinksAsFiles writes vetted symlink targets as regular files.
	MaterializeSymlinksAsFiles bool
}

func DirExists(path string) bool {
	return dirExists(path)
}

func CopyDir(src, dst string) error {
	return copyDir(src, dst)
}

func ZipDirectory(srcDir, dstZip string) error {
	return zipDirectory(srcDir, dstZip)
}

func ExtractZip(srcZip, dstDir string) error {
	return extractZip(srcZip, dstDir)
}

func ExtractZipWithOptions(srcZip, dstDir string, options ZipExtractOptions) error {
	return extractZipWithOptions(srcZip, dstDir, options)
}

func FetchURLToFile(url, dst string) error {
	return fetchURLToFile(url, dst)
}

// FetchURLToFileWithLimit downloads atomically within maxBytes.
func FetchURLToFileWithLimit(url, dst string, maxBytes int64) error {
	return fetchURLToFileWithLimit(url, dst, maxBytes)
}

// ReplaceFile atomically replaces dst with a same-filesystem src.
func ReplaceFile(src, dst string) error {
	return replaceFile(src, dst)
}
