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
	"strings"

	spxlog "github.com/goplus/spx/v2/internal/log"
)

const (
	edgeAreaStage    = "stage"
	edgeAreaCamera   = "camera"
	edgeAreaViewport = "viewport"
)

func normalizeEdgeArea(area string) string {
	switch area = strings.ToLower(area); area {
	case "", edgeAreaStage:
		return edgeAreaStage
	case edgeAreaCamera, edgeAreaViewport:
		return edgeAreaCamera
	default:
		spxlog.Warn("unrecognized edge area %q, using %q", area, edgeAreaStage)
		return edgeAreaStage
	}
}
