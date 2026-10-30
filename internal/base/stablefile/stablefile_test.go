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

package stablefile

import (
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestReadAtSizeLimit(t *testing.T) {
	name := filepath.Join(t.TempDir(), "value")
	const content = "stable contents"
	if err := os.WriteFile(name, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, limit := range []int64{0, int64(len(content)), math.MaxInt64} {
		t.Run(fmt.Sprint(limit), func(t *testing.T) {
			data, info, err := (Reader{MaxBytes: limit}).Read(name)
			if err != nil || string(data) != content || info == nil || info.Size() != int64(len(content)) {
				t.Fatalf("Read = %q, %v, %v; want original contents and metadata", data, info, err)
			}
		})
	}
}

func TestReadRejectsInvalidOrReplacedFiles(t *testing.T) {
	for _, test := range []struct {
		name  string
		setup func(*Reader, string) error
		want  error
	}{
		{"missing", func(_ *Reader, name string) error { return os.Remove(name) }, os.ErrNotExist},
		{"optional missing", func(r *Reader, name string) error {
			r.AllowMissing = true
			return os.Remove(name)
		}, nil},
		{"directory", func(_ *Reader, name string) error {
			if err := os.Remove(name); err != nil {
				return err
			}
			return os.Mkdir(name, 0o700)
		}, ErrNotRegular},
		{"symlink", func(_ *Reader, name string) error {
			if err := os.Rename(name, name+".target"); err != nil {
				return err
			}
			return os.Symlink(name+".target", name)
		}, ErrNotRegular},
		{"too large", func(r *Reader, _ string) error { r.MaxBytes = 3; return nil }, ErrTooLarge},
		{"lookup denied", func(r *Reader, _ string) error {
			r.Lstat = func(string) (os.FileInfo, error) { return nil, os.ErrPermission }
			return nil
		}, os.ErrPermission},
		{"disappears before open", func(r *Reader, _ string) error {
			r.AllowMissing = true
			r.Open = func(name string) (*os.File, error) {
				if err := os.Remove(name); err != nil {
					return nil, err
				}
				return os.Open(name)
			}
			return nil
		}, os.ErrNotExist},
		{"replaced before open", func(r *Reader, name string) error {
			if err := os.WriteFile(name+".new", []byte("data"), 0o600); err != nil {
				return err
			}
			r.Open = func(name string) (*os.File, error) { return os.Open(name + ".new") }
			return nil
		}, ErrChangedOpening},
		{"grows before open", func(r *Reader, _ string) error {
			r.MaxBytes = 4
			r.Open = func(name string) (*os.File, error) {
				if err := os.WriteFile(name, []byte("larger"), 0o600); err != nil {
					return nil, err
				}
				return os.Open(name)
			}
			return nil
		}, ErrTooLarge},
	} {
		t.Run(test.name, func(t *testing.T) {
			name := filepath.Join(t.TempDir(), "value")
			if err := os.WriteFile(name, []byte("data"), 0o600); err != nil {
				t.Fatal(err)
			}
			var reader Reader
			if err := test.setup(&reader, name); err != nil {
				if test.name == "symlink" {
					t.Skipf("symlinks unavailable: %v", err)
				}
				t.Fatal(err)
			}
			data, info, err := reader.Read(name)
			if !errors.Is(err, test.want) || data != nil || info != nil {
				t.Fatalf("Read = %q, %v, %v; want no data or metadata and error %v", data, info, err, test.want)
			}
		})
	}
}

func TestReadRejectsIdentityChangedSinceCollection(t *testing.T) {
	dir := t.TempDir()
	name := filepath.Join(dir, "value")
	if err := os.WriteFile(name, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}
	expected, err := os.Lstat(name)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(name, name+".old"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte("replacement"), 0o600); err != nil {
		t.Fatal(err)
	}

	data, info, err := (Reader{Expected: expected}).Read(name)
	if !errors.Is(err, ErrChangedOpening) || data != nil || info != nil {
		t.Fatalf("Read = %q, %v, %v; want identity change error", data, info, err)
	}
}

func TestCopyRejectsChangesDuringWrite(t *testing.T) {
	writeErr := errors.New("destination failed")
	for _, test := range []struct {
		name   string
		mutate func(string) error
		want   error
	}{
		{"destination failure", func(string) error { return writeErr }, writeErr},
		{"path removed", os.Remove, ErrChangedReading},
		{"path replaced", func(name string) error {
			if err := os.Rename(name, name+".old"); err != nil {
				return err
			}
			return os.WriteFile(name, []byte("data"), 0o600)
		}, ErrChangedReading},
		{"metadata changed", func(name string) error {
			changed := time.Unix(1, 0)
			return os.Chtimes(name, changed, changed)
		}, ErrChangedReading},
		{"grows beyond limit", func(name string) error {
			file, err := os.OpenFile(name, os.O_WRONLY|os.O_APPEND, 0)
			if err != nil {
				return err
			}
			_, err = file.WriteString("extra")
			return errors.Join(err, file.Close())
		}, ErrTooLarge},
	} {
		t.Run(test.name, func(t *testing.T) {
			if runtime.GOOS == "windows" && (test.name == "path removed" || test.name == "path replaced") {
				t.Skip("Windows does not allow removing or replacing an open file")
			}
			name := filepath.Join(t.TempDir(), "value")
			if err := os.WriteFile(name, []byte("data"), 0o600); err != nil {
				t.Fatal(err)
			}
			mutated := false
			dst := writerFunc(func(data []byte) (int, error) {
				if !mutated {
					mutated = true
					if err := test.mutate(name); err != nil {
						return 0, err
					}
				}
				return io.Discard.Write(data)
			})
			info, err := (Reader{MaxBytes: 4}).Copy(name, dst)
			if !errors.Is(err, test.want) || info != nil {
				t.Fatalf("Copy = %v, %v; want no metadata and error %v", info, err, test.want)
			}
		})
	}
}

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(data []byte) (int, error) { return f(data) }
