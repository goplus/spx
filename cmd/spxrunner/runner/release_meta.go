/*
 * Copyright (c) 2024 The XGo Authors (xgo.dev). All rights reserved.
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

package runner

import releasemeta "github.com/goplus/spx/v2/internal/releasemeta"

const (
	RuntimeURLBase    = releasemeta.RuntimeURLBase
	SpxReleaseURLBase = releasemeta.SpxReleaseURLBase
	RuntimeTag        = releasemeta.RuntimeTag
)

type ReleaseMeta = releasemeta.ReleaseMeta
type RuntimeRelease = releasemeta.RuntimeRelease
type PckRelease = releasemeta.PckRelease

func ReleaseMetaForSPXVersion(spxVersion string) ReleaseMeta {
	return releasemeta.ReleaseMetaForSPXVersion(spxVersion)
}

func CurrentReleaseMeta() ReleaseMeta {
	return releasemeta.CurrentReleaseMeta()
}
