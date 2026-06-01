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

package builderai

import _ "embed"

// Sync Builder AI project templates and versions from repository sources.
//
//go:generate cp ../../../../../gox.mod gox.mod
//go:generate go run ./genversion -source ../../../../../cmd/ispx/go.mod -output version_gen.go
//go:embed gox.mod
var defaultGoxModTemplate string
