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

package fsutil_test

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/goplus/spx/v4/fs/fsutil"
)

func TestOpenFile(t *testing.T) {
	body := &trackedBody{Reader: strings.NewReader("hello world")}
	setFileTransport(t, func(req *http.Request) (*http.Response, error) {
		if req.Method != http.MethodGet || req.URL.String() != "https://example.test/data.txt" {
			t.Fatalf("request = %s %s, want GET https://example.test/data.txt", req.Method, req.URL)
		}
		return &http.Response{StatusCode: http.StatusOK, Body: body}, nil
	})
	file, err := fsutil.OpenFile("https://example.test/data.txt")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = file.Close() })
	if !body.closed {
		t.Fatal("OpenFile did not close the HTTP body")
	}
	if data, err := io.ReadAll(file); err != nil || string(data) != "hello world" {
		t.Fatalf("ReadAll = (%q, %v), want hello world", data, err)
	}
	if offset, err := file.Seek(-5, io.SeekEnd); err != nil || offset != 6 {
		t.Fatalf("Seek = (%d, %v), want (6, nil)", offset, err)
	}
	if data, err := io.ReadAll(file); err != nil || string(data) != "world" {
		t.Fatalf("ReadAll after Seek = (%q, %v), want world", data, err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("Close = %v", err)
	}
}

func TestOpenFileErrors(t *testing.T) {
	wantErr := errors.New("read failed")
	t.Run("transport", func(t *testing.T) {
		setFileTransport(t, func(*http.Request) (*http.Response, error) { return nil, wantErr })
		if file, err := fsutil.OpenFile("https://example.test/data.txt"); file != nil || !errors.Is(err, wantErr) {
			t.Fatalf("OpenFile = (%v, %v), want transport error", file, err)
		}
	})
	t.Run("body", func(t *testing.T) {
		body := &trackedBody{Reader: failingReader{wantErr}}
		setFileTransport(t, func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: body}, nil
		})
		if file, err := fsutil.OpenFile("https://example.test/data.txt"); file != nil || !errors.Is(err, wantErr) {
			t.Fatalf("OpenFile = (%v, %v), want body error", file, err)
		}
		if !body.closed {
			t.Fatal("OpenFile did not close the failed HTTP body")
		}
	})
}

type fileTransport func(*http.Request) (*http.Response, error)

func (f fileTransport) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func setFileTransport(t *testing.T, transport fileTransport) {
	t.Helper()
	original := http.DefaultClient
	http.DefaultClient = &http.Client{Transport: transport}
	t.Cleanup(func() { http.DefaultClient = original })
}

type trackedBody struct {
	io.Reader
	closed bool
}

func (b *trackedBody) Close() error {
	b.closed = true
	return nil
}

type failingReader struct{ err error }

func (r failingReader) Read([]byte) (int, error) { return 0, r.err }
