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

type StopKind int

const (
	AllStop              StopKind = -3
	AllOtherScripts      StopKind = -100
	AllSprites           StopKind = -101
	ThisSprite           StopKind = -102
	ThisScript           StopKind = -103
	OtherScriptsInSprite StopKind = -104
)

type StopFilter func(obj any, isCurrent bool) bool

func ResolveStop(kind StopKind, owner any, isSprite func(any) bool, isGame func(any) bool) (StopFilter, bool) {
	switch kind {
	case AllSprites:
		return func(obj any, isCurrent bool) bool {
			return isSprite(obj)
		}, false
	case ThisSprite:
		return func(obj any, isCurrent bool) bool {
			return obj == owner
		}, false
	case OtherScriptsInSprite:
		return func(obj any, isCurrent bool) bool {
			return obj == owner && !isCurrent
		}, false
	case AllOtherScripts:
		return func(obj any, isCurrent bool) bool {
			return !isCurrent && (isSprite(obj) || isGame(obj))
		}, false
	case AllStop:
		return func(obj any, isCurrent bool) bool {
			return isSprite(obj) || isGame(obj)
		}, true
	case ThisScript:
		return nil, true
	default:
		return nil, false
	}
}
