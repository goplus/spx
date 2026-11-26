//go:build windows

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

package command

import (
	"os"
	"strings"
	"syscall"
)

func outputPathsEqual(left, right string) bool {
	return strings.EqualFold(left, right)
}

func launcherOutputIsReparse(info os.FileInfo) bool {
	attrs := info.Sys().(*syscall.Win32FileAttributeData)
	return attrs.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0
}

func launcherOutputAliasAllowed(existing, _ string) bool {
	// EvalSymlinks also expands 8.3 names. Check the original path for
	// reparse points before accepting its expanded spelling.
	return validateLauncherExistingPath(existing, false) == nil
}

// launchpack validates the PE header before this size check.
func launcherStageIsExecutable(info os.FileInfo) bool {
	return info.Size() > 0
}
