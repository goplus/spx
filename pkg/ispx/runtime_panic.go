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

package ispx

import (
	"io"
	"log/slog"
	"os"

	"github.com/goplus/ixgo"
)

type runtimePanicFields struct {
	Error    error
	Function string
	File     string
	Line     int
	Column   int
}

var runtimePanicLogger = newRuntimePanicLogger(os.Stdout)

func newRuntimePanicLogger(w io.Writer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(w, nil))
}

func logRuntimePanic(info *ixgo.PanicInfo) {
	if !shouldLogRuntimePanic(info) {
		return
	}

	position := info.Position()
	logRuntimePanicFields(runtimePanicLogger, runtimePanicFields{
		Error:    info.Error,
		Function: runtimePanicFunction(info),
		File:     position.Filename,
		Line:     position.Line,
		Column:   position.Column,
	})
}

func shouldLogRuntimePanic(info *ixgo.PanicInfo) bool {
	if info == nil {
		return false
	}

	fatal, ok := info.Error.(ixgo.FatalError)
	if !ok {
		return true
	}

	switch fatal.Value.(type) {
	case ixgo.RuntimeError, ixgo.PlainError, ixgo.PanicError:
		return false
	default:
		return true
	}
}

func runtimePanicFunction(info *ixgo.PanicInfo) string {
	if info == nil {
		return ""
	}
	if fn := info.Parent(); fn != nil {
		return fn.String()
	}
	return info.String()
}

func logRuntimePanicFields(logger *slog.Logger, fields runtimePanicFields) {
	logger.Error(
		"panic",
		"error", fields.Error,
		"function", fields.Function,
		"file", fields.File,
		"line", fields.Line,
		"column", fields.Column,
	)
}
