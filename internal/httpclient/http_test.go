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

package httpclient

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func TestDoPreservesClientForHTTPRedirects(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("payload"))
	}))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, target.URL, http.StatusFound)
	}))
	defer server.Close()

	callbackCalls := 0
	client := &http.Client{
		Transport: http.DefaultTransport,
		Timeout:   time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			callbackCalls++
			return nil
		},
	}
	transport := client.Transport
	resp, err := getURL(client, server.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "payload" || callbackCalls != 1 {
		t.Fatalf("body = %q, callback calls = %d", data, callbackCalls)
	}
	if client.Transport != transport || client.Timeout != time.Second || client.CheckRedirect == nil {
		t.Fatal("Do modified its input client")
	}
}

func TestDoRetainsDefaultRedirectLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "/next", http.StatusFound)
	}))
	defer server.Close()

	resp, err := getURL(&http.Client{}, server.URL)
	if resp != nil {
		closeResponseBody(resp)
	}
	if err == nil || !strings.Contains(err.Error(), "stopped after 10 redirects") {
		t.Fatalf("Do error = %v, want default redirect limit", err)
	}
}

func TestDoRechecksRedirectAfterCustomCallback(t *testing.T) {
	insecure := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("downgraded redirect was followed")
	}))
	defer insecure.Close()
	insecureURL, err := url.Parse(insecure.URL)
	if err != nil {
		t.Fatal(err)
	}

	secureTarget := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("secure"))
	}))
	defer secureTarget.Close()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, secureTarget.URL, http.StatusFound)
	}))
	defer server.Close()

	client := server.Client()
	callbackCalled := false
	client.CheckRedirect = func(req *http.Request, _ []*http.Request) error {
		callbackCalled = true
		req.URL = cloneURL(insecureURL)
		return nil
	}
	resp, err := getURL(client, server.URL)
	if resp != nil {
		_ = resp.Body.Close()
	}
	if !errors.Is(err, ErrInsecureRedirect) {
		t.Fatalf("Do error = %v, want HTTPS downgrade rejection", err)
	}
	if !callbackCalled {
		t.Fatal("custom CheckRedirect was not called")
	}
}

func TestDoChecksFinalResponseURL(t *testing.T) {
	insecureURL, err := url.Parse("http://example.invalid/archive.zip")
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("payload")),
			Request:    &http.Request{URL: insecureURL},
		}, nil
	})}

	resp, err := getURL(client, "https://example.invalid/archive.zip")
	if resp != nil {
		_ = resp.Body.Close()
	}
	if !errors.Is(err, ErrInsecureRedirect) {
		t.Fatalf("Do error = %v, want final URL downgrade rejection", err)
	}
}

func TestDoRejectsMissingFinalURLWithoutBody(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Request:    &http.Request{},
		}, nil
	})}

	resp, err := getURL(client, "https://example.invalid/archive.zip")
	if resp != nil {
		closeResponseBody(resp)
	}
	if !errors.Is(err, ErrInsecureRedirect) {
		t.Fatalf("Do error = %v, want missing final URL rejection", err)
	}
}

func getURL(client *http.Client, rawURL string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	return Do(client, req)
}

func TestDoRejectsMissingRequestURL(t *testing.T) {
	for _, req := range []*http.Request{nil, {}} {
		if _, err := Do(nil, req); err == nil {
			t.Fatal("Do accepted a request without a URL")
		}
	}
}

func TestValidateHTTPSRedirects(t *testing.T) {
	request := func(rawURL string) *http.Request {
		t.Helper()
		parsed, err := url.Parse(rawURL)
		if err != nil {
			t.Fatal(err)
		}
		return &http.Request{URL: parsed}
	}

	tests := []struct {
		name    string
		next    *http.Request
		via     []*http.Request
		wantErr bool
	}{
		{name: "missing next", wantErr: true},
		{name: "no redirect history", next: request("https://example.test/next")},
		{name: "missing prior URL", next: request("https://example.test/next"), via: []*http.Request{nil}, wantErr: true},
		{
			name:    "intermediate downgrade",
			next:    request("https://example.test/final"),
			via:     []*http.Request{request("https://example.test/start"), request("http://example.test/middle")},
			wantErr: true,
		},
		{
			name:    "final downgrade",
			next:    request("http://example.test/final"),
			via:     []*http.Request{request("https://example.test/start")},
			wantErr: true,
		},
		{
			name: "secure chain",
			next: request("https://example.test/final"),
			via:  []*http.Request{request("https://example.test/start"), request("https://example.test/middle")},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateHTTPSRedirects(test.next, test.via)
			if test.wantErr {
				if !errors.Is(err, ErrInsecureRedirect) {
					t.Fatalf("validateHTTPSRedirects() error = %v, want %v", err, ErrInsecureRedirect)
				}
				return
			}
			if err != nil {
				t.Fatalf("validateHTTPSRedirects() error = %v, want nil", err)
			}
		})
	}
}

func TestURLHelpers(t *testing.T) {
	if requestURL(nil) != nil {
		t.Fatal("requestURL(nil) returned a URL")
	}
	original, err := url.Parse("https://example.test/path")
	if err != nil {
		t.Fatal(err)
	}
	clone := cloneURL(original)
	if clone == original || clone.String() != original.String() {
		t.Fatalf("cloneURL() = %p, %q; want distinct equal URL", clone, clone)
	}
	clone.Path = "/changed"
	if original.Path != "/path" {
		t.Fatalf("cloneURL shared mutable URL state: original path = %q", original.Path)
	}
	if cloneURL(nil) != nil {
		t.Fatal("cloneURL(nil) returned a URL")
	}
}
