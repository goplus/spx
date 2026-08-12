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

import (
	"flag"
	"fmt"
	"io"
)

func ParseNoArgs(name, usage string, args []string, output io.Writer) error {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(output)
	fs.Usage = func() {
		fmt.Fprintln(output, usage)
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		fs.Usage()
		return ErrUsage
	}
	return nil
}

func validateWebMode(mode string) error {
	switch mode {
	case "normal", "worker", "minigame", "miniprogram":
		return nil
	default:
		return fmt.Errorf("unsupported web-mode: %s", mode)
	}
}

func validateOptionalPlatform(platform string) error {
	switch platform {
	case "", "android", "ios", "web", "linux", "windows", "macos":
		return nil
	default:
		return fmt.Errorf("unsupported platform: %s", platform)
	}
}
