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

// Package stablefile reads regular files while checking their identity and metadata.
package stablefile

import (
	"bytes"
	"errors"
	"io"
	"math"
	"os"
)

var (
	ErrNotRegular     = errors.New("must be a regular non-symlink file")
	ErrChangedOpening = errors.New("changed while opening")
	ErrChangedReading = errors.New("changed while reading")
	ErrTooLarge       = errors.New("file exceeds size limit")
)

// Reader controls optional files and bounded reads. A zero MaxBytes means no
// limit. Lstat and Open default to os.Lstat and os.Open; set both to os.Root
// methods to retain a pinned directory boundary.
type Reader struct {
	MaxBytes     int64
	AllowMissing bool
	Lstat        func(string) (os.FileInfo, error)
	Open         func(string) (*os.File, error)
}

// Read returns bytes and metadata only after a successful stable read.
// An allowed missing file returns nil data, nil metadata, and no error.
func (r Reader) Read(name string) ([]byte, os.FileInfo, error) {
	var data bytes.Buffer
	info, err := r.Copy(name, &data)
	if err != nil || info == nil {
		return nil, nil, err
	}
	return data.Bytes(), info, nil
}

// Copy streams to dst and then verifies the file. Callers must discard written
// bytes on failure. Missing is allowed only at the initial path lookup.
func (r Reader) Copy(name string, dst io.Writer) (os.FileInfo, error) {
	if r.Lstat == nil {
		r.Lstat = os.Lstat
	}
	if r.Open == nil {
		r.Open = os.Open
	}
	before, err := r.Lstat(name)
	if r.AllowMissing && os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() {
		return nil, ErrNotRegular
	}
	if r.tooLarge(before.Size()) {
		return nil, ErrTooLarge
	}

	file, err := r.Open(name)
	if err != nil {
		return nil, err
	}
	opened, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	if !opened.Mode().IsRegular() || !os.SameFile(before, opened) {
		file.Close()
		return nil, ErrChangedOpening
	}
	if r.tooLarge(opened.Size()) {
		file.Close()
		return nil, ErrTooLarge
	}
	var source io.Reader = file
	if r.MaxBytes > 0 && r.MaxBytes < math.MaxInt64 {
		source = io.LimitReader(file, r.MaxBytes+1)
	}
	count, readErr := io.Copy(dst, source)
	afterOpened, statErr := file.Stat()
	closeErr := file.Close()
	afterPath, pathErr := r.Lstat(name)
	if readErr != nil {
		return nil, readErr
	}
	if statErr != nil {
		return nil, statErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if r.tooLarge(count) {
		return nil, ErrTooLarge
	}
	if pathErr != nil || afterPath.Mode()&os.ModeSymlink != 0 ||
		!os.SameFile(opened, afterOpened) || !os.SameFile(afterOpened, afterPath) ||
		count != opened.Size() || !SameMetadata(opened, afterOpened) || !SameMetadata(afterOpened, afterPath) {
		return nil, ErrChangedReading
	}
	return afterOpened, nil
}

func (r Reader) tooLarge(size int64) bool {
	return size < 0 || (r.MaxBytes > 0 && size > r.MaxBytes)
}

// SameMetadata compares the metadata that must remain stable during a read.
func SameMetadata(before, after os.FileInfo) bool {
	return before.Mode() == after.Mode() && before.Size() == after.Size() && before.ModTime() == after.ModTime()
}
