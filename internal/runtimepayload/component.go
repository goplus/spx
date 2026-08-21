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

package runtimepayload

import (
	"fmt"
	"io"
	"strings"

	"github.com/goplus/spx/v3/internal/runtimebundle"
)

// ComponentBundleReaderAt derives a namespace-qualified runtimebundle identity
// without materializing the archive bytes in memory.
func ComponentBundleReaderAt(reader io.ReaderAt, size int64, namespace runtimebundle.Namespace) (runtimebundle.Bundle, error) {
	bundle, err := runtimebundle.VerifyZipReader(reader, size)
	if err != nil {
		return runtimebundle.Bundle{}, err
	}
	bundle.Namespace = namespace
	return bundle.WithDigest()
}

// ComponentBundleSources derives the identity of the canonical component ZIP
// represented by sources without first constructing that ZIP in memory.
func ComponentBundleSources(sources []FileSource, namespace runtimebundle.Namespace) (runtimebundle.Bundle, error) {
	prepared, err := prepareSources(sources, "")
	if err != nil {
		return runtimebundle.Bundle{}, err
	}
	entries := make([]runtimebundle.Entry, len(prepared))
	for i := range prepared {
		entries[i] = prepared[i].entry
	}
	return componentBundleFromEntries(entries, "", namespace)
}

func componentBundleFromEntries(entries []runtimebundle.Entry, prefix string, namespace runtimebundle.Namespace) (runtimebundle.Bundle, error) {
	component := make([]runtimebundle.Entry, 0, len(entries))
	for _, entry := range entries {
		if prefix != "" && !strings.HasPrefix(entry.Name, prefix) {
			continue
		}
		entry.Name = strings.TrimPrefix(entry.Name, prefix)
		component = append(component, entry)
	}
	if len(component) == 0 {
		return runtimebundle.Bundle{}, fmt.Errorf("runtimepayload: empty %s component", strings.TrimSuffix(prefix, "/"))
	}
	bundle := runtimebundle.Bundle{
		Schema: runtimebundle.SchemaV1, Namespace: namespace, Entries: component,
	}
	return bundle.WithDigest()
}
