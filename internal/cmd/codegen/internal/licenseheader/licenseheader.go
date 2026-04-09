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

package licenseheader

import "bytes"

const Phrase = "Copyright (c) 2021 The XGo Authors (xgo.dev). All rights reserved."

const Text = `/*
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

`

var headerBytes = []byte(Text)

func AddToGoSource(src []byte) []byte {
	if Has(src) {
		return src
	}

	buildPrefix, rest := splitBuildTagPrefix(src)

	out := make([]byte, 0, len(buildPrefix)+len(headerBytes)+len(rest))
	out = append(out, buildPrefix...)
	out = append(out, headerBytes...)
	out = append(out, rest...)
	return out
}

func Has(src []byte) bool {
	limit := src
	if len(limit) > 1024 {
		limit = limit[:1024]
	}
	return bytes.Contains(limit, []byte(Phrase))
}

func splitBuildTagPrefix(src []byte) ([]byte, []byte) {
	lines := bytes.SplitAfter(src, []byte("\n"))
	if len(lines) == 0 {
		return nil, src
	}

	idx := 0
	i := 0
	for i < len(lines) {
		line := bytes.TrimRight(lines[i], "\r\n")
		if bytes.HasPrefix(line, []byte("//go:build")) || bytes.HasPrefix(line, []byte("// +build")) {
			idx += len(lines[i])
			i++
			continue
		}
		break
	}
	if idx == 0 {
		return nil, src
	}

	prefix := append([]byte(nil), src[:idx]...)
	if i < len(lines) && len(bytes.TrimSpace(lines[i])) == 0 {
		prefix = append(prefix, lines[i]...)
		idx += len(lines[i])
	} else {
		prefix = append(prefix, '\n')
	}
	return prefix, src[idx:]
}
