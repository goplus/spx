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

type BuildImagesConfig struct {
	ProxyURL string
}

type BuildEngineConfig struct {
	GodotSrc string
}

func Run(args []string) error { return runDocker(args) }
func ParseDockerBuildImagesArgs(args []string) (BuildImagesConfig, error) {
	cfg, err := parseDockerBuildImagesArgs(args)
	return BuildImagesConfig{ProxyURL: cfg.proxyURL}, err
}
func ParseDockerBuildEngineArgs(args []string) (BuildEngineConfig, error) {
	cfg, err := parseDockerBuildEngineArgs(args)
	return BuildEngineConfig{GodotSrc: cfg.godotSrc}, err
}
func RunDockerBuildImages(cfg BuildImagesConfig) error {
	return runDockerBuildImages(dockerBuildImagesConfig{proxyURL: cfg.ProxyURL})
}
func RunDockerBuildEngine(cfg BuildEngineConfig) error {
	return runDockerBuildEngine(dockerBuildEngineConfig{godotSrc: cfg.GodotSrc})
}
func BuildPodmanSConsArgs(godotPath, platform string, sconsArgs []string, useTTY bool) []string {
	return buildPodmanSConsArgs(godotPath, platform, sconsArgs, useTTY)
}
