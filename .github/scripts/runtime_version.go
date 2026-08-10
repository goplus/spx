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

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/goplus/spx/v3/internal/release"
)

var repositoryNamePattern = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)

type releaseDescription struct {
	ReleaseTag        string `json:"release_tag"`
	Version           string `json:"version"`
	RuntimeVersion    string `json:"runtime_version"`
	RuntimeABI        int    `json:"runtime_abi"`
	RuntimeTag        string `json:"runtime_tag"`
	RuntimeManifest   string `json:"runtime_manifest"`
	ReleaseRepository string `json:"release_repository"`
	GodotRepository   string `json:"godot_repository"`
	GodotCommit       string `json:"godot_commit"`
}

type githubOutput struct {
	name  string
	value string
}

func main() {
	spxVersion := flag.Bool("spx-version", false, "print the SPX release tag instead of the runtime version")
	runtimeTag := flag.Bool("runtime-tag", false, "print the runtime release tag instead of the runtime version")
	resolveSPX := flag.String("resolve-spx", "", "resolve the runtime version for an exact SPX release tag")
	jsonOutput := flag.Bool("json", false, "print the complete current release description as JSON")
	githubOutput := flag.String("github-output", "", "append the complete current release description to a GitHub Actions output file")
	flag.Parse()
	selectedModes := 0
	for _, selected := range []bool{*spxVersion, *runtimeTag, *resolveSPX != "", *jsonOutput, *githubOutput != ""} {
		if selected {
			selectedModes++
		}
	}
	if flag.NArg() != 0 || selectedModes > 1 {
		flag.Usage()
		os.Exit(2)
	}
	if *spxVersion {
		fmt.Println(release.DefaultReleaseMeta().SPXVersion)
		return
	}
	if *resolveSPX != "" {
		meta, err := release.ResolveReleaseMetaForSPXVersion(*resolveSPX)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println(meta.Runtime.Version)
		return
	}
	if *runtimeTag {
		fmt.Println(release.DefaultRuntimeLock().RuntimeReleaseTag())
		return
	}
	if *jsonOutput || *githubOutput != "" {
		description, err := describeRelease()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		if *jsonOutput {
			if err := json.NewEncoder(os.Stdout).Encode(description); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			return
		}
		if err := writeGitHubOutput(*githubOutput, description.githubOutputs()); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	fmt.Println(release.DefaultRuntimeLock().RuntimeVersion)
}

func describeRelease() (releaseDescription, error) {
	meta := release.DefaultReleaseMeta()
	lock := release.DefaultRuntimeLock()
	godotRepository, err := githubRepositoryName(lock.Godot.Repository)
	if err != nil {
		return releaseDescription{}, err
	}
	return releaseDescription{
		ReleaseTag:        meta.SPXVersion,
		Version:           strings.TrimPrefix(meta.SPXVersion, "v"),
		RuntimeVersion:    lock.RuntimeVersion,
		RuntimeABI:        lock.RuntimeABI,
		RuntimeTag:        lock.RuntimeReleaseTag(),
		RuntimeManifest:   lock.Manifest,
		ReleaseRepository: lock.ReleaseRepository,
		GodotRepository:   godotRepository,
		GodotCommit:       lock.Godot.Commit,
	}, nil
}

func githubRepositoryName(repositoryURL string) (string, error) {
	const prefix = "https://github.com/"
	if !strings.HasPrefix(repositoryURL, prefix) || !strings.HasSuffix(repositoryURL, ".git") {
		return "", fmt.Errorf("runtime Godot repository %q must use %sowner/repository.git", repositoryURL, prefix)
	}
	repository := strings.TrimSuffix(strings.TrimPrefix(repositoryURL, prefix), ".git")
	if !repositoryNamePattern.MatchString(repository) {
		return "", fmt.Errorf("runtime Godot repository %q must identify one owner/repository", repositoryURL)
	}
	return repository, nil
}

func (d releaseDescription) githubOutputs() []githubOutput {
	return []githubOutput{
		{name: "release_tag", value: d.ReleaseTag},
		{name: "version", value: d.Version},
		{name: "runtime_version", value: d.RuntimeVersion},
		{name: "runtime_abi", value: strconv.Itoa(d.RuntimeABI)},
		{name: "runtime_tag", value: d.RuntimeTag},
		{name: "runtime_manifest", value: d.RuntimeManifest},
		{name: "release_repository", value: d.ReleaseRepository},
		{name: "godot_repository", value: d.GodotRepository},
		{name: "godot_commit", value: d.GodotCommit},
	}
}

func writeGitHubOutput(path string, outputs []githubOutput) error {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		return fmt.Errorf("open GitHub output: %w", err)
	}
	defer file.Close()
	for _, output := range outputs {
		if strings.ContainsAny(output.value, "\r\n") {
			return fmt.Errorf("GitHub output %q must be a single line", output.name)
		}
		if _, err := fmt.Fprintf(file, "%s=%s\n", output.name, output.value); err != nil {
			return fmt.Errorf("write GitHub output: %w", err)
		}
	}
	return nil
}
