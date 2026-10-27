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
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

type remoteRoundTripper func(*http.Request) (*http.Response, error)

type remoteErrorReader struct{ err error }

type remoteTrackedBody struct {
	io.Reader
	closes int
}

func TestParseRemoteURLRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name, raw, scheme string
	}{
		{"transport", "example.test/a.zip", "ftp://"},
		{"empty", "", "http://"},
		{"absolute URL", "http://example.test/a.zip", "http://"},
		{"absolute path", "/example.test/a.zip", "http://"},
		{"backslash prefix", `\example.test/a.zip`, "http://"},
		{"backslash", `example.test/a\b.zip`, "http://"},
		{"fragment", "example.test/a.zip#fragment", "http://"},
		{"space", "example.test/a b.zip", "http://"},
		{"control", "example.test/a\x00.zip", "http://"},
		{"invalid escape", "example.test/%zz", "http://"},
		{"invalid port", "example.test:port/a.zip", "http://"},
		{"credentials", "user:password@example.test/a.zip", "http://"},
		{"missing host", ":80/a.zip", "http://"},
		{"dot host", "./a.zip", "http://"},
		{"parent host", "../a.zip", "http://"},
		{"escaped host whitespace", "example%C2%A0.test/a.zip", "http://"},
		{"escaped backslash", "example.test/a%5Cb.zip", "http://"},
		{"escaped control", "example.test/a%00.zip", "http://"},
		{"dot segment", "example.test/a/./b.zip", "http://"},
		{"escaped parent", "example.test/a/%2e%2E/b.zip", "https://"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			remote, key, err := parseRemoteURL(test.raw, test.scheme)
			if !errors.Is(err, ErrInvalidRemoteURL) || remote != "" || key != "" {
				t.Fatalf("parseRemoteURL(%q, %q) = (%q, %q, %v), want empty results and ErrInvalidRemoteURL", test.raw, test.scheme, remote, key, err)
			}
		})
	}
}

func TestParseRemoteURLCanonicalizesURLsAndCacheKeys(t *testing.T) {
	tests := []struct {
		raw, scheme, want string
	}{
		{"EXAMPLE.test:8080/a%20b.zip?next=https://other.test/a&token=1", "http://", "http://example.test:8080/a%20b.zip?next=https://other.test/a&token=1"},
		{"[::1]:8443/a.zip?token=2", "https://", "https://[::1]:8443/a.zip?token=2"},
		{"example.test", "https://", "https://example.test"},
		{"EXAMPLE.test/a.zip?token=1", "http://", "http://example.test/a.zip?token=1"},
		{"example.test/a.zip?token=1", "https://", "https://example.test/a.zip?token=1"},
		{"example.test/a.zip?token=2", "https://", "https://example.test/a.zip?token=2"},
	}
	keys := make(map[string]bool)
	for _, test := range tests {
		remote, key, err := parseRemoteURL(test.raw, test.scheme)
		if err != nil || remote != test.want {
			t.Fatalf("parseRemoteURL(%q) = (%q, %v), want %q", test.raw, remote, err, test.want)
		}
		digest := sha256.Sum256([]byte(test.want))
		if want := hex.EncodeToString(digest[:]); key != want {
			t.Fatalf("cache key = %q, want SHA-256 %q", key, want)
		}
		if keys[key] {
			t.Fatalf("distinct URLs share cache key %q", key)
		}
		keys[key] = true
	}
}

func TestCheckRemoteHTTPResponse(t *testing.T) {
	tests := []struct {
		name    string
		resp    *http.Response
		wantErr error
		message string
	}{
		{name: "nil response", message: "nil HTTP response"},
		{name: "status text", resp: &http.Response{StatusCode: http.StatusNotFound}, wantErr: ErrHTTPStatus, message: "Not Found"},
		{name: "status number", resp: &http.Response{StatusCode: 599}, wantErr: ErrHTTPStatus, message: "599"},
		{name: "explicit status", resp: &http.Response{StatusCode: http.StatusForbidden, Status: "403 access denied"}, wantErr: ErrHTTPStatus, message: "403 access denied"},
		{name: "oversized", resp: &http.Response{StatusCode: http.StatusOK, ContentLength: MaxRemoteZipBytes + 1}, wantErr: ErrRemoteSizeLimit},
		{name: "at limit", resp: &http.Response{StatusCode: http.StatusOK, ContentLength: MaxRemoteZipBytes}},
		{name: "unknown length", resp: &http.Response{StatusCode: http.StatusOK, ContentLength: -1}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := checkRemoteHTTPResponse(test.resp, "https://example.test/archive.zip")
			if test.wantErr == nil && test.message == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || test.wantErr != nil && !errors.Is(err, test.wantErr) || !strings.Contains(err.Error(), test.message) {
				t.Fatalf("checkRemoteHTTPResponse error = %v, want %v containing %q", err, test.wantErr, test.message)
			}
			if errors.Is(err, ErrHTTPStatus) {
				var status *remoteHTTPStatusError
				if !errors.As(err, &status) || status.StatusCode != test.resp.StatusCode || status.URL != "https://example.test/archive.zip" {
					t.Fatalf("HTTP status error lost response details: %#v", status)
				}
			}
		})
	}
}

