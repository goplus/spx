//go:build js && wasm

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

package ispxai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"syscall/js"

	"github.com/goplus/builder/tools/ai"
)

const defaultAIInteractionEndpoint = "/api/ai/interaction"
const aiDescriptionKnowledgeBaseKey = "AI-generated descriptive summary of the game world"

var aiBridgeTransport = newJSAITransport()
var jsTypeError = js.Global().Get("Error")

func init() {
	ai.SetDefaultTransport(aiBridgeTransport)
	js.Global().Set("xbuilder_set_ai_description", jsFuncOfWithError(setAIDescription))
	js.Global().Set("xbuilder_set_ai_interaction_api_endpoint", jsFuncOfWithError(setAIInteractionAPIEndpoint))
	js.Global().Set("xbuilder_set_ai_interaction_api_token_provider", jsFuncOfWithError(setAIInteractionAPITokenProvider))
}

func jsFuncOfWithError(fn func(this js.Value, args []js.Value) any) js.Func {
	return js.FuncOf(func(this js.Value, args []js.Value) any {
		result := fn(this, args)
		if err, ok := result.(error); ok {
			return jsTypeError.New(err.Error())
		}
		return result
	})
}

type jsAITransport struct {
	mu            sync.RWMutex
	endpoint      string
	tokenProvider js.Value
}

func newJSAITransport() *jsAITransport {
	return &jsAITransport{
		endpoint:      defaultAIInteractionEndpoint,
		tokenProvider: js.Undefined(),
	}
}

func (t *jsAITransport) setEndpoint(endpoint string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if endpoint == "" {
		endpoint = defaultAIInteractionEndpoint
	}
	t.endpoint = endpoint
}

func (t *jsAITransport) setTokenProvider(provider js.Value) error {
	if !isJSFunction(provider) && !isJSNil(provider) {
		return errors.New("token provider must be a function")
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if isJSNil(provider) {
		provider = js.Undefined()
	}
	t.tokenProvider = provider
	return nil
}

func (t *jsAITransport) config() (endpoint string, tokenProvider js.Value) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.endpoint, t.tokenProvider
}

func (t *jsAITransport) Interact(ctx context.Context, req ai.Request) (ai.Response, error) {
	var resp ai.Response
	if err := t.postJSON(ctx, "/turn", req, &resp); err != nil {
		return ai.Response{}, err
	}
	return resp, nil
}

func (t *jsAITransport) Archive(ctx context.Context, turns []ai.Turn, existingArchive string) (ai.ArchivedHistory, error) {
	payload := map[string]any{
		"turns":           turns,
		"existingArchive": existingArchive,
	}
	var resp ai.ArchivedHistory
	if err := t.postJSON(ctx, "/archive", payload, &resp); err != nil {
		return ai.ArchivedHistory{}, err
	}
	return resp, nil
}

func (t *jsAITransport) postJSON(ctx context.Context, path string, payload, target any) error {
	endpoint, tokenProvider := t.config()
	token, err := jsToken(ctx, tokenProvider)
	if err != nil {
		return fmt.Errorf("failed to get ai interaction api token: %w", err)
	}

	reqBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	headers := map[string]any{
		"Content-Type": "application/json",
	}
	if token != "" {
		headers["Authorization"] = "Bearer " + token
	}

	abortController := js.Global().Get("AbortController").New()
	stopAbort := context.AfterFunc(ctx, func() {
		abortController.Call("abort")
	})
	defer stopAbort()

	resp, err := awaitJSValue(ctx, js.Global().Call("fetch", endpoint+path, map[string]any{
		"method":  "POST",
		"headers": headers,
		"body":    string(reqBody),
		"signal":  abortController.Get("signal"),
	}))
	if err != nil {
		return fmt.Errorf("failed to fetch: %w", err)
	}

	if !resp.Get("ok").Bool() {
		return jsHTTPError(ctx, resp)
	}

	body, err := awaitJSValue(ctx, resp.Call("arrayBuffer"))
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}
	data := make([]byte, body.Get("byteLength").Int())
	js.CopyBytesToGo(data, js.Global().Get("Uint8Array").New(body))
	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("failed to unmarshal response json: %w", err)
	}
	return nil
}

