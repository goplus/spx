/*
 * Copyright (c) 2026 The XGo Authors (xgo.dev). All rights reserved.
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

package pack

import (
	"encoding/json"
	"fmt"

	"github.com/goplus/spx/v4/internal/base/stablefile"
)

const maxPackJSONSize = 16 << 20

func readJSONFile(name string, value any) error {
	data, _, err := (stablefile.Reader{MaxBytes: maxPackJSONSize}).Read(name)
	switch {
	case err == stablefile.ErrTooLarge:
		return fmt.Errorf("%s exceeds the JSON size limit", name)
	case err == stablefile.ErrNotRegular:
		return fmt.Errorf("%s must be a regular non-symlink file", name)
	case err == stablefile.ErrChangedOpening:
		return fmt.Errorf("%s changed while it was opened", name)
	case err == stablefile.ErrChangedReading:
		return fmt.Errorf("%s changed while it was read", name)
	case err != nil:
		return err
	}
	return json.Unmarshal(data, value)
}