func TestReadRemoteBody(t *testing.T) {
	readErr := errors.New("interrupted body")
	tests := []struct {
		name    string
		resp    *http.Response
		want    string
		wantErr error
		message string
	}{
		{name: "nil response", message: "nil HTTP response"},
		{name: "bad status", resp: &http.Response{StatusCode: http.StatusNotFound}, wantErr: ErrHTTPStatus},
		{name: "missing body", resp: &http.Response{StatusCode: http.StatusOK}, message: "no body"},
		{name: "read error", resp: &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(io.MultiReader(strings.NewReader("partial"), remoteErrorReader{readErr}))}, wantErr: readErr},
		{name: "unknown length", resp: &http.Response{StatusCode: http.StatusOK, ContentLength: -1, Body: io.NopCloser(strings.NewReader("archive bytes"))}, want: "archive bytes"},
		{name: "empty body", resp: &http.Response{StatusCode: http.StatusOK, Body: http.NoBody}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.resp != nil && test.resp.Body != nil {
				t.Cleanup(func() { _ = test.resp.Body.Close() })
			}
			body, err := readRemoteBody(test.resp)
			if test.wantErr != nil || test.message != "" {
				if err == nil || test.wantErr != nil && !errors.Is(err, test.wantErr) || !strings.Contains(err.Error(), test.message) || body != nil {
					t.Fatalf("readRemoteBody = (%q, %v), want no data and %v containing %q", body, err, test.wantErr, test.message)
				}
				return
			}
			if err != nil || string(body) != test.want {
				t.Fatalf("readRemoteBody = (%q, %v), want %q", body, err, test.want)
			}
		})
	}
}

func TestGetRemoteClosesRejectedResponses(t *testing.T) {
	for _, test := range []struct {
		name, finalScheme string
		status            int
		length            int64
		wantErr           error
	}{
		{name: "insecure final response", finalScheme: "http", status: http.StatusOK, wantErr: ErrInsecureRedirect},
		{name: "bad status", finalScheme: "https", status: http.StatusNotFound, wantErr: ErrHTTPStatus},
		{name: "oversized response", finalScheme: "https", status: http.StatusOK, length: MaxRemoteZipBytes + 1, wantErr: ErrRemoteSizeLimit},
	} {
		t.Run(test.name, func(t *testing.T) {
			body := &remoteTrackedBody{Reader: strings.NewReader("response")}
			setRemoteHTTPClient(t, &http.Client{Transport: remoteRoundTripper(func(req *http.Request) (*http.Response, error) {
				if req.URL.String() != "https://example.test/archive.zip" {
					t.Errorf("requested URL = %q", req.URL)
				}
				return &http.Response{StatusCode: test.status, ContentLength: test.length, Body: body, Request: &http.Request{URL: &url.URL{Scheme: test.finalScheme, Host: "example.test"}}}, nil
			})})
			resp, err := getRemote("https://example.test/archive.zip", "https://")
			if !errors.Is(err, test.wantErr) || resp != nil {
				t.Fatalf("getRemote = (%v, %v), want no response and %v", resp, err, test.wantErr)
			}
			if body.closes != 1 {
				t.Fatalf("rejected response body closed %d times, want 1", body.closes)
			}
		})
	}
}

func TestGetRemoteTransportError(t *testing.T) {
	wantErr := errors.New("transport unavailable")
	setRemoteHTTPClient(t, &http.Client{Transport: remoteRoundTripper(func(*http.Request) (*http.Response, error) {
		return nil, wantErr
	})})
	resp, err := getRemote("https://example.test/archive.zip", "https://")
	if resp != nil || !errors.Is(err, wantErr) {
		t.Fatalf("getRemote = (%v, %v), want no response and transport error", resp, err)
	}
}

