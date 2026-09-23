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
	"reflect"
	"strings"
	"testing"
)

type shiftedSprite struct {
	value int
	SpriteImpl
}

func (*shiftedSprite) Main() {}

func TestSpriteBaseLayoutKeepsLookupAndReloadRules(t *testing.T) {
	sprite := &shiftedSprite{value: 7}
	if got := spriteOf(sprite); got != &sprite.SpriteImpl {
		t.Fatalf("spriteOf = %p, want embedded SpriteImpl %p", got, &sprite.SpriteImpl)
	}
	if err := validateReloadSprite(sprite, reflect.ValueOf(&reloadPreflightGame{}).Elem()); err == nil || !strings.Contains(err.Error(), "missing leading SpriteImpl field") {
		t.Fatalf("validateReloadSprite error = %v, want leading field error", err)
	}

	fields := snapshotSpriteUserFields(reflect.ValueOf(sprite).Elem())
	if got := fields[0].Int(); got != 7 {
		t.Fatalf("saved user field = %d, want 7", got)
	}
	if _, ok := fields[1]; ok {
		t.Fatal("saved user fields include SpriteImpl")
	}
}
