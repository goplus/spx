/*
 * Copyright (c) 2021 The XGo Authors (xgo.dev). All rights reserved.
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
	"bytes"
	"errors"
	"flag"
	"strings"
	"testing"
)

func TestParseNoArgs(t *testing.T) {
	var output bytes.Buffer
	if err := ParseNoArgs("doctor", "Usage: buildctl doctor", nil, &output); err != nil {
		t.Fatalf("ParseNoArgs returned error: %v", err)
	}
	if err := ParseNoArgs("doctor", "Usage: buildctl doctor", []string{"extra"}, &output); !errors.Is(err, ErrUsage) {
		t.Fatalf("ParseNoArgs extra argument error = %v, want ErrUsage", err)
	}
	if !strings.Contains(output.String(), "Usage: buildctl doctor") {
		t.Fatalf("ParseNoArgs did not print usage: %q", output.String())
	}
	if err := ParseNoArgs("doctor", "Usage: buildctl doctor", []string{"--help"}, &output); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("ParseNoArgs --help error = %v, want flag.ErrHelp", err)
	}
}
