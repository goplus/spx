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
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// ErrInsecureRedirect reports an HTTP redirect that weakens transport security.
var ErrInsecureRedirect = errors.New("buildctl: insecure redirect")

// GetURL performs a GET without allowing an HTTPS redirect to downgrade.
func GetURL(base *http.Client, rawURL string) (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}

	if base == nil {
		base = http.DefaultClient
	}
	if base == nil {
		base = &http.Client{}
	}
	client := *base
	originalCheckRedirect := client.CheckRedirect
	lastURL := cloneURL(req.URL)
	client.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if err := validateHTTPSRedirects(next, via); err != nil {
			return err
		}

		if originalCheckRedirect != nil {
			if err := originalCheckRedirect(next, via); err != nil {
				return err
			}
		} else if len(via) >= 10 {
			return errors.New("stopped after 10 redirects")
		}

		if err := validateHTTPSRedirects(next, via); err != nil {
			return err
		}
		if err := validateHTTPSHop(lastURL, next.URL); err != nil {
			return err
		}
		lastURL = cloneURL(next.URL)
		return nil
	}

	resp, err := client.Do(req)
	if err != nil {
		return resp, err
	}
	if resp.Request == nil || resp.Request.URL == nil {
		closeResponseBody(resp)
		return nil, fmt.Errorf("%w: download response is missing its final URL", ErrInsecureRedirect)
	}
	if err := validateHTTPSHop(lastURL, resp.Request.URL); err != nil {
		closeResponseBody(resp)
		return nil, err
	}
	return resp, nil
}

func validateHTTPSRedirects(next *http.Request, via []*http.Request) error {
	if next == nil || next.URL == nil {
		return fmt.Errorf("%w: download redirect is missing its URL", ErrInsecureRedirect)
	}
	if len(via) == 0 {
		return nil
	}
	for i := 1; i < len(via); i++ {
		if err := validateHTTPSHop(requestURL(via[i-1]), requestURL(via[i])); err != nil {
			return err
		}
	}
	return validateHTTPSHop(requestURL(via[len(via)-1]), next.URL)
}

func validateHTTPSHop(from, to *url.URL) error {
	if from == nil || to == nil {
		return fmt.Errorf("%w: download redirect is missing its URL", ErrInsecureRedirect)
	}
	if strings.EqualFold(from.Scheme, "https") && !strings.EqualFold(to.Scheme, "https") {
		return fmt.Errorf("%w: refusing HTTPS downgrade from %s to %s", ErrInsecureRedirect, from.Redacted(), to.Redacted())
	}
	return nil
}

func requestURL(req *http.Request) *url.URL {
	if req == nil {
		return nil
	}
	return req.URL
}

func cloneURL(value *url.URL) *url.URL {
	if value == nil {
		return nil
	}
	clone := *value
	return &clone
}

func closeResponseBody(resp *http.Response) {
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
}
