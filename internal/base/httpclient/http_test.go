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

type closeTrackingBody struct {
	io.Reader
	closes   int
	closeErr error
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

func TestDoAllowsCustomRedirectLimit(t *testing.T) {
	const redirects = 12
	requests, callbacks := 0, 0
	client := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			requests++
			resp := &http.Response{StatusCode: http.StatusOK, Request: req, Body: http.NoBody}
			if requests <= redirects {
				resp.StatusCode = http.StatusFound
				resp.Header = http.Header{"Location": {"/next"}}
			}
			return resp, nil
		}),
		CheckRedirect: func(_ *http.Request, via []*http.Request) error {
			callbacks++
			if len(via) != callbacks {
				t.Errorf("redirect history length = %d, want %d", len(via), callbacks)
			}
			return nil
		},
	}
	resp, err := getURL(client, "https://example.test/start")
	if err != nil {
		closeResponseBody(resp)
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK || requests != redirects+1 || callbacks != redirects {
		t.Fatalf("status/requests/callbacks = %d/%d/%d, want 200/%d/%d", resp.StatusCode, requests, callbacks, redirects+1, redirects)
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
	for _, tc := range []struct {
		name    string
		request *http.Request
	}{
		{"missing request", nil},
		{"missing URL", &http.Request{}},
		{"downgrade", &http.Request{URL: &url.URL{Scheme: "http", Host: "example.test", Path: "/archive.zip"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := &closeTrackingBody{Reader: strings.NewReader("payload"), closeErr: errors.New("close failed")}
			client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Body: body, Request: tc.request}, nil
			})}
			resp, err := getURL(client, "https://example.test/archive.zip")
			if resp != nil || !errors.Is(err, ErrInsecureRedirect) || body.closes != 1 {
				t.Fatalf("Do = (%v, %v), closes = %d; want insecure-response error and exactly one close", resp, err, body.closes)
			}
		})
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

func TestDoUsesDefaultClientWithoutMutatingIt(t *testing.T) {
	for _, nilDefault := range []bool{false, true} {
		name := "default client"
		if nilDefault {
			name = "nil default client"
		}
		t.Run(name, func(t *testing.T) {
			previousClient, previousTransport := http.DefaultClient, http.DefaultTransport
			t.Cleanup(func() {
				http.DefaultClient, http.DefaultTransport = previousClient, previousTransport
			})
			calls := 0
			transport := roundTripFunc(func(req *http.Request) (*http.Response, error) {
				calls++
				if req.Method != http.MethodGet || req.URL.String() != "https://example.test/file" {
					t.Errorf("request = %s %s", req.Method, req.URL)
				}
				return &http.Response{StatusCode: http.StatusOK, Request: req, Body: io.NopCloser(strings.NewReader("payload"))}, nil
			})
			base := &http.Client{Transport: transport, Timeout: time.Second}
			if nilDefault {
				base = nil
			}
			http.DefaultClient, http.DefaultTransport = base, transport
			resp, err := getURL(nil, "https://example.test/file")
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			data, err := io.ReadAll(resp.Body)
			if err != nil || string(data) != "payload" || calls != 1 {
				t.Fatalf("response = %q, %v; requests = %d", data, err, calls)
			}
			if http.DefaultClient != base || base != nil && (base.Timeout != time.Second || base.CheckRedirect != nil) {
				t.Fatal("Do changed the default client")
			}
		})
	}
}

func TestDoPreservesRedirectCallbackErrorsAndBodyOwnership(t *testing.T) {
	rejected := errors.New("caller rejected redirect")
	for _, tc := range []struct {
		name                 string
		callbackErr, wantErr error
		wantCloses           int
	}{
		{"callback error", rejected, rejected, 1},
		{"use last response", http.ErrUseLastResponse, nil, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body := &closeTrackingBody{Reader: strings.NewReader("redirect body")}
			requests, callbacks := 0, 0
			client := &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					requests++
					return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": {"https://example.test/next"}}, Request: req, Body: body}, nil
				}),
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					callbacks++
					if req.URL.Path != "/next" || len(via) != 1 || via[0].URL.Path != "/start" {
						t.Errorf("callback arguments = %v, %v", req.URL, via)
					}
					return tc.callbackErr
				},
			}
			resp, err := getURL(client, "https://example.test/start")
			if !errors.Is(err, tc.wantErr) || resp == nil || resp.StatusCode != http.StatusFound {
				t.Fatalf("Do = (%v, %v), want redirect response and %v", resp, err, tc.wantErr)
			}
			if requests != 1 || callbacks != 1 || body.closes != tc.wantCloses {
				t.Fatalf("requests/callbacks/closes = %d/%d/%d, want 1/1/%d", requests, callbacks, body.closes, tc.wantCloses)
			}
			if tc.wantErr == nil {
				data, err := io.ReadAll(resp.Body)
				if err != nil || string(data) != "redirect body" {
					t.Fatalf("last response body = %q, %v", data, err)
				}
			}
		})
	}
}

