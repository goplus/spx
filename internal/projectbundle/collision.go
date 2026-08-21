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

package projectbundle

import (
	"fmt"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

var unicodeFold = cases.Fold()

func (c *collector) reserveName(name string) error {
	if previous, ok := c.exact[name]; ok {
		return fmt.Errorf("%w: duplicate path %q (already declared as %q)", ErrCollision, name, previous)
	}
	canonical := norm.NFC.String(name)
	if previous, ok := c.canonical[canonical]; ok {
		return fmt.Errorf("%w: Unicode-canonical paths %q and %q", ErrCollision, previous, name)
	}
	folded := unicodeFold.String(canonical)
	if previous, ok := c.folded[folded]; ok {
		return fmt.Errorf("%w: case-folded paths %q and %q", ErrCollision, previous, name)
	}
	c.exact[name] = name
	c.canonical[canonical] = name
	c.folded[folded] = name
	return nil
}
