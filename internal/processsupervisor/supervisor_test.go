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

package processsupervisor

import (
	"context"
	"errors"
	"testing"
)

func TestCancellationError(t *testing.T) {
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()

	cause := errors.New("custom cancellation cause")
	causedCtx, cancelCause := context.WithCancelCause(context.Background())
	cancelCause(cause)

	tests := []struct {
		name     string
		ctx      context.Context
		observed bool
		want     error
	}{
		{name: "not canceled", ctx: context.Background()},
		{name: "observed command cancellation", ctx: context.Background(), observed: true, want: context.Canceled},
		{name: "context canceled before observation", ctx: canceledCtx, want: context.Canceled},
		{name: "context cancellation cause", ctx: causedCtx, want: cause},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cancellationError(tt.ctx, tt.observed)
			if !errors.Is(got, tt.want) {
				t.Fatalf("cancellationError() = %v, want %v", got, tt.want)
			}
		})
	}
}