func TestDoRejectsDowngradeDespiteMutableRedirectHistory(t *testing.T) {
	for _, action := range []string{"direct downgrade", "clear URL", "rewrite history"} {
		t.Run(action, func(t *testing.T) {
			requests, callbacks := 0, 0
			body := &closeTrackingBody{Reader: strings.NewReader("redirect")}
			client := &http.Client{
				Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
					requests++
					location := "https://example.test/next"
					if action == "direct downgrade" {
						location = "http://example.test/next"
					}
					return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": {location}}, Request: req, Body: body}, nil
				}),
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					callbacks++
					switch action {
					case "clear URL":
						req.URL = nil
					case "rewrite history":
						// Rewriting via must not rewrite Do's independently saved last URL.
						for _, previous := range via {
							previous.URL.Scheme = "http"
						}
						req.URL.Scheme = "http"
					}
					return nil
				},
			}
			_, err := getURL(client, "https://example.test/start")
			wantCallbacks := 1
			if action == "direct downgrade" {
				wantCallbacks = 0
			}
			if !errors.Is(err, ErrInsecureRedirect) || requests != 1 || callbacks != wantCallbacks || body.closes != 1 {
				t.Fatalf("error = %v; requests/callbacks/closes = %d/%d/%d, want insecure redirect and 1/%d/1", err, requests, callbacks, body.closes, wantCallbacks)
			}
		})
	}
}

func TestDoTracksHTTPSUpgradeThroughFinalResponse(t *testing.T) {
	for _, finalScheme := range []string{"https", "http"} {
		t.Run(finalScheme, func(t *testing.T) {
			requests := 0
			body := &closeTrackingBody{Reader: strings.NewReader("payload")}
			client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				requests++
				if requests == 1 {
					return &http.Response{StatusCode: http.StatusFound, Header: http.Header{"Location": {"https://example.test/secure"}}, Request: req, Body: http.NoBody}, nil
				}
				if req.URL.Scheme != "https" || req.URL.Path != "/secure" {
					t.Errorf("redirect request = %s", req.URL)
				}
				finalURL := cloneURL(req.URL)
				finalURL.Scheme = finalScheme
				return &http.Response{StatusCode: http.StatusOK, Request: &http.Request{URL: finalURL}, Body: body}, nil
			})}
			resp, err := getURL(client, "http://example.test/start")
			if requests != 2 {
				t.Fatalf("requests = %d, want 2", requests)
			}
			if finalScheme == "http" {
				if resp != nil || !errors.Is(err, ErrInsecureRedirect) || body.closes != 1 {
					t.Fatalf("Do = (%v, %v), closes = %d; want rejected final downgrade and closed body", resp, err, body.closes)
				}
				return
			}
			if err != nil || resp == nil || body.closes != 0 {
				t.Fatalf("Do = (%v, %v), closes = %d; want accepted upgrade and open body", resp, err, body.closes)
			}
			data, err := io.ReadAll(resp.Body)
			if err != nil || string(data) != "payload" {
				t.Fatalf("body = %q, %v", data, err)
			}
		})
	}
}

func TestDoPreservesTransportError(t *testing.T) {
	wantErr := errors.New("transport failed")
	calls := 0
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		calls++
		return nil, wantErr
	})}
	resp, err := getURL(client, "https://example.test/file")
	if resp != nil || !errors.Is(err, wantErr) || calls != 1 {
		t.Fatalf("Do = (%v, %v), requests = %d; want nil, transport error, 1", resp, err, calls)
	}
}

func TestValidateHTTPSHopRedactsPasswords(t *testing.T) {
	from := &url.URL{Scheme: "HTTPS", Host: "example.test", User: url.UserPassword("user", "source-secret")}
	to := &url.URL{Scheme: "http", Host: "example.test", User: url.UserPassword("user", "target-secret")}
	err := validateHTTPSHop(from, to)
	if !errors.Is(err, ErrInsecureRedirect) {
		t.Fatalf("validateHTTPSHop = %v, want ErrInsecureRedirect", err)
	}
	for _, secret := range []string{"source-secret", "target-secret"} {
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("redirect error exposed a password: %v", err)
		}
	}
	to.Scheme = "hTtPs"
	if err := validateHTTPSHop(from, to); err != nil {
		t.Fatalf("case-insensitive HTTPS hop rejected: %v", err)
	}
}

func TestCloseResponseBodyHandlesEmptyAndFailedBodies(t *testing.T) {
	closeResponseBody(nil)
	closeResponseBody(&http.Response{})
	body := &closeTrackingBody{Reader: strings.NewReader(""), closeErr: errors.New("close failed")}
	closeResponseBody(&http.Response{Body: body})
	if body.closes != 1 {
		t.Fatalf("Close calls = %d, want 1", body.closes)
	}
}

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func (b *closeTrackingBody) Close() error {
	b.closes++
	return b.closeErr
}

func getURL(client *http.Client, rawURL string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	return Do(client, req)
}