func setAIDescription(this js.Value, args []js.Value) any {
	if len(args) == 0 {
		return errors.New("missing ai description argument")
	}
	description := args[0].String()
	if description == "" {
		ai.SetDefaultKnowledgeBase(nil)
		return nil
	}
	ai.SetDefaultKnowledgeBase(map[string]any{
		aiDescriptionKnowledgeBaseKey: description,
	})
	return nil
}

func setAIInteractionAPIEndpoint(this js.Value, args []js.Value) any {
	if len(args) == 0 {
		return errors.New("missing ai interaction api endpoint argument")
	}
	aiBridgeTransport.setEndpoint(args[0].String())
	return nil
}

func setAIInteractionAPITokenProvider(this js.Value, args []js.Value) any {
	if len(args) == 0 {
		return errors.New("missing ai interaction api token provider argument")
	}
	if err := aiBridgeTransport.setTokenProvider(args[0]); err != nil {
		return err
	}
	return nil
}

func jsToken(ctx context.Context, provider js.Value) (string, error) {
	if !isJSFunction(provider) {
		return "", nil
	}
	token, err := awaitJSValue(ctx, provider.Invoke())
	if err != nil {
		return "", err
	}
	if isJSNil(token) {
		return "", nil
	}
	return token.String(), nil
}

func awaitJSValue(ctx context.Context, value js.Value) (js.Value, error) {
	if isJSPromise(value) {
		return awaitJSPromise(ctx, value)
	}
	return value, nil
}

func awaitJSPromise(ctx context.Context, promise js.Value) (js.Value, error) {
	resultChan := make(chan js.Value, 1)
	then := js.FuncOf(func(this js.Value, args []js.Value) any {
		result := js.Undefined()
		if len(args) > 0 {
			result = args[0]
		}
		resultChan <- result
		return nil
	})
	defer then.Release()

	errChan := make(chan error, 1)
	catch := js.FuncOf(func(this js.Value, args []js.Value) any {
		errChan <- jsError(args)
		return nil
	})
	defer catch.Release()

	promise.Call("then", then).Call("catch", catch)
	select {
	case result := <-resultChan:
		return result, nil
	case err := <-errChan:
		return js.Undefined(), err
	case <-ctx.Done():
		return js.Undefined(), fmt.Errorf("context canceled while waiting for promise: %w", ctx.Err())
	}
}

func jsHTTPError(ctx context.Context, resp js.Value) error {
	status := resp.Get("status").Int()
	statusText := resp.Get("statusText").String()
	bodyVal, err := awaitJSValue(ctx, resp.Call("text"))
	if err != nil {
		return fmt.Errorf("failed to fetch with status %d %s (and failed to read error body: %w)", status, statusText, err)
	}
	err = fmt.Errorf("failed to fetch with status %d %s: %s", status, statusText, bodyVal.String())
	if status == 429 {
		retryAfter := ai.RetryAfterFromHeader(resp.Get("headers").Call("get", "Retry-After").String())
		return &ai.TooManyRequestsError{RetryAfter: retryAfter, Err: err}
	}
	return err
}

func jsError(args []js.Value) error {
	if len(args) == 0 || isJSNil(args[0]) {
		return errors.New("promise rejected")
	}
	errVal := args[0]
	if errVal.Type() == js.TypeObject && errVal.Get("message").Type() == js.TypeString {
		return fmt.Errorf("promise rejected: %s", errVal.Get("message").String())
	}
	if errVal.Type() == js.TypeString {
		return fmt.Errorf("promise rejected: %s", errVal.String())
	}
	return fmt.Errorf("promise rejected: %v", errVal)
}

func isJSFunction(value js.Value) bool {
	return !isJSNil(value) && value.Type() == js.TypeFunction
}

func isJSPromise(value js.Value) bool {
	return !isJSNil(value) && value.Type() == js.TypeObject && value.Get("then").Type() == js.TypeFunction
}

func isJSNil(value js.Value) bool {
	return value.IsUndefined() || value.IsNull()
}
