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

package xgodriver

import "errors"

type requestErrorMarker struct{ error }

func (e *requestErrorMarker) Unwrap() error { return e.error }

// IsRequestError reports whether err describes an invalid request or identity.
func IsRequestError(err error) bool {
	var marker *requestErrorMarker
	return errors.As(err, &marker)
}

func requestError(err error) error {
	if err == nil {
		return nil
	}
	if IsRequestError(err) {
		return err
	}
	return &requestErrorMarker{error: err}
}
