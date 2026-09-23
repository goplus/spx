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

package engine

import (
	"fmt"
	"os"
	"time"

	"github.com/goplus/spx/v3/internal/cmd/buildctl/shared"
	"github.com/goplus/spx/v3/internal/runtimebundle"
)

func fetchURLToFile(url, dst string) error {
	return fetchURLToFileWithLimit(url, dst, runtimebundle.MaxArchiveBytes)
}

func fetchURLToFileWithLimit(url, dst string, maxBytes int64) error {
	if maxBytes <= 0 {
		return fmt.Errorf("invalid download size limit %d", maxBytes)
	}
	fmt.Fprintf(os.Stdout, "Downloading %s -> %s\n", url, dst)
	lastReport := time.Now().Add(-time.Second)
	return shared.DownloadURL(url, dst, maxBytes, func(done, declared int64) {
		if time.Since(lastReport) >= 500*time.Millisecond || done == declared {
			end := "\r"
			if done == declared {
				end = "\n"
			}
			fmt.Fprintf(os.Stdout, "  %.1f%% (%s/%s)%s", float64(done)*100/float64(declared), formatDownloadSize(done), formatDownloadSize(declared), end)
			lastReport = time.Now()
		}
	})
}

func formatDownloadSize(size int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)

	switch {
	case size >= gb:
		return fmt.Sprintf("%.1fGB", float64(size)/float64(gb))
	case size >= mb:
		return fmt.Sprintf("%.1fMB", float64(size)/float64(mb))
	case size >= kb:
		return fmt.Sprintf("%.1fKB", float64(size)/float64(kb))
	default:
		return fmt.Sprintf("%dB", size)
	}
}
