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

import "reflect"

// listMonitorItems snapshots the current contents without retaining the source
// slice or list. Replacing or growing a list is reflected on the next refresh.
func listMonitorItems(value any) []string {
	switch list := value.(type) {
	case List:
		return listMonitorItems(list.data)
	case *List:
		if list == nil {
			return nil
		}
		return listMonitorItems(list.data)
	}
	list := reflect.ValueOf(value)
	for list.IsValid() && (list.Kind() == reflect.Pointer || list.Kind() == reflect.Interface) {
		list = list.Elem()
	}
	if !list.IsValid() || (list.Kind() != reflect.Slice && list.Kind() != reflect.Array) {
		return nil
	}
	items := make([]string, list.Len())
	for i := range items {
		items[i] = toString(list.Index(i).Interface())
	}
	return items
}
