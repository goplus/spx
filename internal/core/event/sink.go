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
	"math"
	"slices"
)

func NewSink(owner any, handler any, cond ...func(any) bool) Sink {
	sink := Sink{
		Owner:   owner,
		Handler: handler,
	}
	if len(cond) > 0 {
		sink.Cond = cond[0]
	}
	return sink
}

func MatchOwner(owner any) func(any) bool {
	return func(data any) bool {
		return data == owner
	}
}

func MatchOwnerOrNil(owner any) func(any) bool {
	return func(data any) bool {
		return data == nil || data == owner
	}
}

func MatchValue[T comparable](want T) func(any) bool {
	return func(data any) bool {
		got, ok := data.(T)
		return ok && got == want
	}
}

func MatchAnyOf[T comparable](values []T) func(any) bool {
	return func(data any) bool {
		got, ok := data.(T)
		if !ok {
			return false
		}
		return slices.Contains(values, got)
	}
}

func MatchApproxFloat(want, tolerance float64) func(any) bool {
	return func(data any) bool {
		got, ok := data.(float64)
		return ok && math.Abs(got-want) < tolerance
	}
}
