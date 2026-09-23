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

package runtime

import "github.com/goplus/spx/v3/internal/engine"

func ProcessTriggerPairs[T any](
	pairs []engine.TriggerEvent,
	resolve func(any) (T, bool),
	isTouchable func(T) bool,
	onTouch func(T, T),
	onInvalid func(),
) {
	for _, pair := range pairs {
		if pair.Src == nil || pair.Dst == nil {
			onInvalid()
			continue
		}

		src, ok1 := resolve(pair.Src.Target)
		dst, ok2 := resolve(pair.Dst.Target)
		if !ok1 || !ok2 {
			onInvalid()
			continue
		}
		if !isTouchable(src) || !isTouchable(dst) {
			continue
		}
		onTouch(src, dst)
	}
}
