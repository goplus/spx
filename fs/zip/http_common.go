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
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"
)

const (
	// MaxRemoteZipBytes bounds the amount of data accepted from an hzip/hzips
	// response. The limit applies even when the server omits Content-Length.
	MaxRemoteZipBytes int64 = 512 << 20
)

var (
	// remoteHTTPClient is a package test hook. A nil value follows
	// http.DefaultClient at request time, preserving the standard library's
	// transport, proxy, cookie jar, and redirect configuration.
	remoteHTTPClient  *http.Client
	remoteHTTPTimeout = 5 * time.Minute
	// maxRemoteZipBytes is kept separate from the public default to make the
	// bound straightforward to exercise in tests.
	maxRemoteZipBytes int64 = MaxRemoteZipBytes

	ErrInvalidRemoteURL = errors.New("zip: invalid remote URL")
	ErrHTTPStatus       = errors.New("zip: unexpected HTTP status")
	ErrRemoteSizeLimit  = errors.New("zip: remote archive too large")
	ErrInsecureRedirect = errors.New("zip: insecure HTTPS redirect")
)

type remoteHTTPStatusError struct {
	URL        string
	StatusCode int
	Status     string
}

func (e *remoteHTTPStatusError) Error() string {
	if e.Status != "" {
		return fmt.Sprintf("zip: GET %s: unexpected HTTP status %s", e.URL, e.Status)
	}
	return fmt.Sprintf("zip: GET %s: unexpected HTTP status %d", e.URL, e.StatusCode)
}

func (e *remoteHTTPStatusError) Unwrap() error { return ErrHTTPStatus }

func remoteMaxBytes() int64 {
	if maxRemoteZipBytes > 0 {
		return maxRemoteZipBytes
	}
	return MaxRemoteZipBytes
}

