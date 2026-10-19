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

package zip

import (
	archivezip "archive/zip"
	"bytes"
	"errors"
	"io"
	iofs "io/fs"
	"net/http"
	"strings"
	"testing"

	spxfs "github.com/goplus/spx/v3/fs"
)

func TestOpenRemoteZipSchemas(t *testing.T) {
	archive := makeWasmZip(t, "sprites/hello.txt")
	for _, tc := range []struct {
		schema string
		url    string
	}{
		{"zip", "http://example.test/project.zip"},
		{"hzip", "http://example.test/project.zip"},
		{"hzips", "https://example.test/project.zip"},
	} {
		t.Run(tc.schema, func(t *testing.T) {
			body := &remoteTrackedBody{Reader: bytes.NewReader(archive)}
			requests := 0
			setRemoteHTTPClient(t, &http.Client{Transport: remoteRoundTripper(func(req *http.Request) (*http.Response, error) {
				requests++
				if req.Method != http.MethodGet || req.URL.String() != tc.url {
					t.Fatalf("request = %s %s, want GET %s", req.Method, req.URL, tc.url)
				}
				return &http.Response{StatusCode: http.StatusOK, Body: body, Request: req}, nil
			})})
			dir, err := spxfs.Open(tc.schema + "://example.test/project.zip")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := dir.Close(); err != nil {
					t.Errorf("Close = %v", err)
				}
			})
			if requests != 1 || body.closes != 1 {
				t.Fatalf("download requests = %d, body close calls = %d; want one request and a closed body", requests, body.closes)
			}
			file, err := dir.Open("sprites/hello.txt")
			if err != nil {
				t.Fatal(err)
			}
			data, readErr := io.ReadAll(file)
			closeErr := file.Close()
			if string(data) != "contents of sprites/hello.txt" || readErr != nil || closeErr != nil {
				t.Fatalf("entry data = %q, read error = %v, close error = %v", data, readErr, closeErr)
			}
			if file, err := dir.Open("missing.txt"); file != nil || !errors.Is(err, iofs.ErrNotExist) {
				t.Fatalf("Open missing entry = (%v, %v), want fs.ErrNotExist", file, err)
			}
		})
	}
}

func TestOpenRemoteZipFailures(t *testing.T) {
	wantReadErr := errors.New("response read failed")
	tests := []struct {
		name       string
		path       string
		archive    []byte
		readErr    error
		requestErr error
		wantErr    error
		insecure   bool
	}{
		{name: "invalid URL", path: "example.test/../project.zip", wantErr: ErrInvalidRemoteURL},
		{name: "transport error", requestErr: wantReadErr, wantErr: wantReadErr},
		{name: "body error", readErr: wantReadErr, wantErr: wantReadErr},
		{name: "invalid archive", archive: []byte("not a zip"), wantErr: archivezip.ErrFormat},
		{name: "insecure reader path", archive: makeWasmZip(t, "../entry"), insecure: true, wantErr: archivezip.ErrInsecurePath},
		{name: "duplicate entries", archive: makeWasmZip(t, "entry", "entry"), wantErr: archivezip.ErrFormat},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.insecure {
				t.Setenv("GODEBUG", "zipinsecurepath=0")
			}
			var reader io.Reader = bytes.NewReader(tc.archive)
			if tc.readErr != nil {
				reader = remoteErrorReader{tc.readErr}
			}
			body := &remoteTrackedBody{Reader: reader}
			requests := 0
			setRemoteHTTPClient(t, &http.Client{Transport: remoteRoundTripper(func(req *http.Request) (*http.Response, error) {
				requests++
				if tc.requestErr != nil {
					return nil, tc.requestErr
				}
				return &http.Response{StatusCode: http.StatusOK, Body: body, Request: req}, nil
			})})
			path := tc.path
			if path == "" {
				path = "example.test/project.zip"
			}
			dir, err := Open(path)
			if dir != nil || !errors.Is(err, tc.wantErr) {
				if dir != nil {
					_ = dir.Close()
				}
				t.Fatalf("Open = (%v, %v), want %v", dir, err, tc.wantErr)
			}
			if errors.Is(tc.wantErr, ErrInvalidRemoteURL) {
				if requests != 0 {
					t.Fatalf("invalid URL made %d requests", requests)
				}
			} else if requests != 1 || tc.requestErr == nil && body.closes != 1 {
				t.Fatalf("requests = %d, body close calls = %d", requests, body.closes)
			}
			if tc.name == "duplicate entries" && !strings.Contains(err.Error(), "duplicate entry") {
				t.Fatalf("Open error = %v, want duplicate-entry validation", err)
			}
		})
	}
}

func makeWasmZip(t *testing.T, names ...string) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := archivezip.NewWriter(&buf)
	for _, name := range names {
		entry, err := writer.CreateHeader(&archivezip.FileHeader{Name: name, Method: archivezip.Store})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(entry, "contents of "+name); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
