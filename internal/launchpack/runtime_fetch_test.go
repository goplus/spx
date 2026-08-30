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

package launchpack

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/goplus/spx/v3/internal/runtimebundle"
)

func TestFetchReleaseURLClassifiesUnavailableRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	if err := fetchRuntimeURL(context.Background(), server.URL, io.Discard); !errors.Is(err, errReleaseUnavailable) {
		t.Fatalf("fetch error = %v, want release unavailable", err)
	}
}

func TestFetchReleaseURLPreservesCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := fetchRuntimeURL(ctx, "https://example.invalid/runtime", io.Discard)
	if !errors.Is(err, context.Canceled) || errors.Is(err, errReleaseUnavailable) {
		t.Fatalf("canceled fetch error = %v", err)
	}
}

func TestFetchReleaseURLPreservesDestinationFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(response, "payload")
	}))
	defer server.Close()
	want := errors.New("destination rejected payload")
	err := fetchRuntimeURL(context.Background(), server.URL, errorWriter{err: want})
	if !errors.Is(err, want) || errors.Is(err, errReleaseUnavailable) {
		t.Fatalf("destination failure = %v", err)
	}
}

func TestRuntimeReleaseUnavailableClassification(t *testing.T) {
	for _, err := range []error{
		io.ErrShortWrite,
		runtimebundle.ErrDigestMismatch,
		runtimebundle.ErrInvalidManifest,
		runtimebundle.ErrArchiveLimit,
		runtimebundle.ErrUnsafeArchive,
		runtimebundle.ErrUnsupportedArchiveEntry,
	} {
		if runtimeReleaseUnavailable(err) {
			t.Fatalf("integrity error %v classified as unavailable", err)
		}
	}
	if !runtimeReleaseUnavailable(errReleaseUnavailable) || !runtimeReleaseUnavailable(runtimebundle.ErrOfflineCacheMiss) {
		t.Fatal("availability errors were not classified for source fallback")
	}
}

type errorWriter struct{ err error }

func (w errorWriter) Write([]byte) (int, error) { return 0, w.err }
