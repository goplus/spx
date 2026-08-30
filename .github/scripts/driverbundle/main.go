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

// Command driverbundle packages and verifies the host driver bundle.
package main

import (
	"fmt"
	"io"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		usage(os.Stderr)
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "package":
		err = runPackage(os.Args[2:])
	case "verify":
		err = runVerify(os.Args[2:])
	case "assemble":
		err = runAssemble(os.Args[2:])
	case "verify-release":
		err = runVerifyRelease(os.Args[2:])
	case "-h", "--help", "help":
		usage(os.Stdout)
		return
	default:
		err = fmt.Errorf("unknown command %q (want package, verify, assemble, or verify-release)", os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func usage(output io.Writer) {
	fmt.Fprintln(output, "usage: driverbundle package|verify|assemble|verify-release [flags]")
}
