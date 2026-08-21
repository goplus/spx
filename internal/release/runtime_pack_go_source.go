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

package release

import (
	"bytes"
	"fmt"
	"go/build"
	"io"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	runtimePackGOOS   = "linux"
	runtimePackGOARCH = "amd64"
)

type runtimePackBlob struct {
	name string
	hash string
}

func runtimePackSourcePaths(repoRoot string, tree []byte) (map[string]struct{}, map[string][]byte, error) {
	paths := make(map[string]struct{})
	var blobs []runtimePackBlob
	if err := walkGitTree(tree, func(entry []byte, name string) error {
		if !isRuntimePackSourcePath(name) {
			return nil
		}
		if !strings.HasSuffix(name, ".go") && name != runtimePackExportPresets {
			paths[name] = struct{}{}
			return nil
		}
		fields := strings.Fields(string(entry[:bytes.IndexByte(entry, '\t')]))
		if len(fields) != 3 || fields[1] != "blob" {
			return fmt.Errorf("release: invalid git blob entry %q", entry)
		}
		blobs = append(blobs, runtimePackBlob{name: name, hash: fields[2]})
		return nil
	}); err != nil {
		return nil, nil, err
	}

	sources, err := readGitBlobs(repoRoot, blobs)
	if err != nil {
		return nil, nil, err
	}
	projections := make(map[string][]byte)
	for _, blob := range blobs {
		if blob.name == runtimePackExportPresets {
			projection, err := projectGodotPreset(sources[blob.name], runtimePackExportPresetName)
			if err != nil {
				return nil, nil, fmt.Errorf("release: project runtime pack preset: %w", err)
			}
			projections[blob.name] = projection
			continue
		}
		matched, err := matchesRuntimePackTarget(blob.name, sources[blob.name])
		if err != nil {
			return nil, nil, fmt.Errorf("release: match runtime pack source %q: %w", blob.name, err)
		}
		if matched {
			paths[blob.name] = struct{}{}
		}
	}
	return paths, projections, nil
}

func matchesRuntimePackTarget(name string, content []byte) (bool, error) {
	ctx := build.Default
	ctx.GOOS = runtimePackGOOS
	ctx.GOARCH = runtimePackGOARCH
	ctx.CgoEnabled = true
	ctx.BuildTags = nil
	ctx.OpenFile = func(path string) (io.ReadCloser, error) {
		if filepath.ToSlash(filepath.Clean(path)) != filepath.ToSlash(filepath.Clean(name)) {
			return nil, fmt.Errorf("unexpected source path %q", path)
		}
		return io.NopCloser(bytes.NewReader(content)), nil
	}
	return ctx.MatchFile(filepath.Dir(name), filepath.Base(name))
}

func readGitBlobs(repoRoot string, blobs []runtimePackBlob) (map[string][]byte, error) {
	if len(blobs) == 0 {
		return nil, nil
	}
	var input strings.Builder
	for _, blob := range blobs {
		input.WriteString(blob.hash)
		input.WriteByte('\n')
	}
	command := exec.Command("git", "-C", filepath.Clean(repoRoot), "cat-file", "--batch")
	command.Stdin = strings.NewReader(input.String())
	output, err := command.Output()
	if err != nil {
		return nil, fmt.Errorf("release: read runtime pack source blobs: %w", err)
	}

	contents := make(map[string][]byte, len(blobs))
	for _, blob := range blobs {
		end := bytes.IndexByte(output, '\n')
		if end < 0 {
			return nil, fmt.Errorf("release: malformed git cat-file response for %s", blob.hash)
		}
		fields := strings.Fields(string(output[:end]))
		if len(fields) != 3 || fields[0] != blob.hash || fields[1] != "blob" {
			return nil, fmt.Errorf("release: invalid git cat-file response %q", output[:end])
		}
		size, err := strconv.Atoi(fields[2])
		if err != nil || size < 0 || len(output) < end+1+size+1 {
			return nil, fmt.Errorf("release: invalid git blob size %q", fields[2])
		}
		start := end + 1
		contents[blob.name] = output[start : start+size]
		output = output[start+size+1:]
	}
	if len(output) != 0 {
		return nil, fmt.Errorf("release: unexpected trailing git cat-file output")
	}
	return contents, nil
}
