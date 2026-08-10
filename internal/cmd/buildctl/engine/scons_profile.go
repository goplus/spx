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

import "github.com/goplus/spx/v3/internal/cmd/buildctl/shared"

type sconsProfile = shared.SConsProfile

func loadSConsProfile(spxModuleSource string) (sconsProfile, error) {
	return shared.LoadSConsProfile(spxModuleSource)
}

func parseSConsProfile(data []byte) (sconsProfile, error) {
	return shared.ParseSConsProfile(data)
}
