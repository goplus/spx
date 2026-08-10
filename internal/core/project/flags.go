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

package project

import "flag"

type CommandLineEffects struct {
	Verbose  bool
	ShowHelp bool
}

func ParseCommandLineFlags(fs *flag.FlagSet, args []string, conf *Config) (CommandLineEffects, error) {
	var effects CommandLineEffects
	if conf.DontParseFlags {
		return effects, nil
	}

	verbose := fs.Bool("v", false, "print verbose information")
	fullscreen := fs.Bool("f", false, "full screen")
	help := fs.Bool("h", false, "show help information")
	fullscreen2 := fs.Bool("fullscreen", false, "server mode")

	fs.String("controller", "", "controller's name")
	fs.Bool("servermode", false, "server mode")
	fs.String("serveraddr", "", "server address")
	fs.Bool("nomap", false, "server mode")
	fs.Bool("debugweb", false, "server mode")
	fs.String("write-movie", "", "movie mode")

	fs.String("path", "", "gdspx project path")
	fs.Bool("e", false, "editor mode")
	fs.Bool("headless", false, "Headless Mode")
	fs.Bool("remote-debug", false, "remote Debug Mode")
	fs.Bool("no-header", false, "disable engine's header output")

	if err := fs.Parse(args); err != nil {
		return effects, err
	}

	effects.Verbose = *verbose
	effects.ShowHelp = *help
	conf.FullScreen = conf.FullScreen || *fullscreen2 || *fullscreen
	return effects, nil
}
