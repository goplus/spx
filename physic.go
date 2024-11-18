/*
 * Copyright (c) 2021 The GoPlus Authors (goplus.org). All rights reserved.
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

package spx

const (
	physicColliderNone   = 0x00
	physicColliderAuto   = 0x01
	physicColliderCircle = 0x02
	physicColliderRect   = 0x03
)

func parseDefaultValue(pval *int64, defaultValue int64) int64 {
	if pval == nil {
		return defaultValue
	}
	return *pval
}
func parseLayerMaskValue(pval *int64) int64 {
	return parseDefaultValue(pval, 1)
}
func paserColliderType(typeName string, defaultValue int64) int64 {
	switch typeName {
	case "none":
		return physicColliderNone
	case "auto":
		return physicColliderAuto
	case "circle":
		return physicColliderCircle
	case "rect":
		return physicColliderRect
	}
	return defaultValue
}
