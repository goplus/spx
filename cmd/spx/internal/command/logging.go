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

package command

import "github.com/goplus/spx/v3/cmd/spx/internal/logutil"

func enableDebugLogging() {
	logutil.EnableDebug()
}

func logDebugf(format string, args ...any) {
	logutil.Debugf(format, args...)
}

func logInfof(format string, args ...any) {
	logutil.Infof(format, args...)
}

func logWarnf(format string, args ...any) {
	logutil.Warnf(format, args...)
}

func logErrorf(format string, args ...any) {
	logutil.Errorf(format, args...)
}

func logFatalf(format string, args ...any) {
	logutil.Fatalf(format, args...)
}
