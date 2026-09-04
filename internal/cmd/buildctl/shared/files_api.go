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
	// MaterializeSymlinksAsFiles writes validated ZIP symlink target text as
	// an ordinary file. It is intended only for vetted toolchain archives.
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

func ExtractZipWithLimits(srcZip, dstDir string, limits ZipLimits) error {
	return extractZipWithLimits(srcZip, dstDir, limits)
}

func ExtractZipWithOptions(srcZip, dstDir string, options ZipExtractOptions) error {
	return extractZipWithOptions(srcZip, dstDir, options)
}

func FetchURLToFile(url, dst string) error {
	return fetchURLToFile(url, dst)
}
