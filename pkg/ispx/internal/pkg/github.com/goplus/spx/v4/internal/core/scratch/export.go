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

// export by github.com/goplus/ixgo/cmd/qexp

package scratch

import (
	q "github.com/goplus/spx/v4/internal/core/scratch"

	"github.com/goplus/ixgo"
)

func init() {
	ixgo.RegisterDirectCalls("github.com/goplus/spx/v4/internal/core/scratch", map[string]ixgo.DirectCallAdapter{
		"(*List).Append":   method_ptr_List_Append,
		"(*List).At":       method_ptr_List_At,
		"(*List).Clear":    method_ptr_List_Clear,
		"(*List).Contains": method_ptr_List_Contains,
		"(*List).Delete":   method_ptr_List_Delete,
		"(*List).IndexOf":  method_ptr_List_IndexOf,
		"(*List).Init":     method_ptr_List_Init,
		"(*List).InitFrom": method_ptr_List_InitFrom,
		"(*List).Insert":   method_ptr_List_Insert,
		"(*List).Len":      method_ptr_List_Len,
		"(*List).Set":      method_ptr_List_Set,
		"(*List).String":   method_ptr_List_String,
		"(*Value).Equal":   method_ptr_Value_Equal,
		"(*Value).Float":   method_ptr_Value_Float,
		"(*Value).Int":     method_ptr_Value_Int,
		"(*Value).Raw":     method_ptr_Value_Raw,
		"(*Value).Set":     method_ptr_Value_Set,
		"(*Value).String":  method_ptr_Value_String,
		"(Value).Equal":    method_Value_Equal,
		"(Value).Float":    method_Value_Float,
		"(Value).Int":      method_Value_Int,
		"(Value).Raw":      method_Value_Raw,
		"(Value).String":   method_Value_String,
	})
}

func method_ptr_List_Append(ctx ixgo.DirectCallContext) {
	(*q.List).Append(ixgo.DirectCallArg[*q.List](ctx, 0), ixgo.DirectCallArg[any](ctx, 1))
}

func method_ptr_List_At(ctx ixgo.DirectCallContext) {
	ctx.SetResult((*q.List).At(ixgo.DirectCallArg[*q.List](ctx, 0), ixgo.DirectCallArg[int](ctx, 1)))
}

func method_ptr_List_Clear(ctx ixgo.DirectCallContext) {
	(*q.List).Clear(ixgo.DirectCallArg[*q.List](ctx, 0))
}

func method_ptr_List_Contains(ctx ixgo.DirectCallContext) {
	ctx.SetResult((*q.List).Contains(ixgo.DirectCallArg[*q.List](ctx, 0), ixgo.DirectCallArg[any](ctx, 1)))
}

func method_ptr_List_Delete(ctx ixgo.DirectCallContext) {
	(*q.List).Delete(ixgo.DirectCallArg[*q.List](ctx, 0), ixgo.DirectCallArg[int](ctx, 1))
}

func method_ptr_List_IndexOf(ctx ixgo.DirectCallContext) {
	ctx.SetResult((*q.List).IndexOf(ixgo.DirectCallArg[*q.List](ctx, 0), ixgo.DirectCallArg[any](ctx, 1)))
}

func method_ptr_List_Init(ctx ixgo.DirectCallContext) {
	(*q.List).Init(ixgo.DirectCallArg[*q.List](ctx, 0), ixgo.DirectCallArg[[]any](ctx, 1)...)
}

func method_ptr_List_InitFrom(ctx ixgo.DirectCallContext) {
	(*q.List).InitFrom(ixgo.DirectCallArg[*q.List](ctx, 0), ixgo.DirectCallArg[*q.List](ctx, 1))
}

func method_ptr_List_Insert(ctx ixgo.DirectCallContext) {
	(*q.List).Insert(ixgo.DirectCallArg[*q.List](ctx, 0), ixgo.DirectCallArg[int](ctx, 1), ixgo.DirectCallArg[any](ctx, 2))
}

func method_ptr_List_Len(ctx ixgo.DirectCallContext) {
	ctx.SetResult((*q.List).Len(ixgo.DirectCallArg[*q.List](ctx, 0)))
}

func method_ptr_List_Set(ctx ixgo.DirectCallContext) {
	(*q.List).Set(ixgo.DirectCallArg[*q.List](ctx, 0), ixgo.DirectCallArg[int](ctx, 1), ixgo.DirectCallArg[any](ctx, 2))
}

func method_ptr_List_String(ctx ixgo.DirectCallContext) {
	ctx.SetResult((*q.List).String(ixgo.DirectCallArg[*q.List](ctx, 0)))
}

func method_Value_Equal(ctx ixgo.DirectCallContext) {
	ctx.SetResult(q.Value.Equal(ixgo.DirectCallArg[q.Value](ctx, 0), ixgo.DirectCallArg[any](ctx, 1)))
}

func method_ptr_Value_Equal(ctx ixgo.DirectCallContext) {
	ctx.SetResult((*q.Value).Equal(ixgo.DirectCallArg[*q.Value](ctx, 0), ixgo.DirectCallArg[any](ctx, 1)))
}

func method_Value_Float(ctx ixgo.DirectCallContext) {
	ctx.SetResult(q.Value.Float(ixgo.DirectCallArg[q.Value](ctx, 0)))
}

func method_ptr_Value_Float(ctx ixgo.DirectCallContext) {
	ctx.SetResult((*q.Value).Float(ixgo.DirectCallArg[*q.Value](ctx, 0)))
}

func method_Value_Int(ctx ixgo.DirectCallContext) {
	ctx.SetResult(q.Value.Int(ixgo.DirectCallArg[q.Value](ctx, 0)))
}

func method_ptr_Value_Int(ctx ixgo.DirectCallContext) {
	ctx.SetResult((*q.Value).Int(ixgo.DirectCallArg[*q.Value](ctx, 0)))
}

func method_Value_Raw(ctx ixgo.DirectCallContext) {
	ctx.SetResult(q.Value.Raw(ixgo.DirectCallArg[q.Value](ctx, 0)))
}

func method_ptr_Value_Raw(ctx ixgo.DirectCallContext) {
	ctx.SetResult((*q.Value).Raw(ixgo.DirectCallArg[*q.Value](ctx, 0)))
}

func method_ptr_Value_Set(ctx ixgo.DirectCallContext) {
	(*q.Value).Set(ixgo.DirectCallArg[*q.Value](ctx, 0), ixgo.DirectCallArg[any](ctx, 1))
}

func method_Value_String(ctx ixgo.DirectCallContext) {
	ctx.SetResult(q.Value.String(ixgo.DirectCallArg[q.Value](ctx, 0)))
}

func method_ptr_Value_String(ctx ixgo.DirectCallContext) {
	ctx.SetResult((*q.Value).String(ixgo.DirectCallArg[*q.Value](ctx, 0)))
}
