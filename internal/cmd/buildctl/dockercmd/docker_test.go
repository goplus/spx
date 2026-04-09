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

package dockercmd

import (
	"reflect"
	"testing"
)

func TestParseDockerBuildImagesArgsFromFlag(t *testing.T) {
	cfg, err := parseDockerBuildImagesArgs([]string{"--proxy-url", "http://127.0.0.1:7890"})
	if err != nil {
		t.Fatalf("parseDockerBuildImagesArgs returned error: %v", err)
	}
	if cfg.proxyURL != "http://127.0.0.1:7890" {
		t.Fatalf("proxyURL = %q, want %q", cfg.proxyURL, "http://127.0.0.1:7890")
	}
}

func TestParseDockerBuildImagesArgsFromPositional(t *testing.T) {
	cfg, err := parseDockerBuildImagesArgs([]string{"http://127.0.0.1:7890"})
	if err != nil {
		t.Fatalf("parseDockerBuildImagesArgs returned error: %v", err)
	}
	if cfg.proxyURL != "http://127.0.0.1:7890" {
		t.Fatalf("proxyURL = %q, want %q", cfg.proxyURL, "http://127.0.0.1:7890")
	}
}

func TestParseDockerBuildEngineArgsDefault(t *testing.T) {
	cfg, err := parseDockerBuildEngineArgs(nil)
	if err != nil {
		t.Fatalf("parseDockerBuildEngineArgs returned error: %v", err)
	}
	if cfg.godotSrc != "" {
		t.Fatalf("godotSrc = %q, want empty", cfg.godotSrc)
	}
}

func TestBuildPodmanSConsArgsAvoidsShellConcatenation(t *testing.T) {
	got := buildPodmanSConsArgs("/tmp/godot", "windows", []string{"target=template_release", "arch=x86_64"}, true)
	want := []string{
		"run",
		"--rm",
		"-it",
		"-w", "/root/godot",
		"-v", "/tmp/godot:/root/godot:z",
		"godot-windows:" + DockerImageVersion,
		"scons",
		"platform=windows",
		"target=template_release",
		"arch=x86_64",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildPodmanSConsArgs = %#v, want %#v", got, want)
	}
}
