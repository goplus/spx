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

package event

import (
	"strconv"
	"testing"
)

func BenchmarkManagerRegistration(b *testing.B) {
	for _, method := range []string{"Add", "TryAddStart"} {
		b.Run(method, func(b *testing.B) {
			for _, size := range []int{1, 16, 256, 1024} {
				b.Run(strconv.Itoa(size), func(b *testing.B) {
					b.ReportAllocs()
					for b.Loop() {
						var mgr Manager
						sink := Sink{Owner: "sprite"}
						for range size {
							if method == "Add" {
								mgr.Add(BucketClick, sink)
							} else {
								mgr.TryAddStart(sink)
							}
						}
					}
				})
			}
		})
	}
}
