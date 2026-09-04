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

package shared

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

func TestGetURLPreservesClientForHTTPRedirects(t *testing.T) {
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
	resp, err := GetURL(client, server.URL)
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
		t.Fatal("GetURL modified its input client")
	}
}

func TestGetURLRetainsDefaultRedirectLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		http.Redirect(w, req, "/next", http.StatusFound)
	}))
	defer server.Close()

	resp, err := GetURL(&http.Client{}, server.URL)
	if resp != nil {
		closeResponseBody(resp)
	}
	if err == nil || !strings.Contains(err.Error(), "stopped after 10 redirects") {
		t.Fatalf("GetURL error = %v, want default redirect limit", err)
	}
}

func TestGetURLRechecksRedirectAfterCustomCallback(t *testing.T) {
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
	resp, err := GetURL(client, server.URL)
	if resp != nil {
		_ = resp.Body.Close()
	}
	if !errors.Is(err, ErrInsecureRedirect) {
		t.Fatalf("GetURL error = %v, want HTTPS downgrade rejection", err)
	}
	if !callbackCalled {
		t.Fatal("custom CheckRedirect was not called")
	}
}

func TestGetURLChecksFinalResponseURL(t *testing.T) {
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

	resp, err := GetURL(client, "https://example.invalid/archive.zip")
	if resp != nil {
		_ = resp.Body.Close()
	}
	if !errors.Is(err, ErrInsecureRedirect) {
		t.Fatalf("GetURL error = %v, want final URL downgrade rejection", err)
	}
}

func TestGetURLRejectsMissingFinalURLWithoutBody(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Request:    &http.Request{},
		}, nil
	})}

	resp, err := GetURL(client, "https://example.invalid/archive.zip")
	if resp != nil {
		closeResponseBody(resp)
	}
	if !errors.Is(err, ErrInsecureRedirect) {
		t.Fatalf("GetURL error = %v, want missing final URL rejection", err)
	}
}