func TestRemoteClientTimeoutAndNilDefault(t *testing.T) {
	const defaultTimeout = 5 * time.Minute
	t.Run("nil default", func(t *testing.T) {
		setRemoteHTTPClient(t, nil)
		client := remoteClientForScheme("http://")
		if client == nil || client.Timeout != defaultTimeout || http.DefaultClient != nil {
			t.Fatalf("client = %#v, default = %#v", client, http.DefaultClient)
		}
	})
	for _, test := range []struct {
		timeout, want time.Duration
	}{
		{-time.Second, defaultTimeout}, {0, defaultTimeout}, {time.Minute, time.Minute},
		{defaultTimeout, defaultTimeout}, {defaultTimeout + time.Second, defaultTimeout},
	} {
		t.Run(test.timeout.String(), func(t *testing.T) {
			original := &http.Client{Timeout: test.timeout}
			setRemoteHTTPClient(t, original)
			client := remoteClientForScheme("http://")
			if client == original || client.Timeout != test.want || original.Timeout != test.timeout {
				t.Fatalf("clone timeout = %v, original = %v, want %v and %v", client.Timeout, original.Timeout, test.want, test.timeout)
			}
		})
	}
}

func TestHTTPSRedirectPolicy(t *testing.T) {
	setRemoteHTTPClient(t, &http.Client{})
	client := remoteClientForScheme("https://")
	for _, request := range []*http.Request{nil, {}, {URL: &url.URL{Scheme: "http", Host: "example.test"}}} {
		if err := client.CheckRedirect(request, nil); !errors.Is(err, ErrInsecureRedirect) {
			t.Errorf("CheckRedirect(%v) = %v, want ErrInsecureRedirect", request, err)
		}
	}
	request := &http.Request{URL: &url.URL{Scheme: "https", Host: "example.test"}}
	if err := client.CheckRedirect(request, make([]*http.Request, 9)); err != nil {
		t.Fatalf("HTTPS redirect before limit: %v", err)
	}
	if err := client.CheckRedirect(request, make([]*http.Request, 10)); !errors.Is(err, http.ErrUseLastResponse) {
		t.Fatalf("redirect at limit = %v, want ErrUseLastResponse", err)
	}
}

func TestHTTPSRedirectPreservesAndChecksCallback(t *testing.T) {
	callbackErr := errors.New("redirect rejected by caller")
	for _, action := range []string{"accept", "reject", "downgrade", "clear URL"} {
		t.Run(action, func(t *testing.T) {
			calls := 0
			previous := &http.Request{URL: &url.URL{Scheme: "https", Host: "original.test"}}
			request := &http.Request{URL: &url.URL{Scheme: "https", Host: "redirect.test"}}
			setRemoteHTTPClient(t, &http.Client{CheckRedirect: func(req *http.Request, via []*http.Request) error {
				calls++
				if req != request || len(via) != 1 || via[0] != previous {
					t.Fatalf("redirect callback did not receive original arguments")
				}
				switch action {
				case "reject":
					return callbackErr
				case "downgrade":
					req.URL.Scheme = "http"
				case "clear URL":
					req.URL = nil
				}
				return nil
			}})
			client := remoteClientForScheme("https://")
			err := client.CheckRedirect(request, []*http.Request{previous})
			var wantErr error
			switch action {
			case "reject":
				wantErr = callbackErr
			case "downgrade", "clear URL":
				wantErr = ErrInsecureRedirect
			}
			if calls != 1 || !errors.Is(err, wantErr) {
				t.Fatalf("CheckRedirect = %v with %d callbacks, want %v and 1", err, calls, wantErr)
			}
		})
	}
}

func TestCheckFinalRemoteScheme(t *testing.T) {
	for _, response := range []*http.Response{nil, {}, {Request: &http.Request{}}, {Request: &http.Request{URL: &url.URL{Scheme: "https"}}}} {
		if err := checkFinalRemoteScheme(response, "https://"); err != nil {
			t.Errorf("checkFinalRemoteScheme(%v) = %v", response, err)
		}
	}
	response := &http.Response{Request: &http.Request{URL: &url.URL{Scheme: "http", Host: "example.test"}}}
	if err := checkFinalRemoteScheme(response, "http://"); err != nil {
		t.Fatalf("HTTP request rejected: %v", err)
	}
	if err := checkFinalRemoteScheme(response, "https://"); !errors.Is(err, ErrInsecureRedirect) {
		t.Fatalf("HTTPS downgrade error = %v, want ErrInsecureRedirect", err)
	}
}

func (t remoteRoundTripper) RoundTrip(request *http.Request) (*http.Response, error) {
	return t(request)
}

func (r remoteErrorReader) Read([]byte) (int, error) { return 0, r.err }

func (b *remoteTrackedBody) Close() error {
	b.closes++
	return nil
}

func setRemoteHTTPClient(t *testing.T, client *http.Client) {
	t.Helper()
	previous := http.DefaultClient
	http.DefaultClient = client
	t.Cleanup(func() { http.DefaultClient = previous })
}
