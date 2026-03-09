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

package spx

import (
	"github.com/goplus/spx/v2/internal/base/valueutil"
	spxlog "github.com/goplus/spx/v2/internal/log"
)

// parseLayerMaskValue parses layer mask value.
func parseLayerMaskValue(pval *int64) int64 {
	return valueutil.OrDefault(pval, 1)
}

// toRotationStyle converts a string representation to a RotationStyle constant.
func toRotationStyle(style string) RotationStyle {
	switch style {
	case "left-right":
		return LeftRight
	case "none":
		return None
	case "normal":
		return Normal
	default:
		spxlog.Warn("Unrecognized rotationStyle value '%s', using default 'Normal'.", style)
		return Normal
	}
}