// parseRemoteURL accepts the historical hzip form (host/path) and turns it
// into an absolute URL. The original URL is never used as a filesystem path.
func parseRemoteURL(raw, schema string) (remote, cacheKey string, err error) {
	var expectedScheme string
	switch schema {
	case "http://":
		expectedScheme = "http"
	case "https://":
		expectedScheme = "https"
	default:
		return "", "", fmt.Errorf("%w: unsupported transport %q", ErrInvalidRemoteURL, schema)
	}
	authority := raw
	if query := strings.IndexAny(authority, "?#"); query >= 0 {
		authority = authority[:query]
	}
	if raw == "" || strings.Contains(authority, "://") || strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, `\`) {
		return "", "", fmt.Errorf("%w: expected host/path, got %q", ErrInvalidRemoteURL, raw)
	}
	if strings.ContainsAny(raw, `\`) || strings.Contains(raw, "#") {
		return "", "", fmt.Errorf("%w: unsupported path character in %q", ErrInvalidRemoteURL, raw)
	}
	for _, r := range raw {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return "", "", fmt.Errorf("%w: control or whitespace character in URL", ErrInvalidRemoteURL)
		}
	}

	parsed, parseErr := url.Parse(schema + raw)
	if parseErr != nil {
		return "", "", fmt.Errorf("%w: %v", ErrInvalidRemoteURL, parseErr)
	}
	if parsed.Scheme != expectedScheme || parsed.Opaque != "" || parsed.Host == "" || parsed.User != nil || parsed.Fragment != "" {
		return "", "", fmt.Errorf("%w: URL must contain only a host, path, and query", ErrInvalidRemoteURL)
	}
	host := parsed.Hostname()
	if host == "" || host == "." || host == ".." {
		return "", "", fmt.Errorf("%w: missing or invalid host", ErrInvalidRemoteURL)
	}
	for _, r := range host {
		if unicode.IsControl(r) || unicode.IsSpace(r) {
			return "", "", fmt.Errorf("%w: invalid host", ErrInvalidRemoteURL)
		}
	}

	decodedPath, pathErr := url.PathUnescape(parsed.EscapedPath())
	if pathErr != nil {
		return "", "", fmt.Errorf("%w: invalid escaped path: %v", ErrInvalidRemoteURL, pathErr)
	}
	if strings.ContainsAny(decodedPath, `\`) {
		return "", "", fmt.Errorf("%w: backslash in path", ErrInvalidRemoteURL)
	}
	for _, r := range decodedPath {
		if unicode.IsControl(r) {
			return "", "", fmt.Errorf("%w: control character in path", ErrInvalidRemoteURL)
		}
	}
	for _, segment := range strings.Split(decodedPath, "/") {
		if segment == "." || segment == ".." {
			return "", "", fmt.Errorf("%w: path traversal segment in URL", ErrInvalidRemoteURL)
		}
	}

	// Host names are case-insensitive. Keeping one canonical representation
	// makes equivalent URLs share a cache entry while retaining the protocol in
	// the key so hzip and hzips can never collide.
	canonicalURL := *parsed
	canonicalURL.Host = strings.ToLower(parsed.Host)
	remote = canonicalURL.String()
	digest := sha256.Sum256([]byte(remote))
	return remote, hex.EncodeToString(digest[:]), nil
}

func checkRemoteHTTPResponse(resp *http.Response, remote string) error {
	if resp == nil {
		return fmt.Errorf("zip: nil HTTP response")
	}
	if resp.StatusCode != http.StatusOK {
		status := resp.Status
		if status == "" {
			status = http.StatusText(resp.StatusCode)
		}
		return &remoteHTTPStatusError{URL: remote, StatusCode: resp.StatusCode, Status: status}
	}
	if resp.ContentLength > remoteMaxBytes() {
		return fmt.Errorf("%w: response length %d exceeds limit %d", ErrRemoteSizeLimit, resp.ContentLength, remoteMaxBytes())
	}
	return nil
}

func readRemoteBody(resp *http.Response) ([]byte, error) {
	if err := checkRemoteHTTPResponse(resp, "remote archive"); err != nil {
		return nil, err
	}
	if resp.Body == nil {
		return nil, fmt.Errorf("zip: HTTP response has no body")
	}
	limit := remoteMaxBytes()
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("zip: read remote archive: %w", err)
	}
	if int64(len(body)) > limit {
		return nil, fmt.Errorf("%w: response exceeds limit %d", ErrRemoteSizeLimit, limit)
	}
	return body, nil
}

func remoteClientForScheme(client *http.Client, schema string) *http.Client {
	if client == nil {
		client = http.DefaultClient
	}
	if client == nil {
		client = &http.Client{}
	}
	clone := *client
	timeout := remoteHTTPTimeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}
	if clone.Timeout <= 0 || clone.Timeout > timeout {
		clone.Timeout = timeout
	}
	if schema != "https://" {
		return &clone
	}
	previous := clone.CheckRedirect
	clone.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if req == nil || req.URL == nil || req.URL.Scheme != "https" {
			var target any
			if req != nil {
				target = req.URL
			}
			return fmt.Errorf("%w: HTTPS URL redirected to %q", ErrInsecureRedirect, target)
		}
		if previous != nil {
			if err := previous(req, via); err != nil {
				return err
			}
			if req.URL == nil || req.URL.Scheme != "https" {
				return fmt.Errorf("%w: redirect callback selected %q", ErrInsecureRedirect, req.URL)
			}
			return nil
		}
		if len(via) >= 10 {
			return fmt.Errorf("%w: stopped after 10 redirects", http.ErrUseLastResponse)
		}
		return nil
	}
	return &clone
}

func checkFinalRemoteScheme(resp *http.Response, schema string) error {
	if schema != "https://" || resp == nil || resp.Request == nil || resp.Request.URL == nil {
		return nil
	}
	if resp.Request.URL.Scheme != "https" {
		return fmt.Errorf("%w: HTTPS URL resolved to %q", ErrInsecureRedirect, resp.Request.URL)
	}
	return nil
}
