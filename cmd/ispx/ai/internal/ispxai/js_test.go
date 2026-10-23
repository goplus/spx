//go:build js && wasm

package ispxai

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"syscall/js"
	"testing"
	"time"

	"github.com/goplus/builder/tools/ai"
)

func bridgeRequireJSError(t *testing.T, value js.Value, message string) {
	t.Helper()
	if !value.InstanceOf(js.Global().Get("Error")) {
		t.Fatalf("result = %v, want a JavaScript Error", value)
	}
	if got := value.Get("message").String(); got != message {
		t.Errorf("error = %q, want %q", got, message)
	}
}

func bridgeRequireSuccess(t *testing.T, value js.Value) {
	t.Helper()
	if !value.IsNull() {
		t.Fatalf("result = %v, want null", value)
	}
}

func TestBridgeGlobals(t *testing.T) {
	originalKB := ai.DefaultKnowledgeBase()
	originalEndpoint, _ := aiBridgeTransport.config()
	t.Cleanup(func() {
		ai.SetDefaultKnowledgeBase(originalKB)
		aiBridgeTransport.setEndpoint(originalEndpoint)
	})
	if got := ai.DefaultTransport(); got != aiBridgeTransport {
		t.Fatalf("default AI transport = %T, want the JavaScript bridge", got)
	}

	t.Run("description", func(t *testing.T) {
		setter := js.Global().Get("xbuilder_set_ai_description")
		bridgeRequireSuccess(t, setter.Invoke("A forest with a talking fox."))
		want := map[string]any{aiDescriptionKnowledgeBaseKey: "A forest with a talking fox."}
		if got := ai.DefaultKnowledgeBase(); !reflect.DeepEqual(got, want) {
			t.Fatalf("knowledge base = %v, want %v", got, want)
		}

		bridgeRequireJSError(t, setter.Invoke(), "missing ai description argument")
		if got := ai.DefaultKnowledgeBase(); !reflect.DeepEqual(got, want) {
			t.Errorf("missing argument changed knowledge base to %v", got)
		}

		bridgeRequireSuccess(t, setter.Invoke(""))
		if got := ai.DefaultKnowledgeBase(); got != nil {
			t.Errorf("cleared knowledge base = %v, want nil", got)
		}
	})

	t.Run("endpoint", func(t *testing.T) {
		setter := js.Global().Get("xbuilder_set_ai_interaction_api_endpoint")
		bridgeRequireSuccess(t, setter.Invoke("https://example.test/ai"))
		if got, _ := aiBridgeTransport.config(); got != "https://example.test/ai" {
			t.Fatalf("endpoint = %q, want custom endpoint", got)
		}

		bridgeRequireJSError(t, setter.Invoke(), "missing ai interaction api endpoint argument")
		if got, _ := aiBridgeTransport.config(); got != "https://example.test/ai" {
			t.Errorf("missing argument changed endpoint to %q", got)
		}

		bridgeRequireSuccess(t, setter.Invoke(""))
		if got, _ := aiBridgeTransport.config(); got != defaultAIInteractionEndpoint {
			t.Errorf("reset endpoint = %q, want %q", got, defaultAIInteractionEndpoint)
		}
	})

	t.Run("token provider", func(t *testing.T) {
		setter := js.Global().Get("xbuilder_set_ai_interaction_api_token_provider")
		provider := jsTestFunc(t, func(this js.Value, args []js.Value) any { return "access-token" })
		_, originalProvider := aiBridgeTransport.config()
		// Restore the configured callback before jsTestFunc releases it.
		t.Cleanup(func() {
			if err := aiBridgeTransport.setTokenProvider(originalProvider); err != nil {
				t.Error(err)
			}
		})
		bridgeRequireSuccess(t, setter.Invoke(provider))
		if _, got := aiBridgeTransport.config(); !got.Equal(provider.Value) {
			t.Fatal("configured token provider does not match the supplied function")
		}

		bridgeRequireJSError(t, setter.Invoke(), "missing ai interaction api token provider argument")
		for _, invalid := range []any{"token", 42, true, map[string]any{}} {
			bridgeRequireJSError(t, setter.Invoke(invalid), "token provider must be a function")
			if _, got := aiBridgeTransport.config(); !got.Equal(provider.Value) {
				t.Fatalf("invalid argument %v changed token provider", invalid)
			}
		}

		for _, empty := range []js.Value{js.Null(), js.Undefined()} {
			bridgeRequireSuccess(t, setter.Invoke(provider))
			bridgeRequireSuccess(t, setter.Invoke(empty))
			if _, got := aiBridgeTransport.config(); !got.IsUndefined() {
				t.Errorf("cleared token provider = %v, want undefined", got)
			}
		}
	})
}

func TestBridgeToken(t *testing.T) {
	for _, provider := range []js.Value{js.Undefined(), js.Null(), js.ValueOf("not a function")} {
		token, err := jsToken(context.Background(), provider)
		if err != nil || token != "" {
			t.Errorf("jsToken(%v) = (%q, %v), want empty token without error", provider, token, err)
		}
	}

	tests := []struct {
		name      string
		result    func() any
		wantToken string
		wantError string
	}{
		{"synchronous token", func() any { return "sync-token" }, "sync-token", ""},
		{"synchronous null", func() any { return js.Null() }, "", ""},
		{"synchronous undefined", func() any { return js.Undefined() }, "", ""},
		{"promised token", func() any {
			return js.Global().Get("Promise").Call("resolve", "async-token")
		}, "async-token", ""},
		{"promised null", func() any {
			return js.Global().Get("Promise").Call("resolve", js.Null())
		}, "", ""},
		{"promised undefined", func() any {
			return js.Global().Get("Promise").Call("resolve")
		}, "", ""},
		{"rejected error", func() any {
			return js.Global().Get("Promise").Call("reject", js.Global().Get("Error").New("expired token"))
		}, "", "promise rejected: expired token"},
		{"rejected string", func() any {
			return js.Global().Get("Promise").Call("reject", "signed out")
		}, "", "promise rejected: signed out"},
		{"rejected without reason", func() any {
			return js.Global().Get("Promise").Call("reject")
		}, "", "promise rejected"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			provider := jsTestFunc(t, func(this js.Value, args []js.Value) any {
				calls++
				return tt.result()
			})
			ctx := jsTestContext(t)
			token, err := jsToken(ctx, provider.Value)
			if token != tt.wantToken {
				t.Errorf("token = %q, want %q", token, tt.wantToken)
			}
			if tt.wantError == "" {
				if err != nil {
					t.Errorf("unexpected token error: %v", err)
				}
			} else if err == nil || err.Error() != tt.wantError {
				t.Errorf("token error = %v, want %q", err, tt.wantError)
			}
			if calls != 1 {
				t.Errorf("token provider invoked %d times, want once", calls)
			}
		})
	}
}

func TestBridgePromiseCancellation(t *testing.T) {
	// Keep this promise pending so cancellation is the only possible result.
	executor := jsTestFunc(t, func(this js.Value, args []js.Value) any { return nil })
	promise := js.Global().Get("Promise").New(executor)
	provider := jsTestFunc(t, func(this js.Value, args []js.Value) any { return promise })
	ctx, cancel := context.WithCancel(jsTestContext(t))
	cancel()
	token, err := jsToken(ctx, provider.Value)
	if token != "" || !errors.Is(err, context.Canceled) {
		t.Errorf("canceled token = (%q, %v), want empty token and context.Canceled", token, err)
	}
}

func TestBridgeThenableWithoutValue(t *testing.T) {
	catch := jsTestFunc(t, func(this js.Value, args []js.Value) any { return nil })
	then := jsTestFunc(t, func(this js.Value, args []js.Value) any {
		args[0].Invoke()
		return map[string]any{"catch": catch}
	})
	ctx := jsTestContext(t)
	value, err := awaitJSValue(ctx, js.ValueOf(map[string]any{"then": then}))
	if err != nil || !value.IsUndefined() {
		t.Errorf("thenable result = (%v, %v), want undefined without error", value, err)
	}
}

func TestBridgeRejectionErrors(t *testing.T) {
	tests := []struct {
		name string
		args []js.Value
		want string
	}{
		{"no arguments", nil, "promise rejected"},
		{"undefined", []js.Value{js.Undefined()}, "promise rejected"},
		{"null", []js.Value{js.Null()}, "promise rejected"},
		{"error", []js.Value{js.Global().Get("Error").New("network unavailable")}, "promise rejected: network unavailable"},
		{"message object", []js.Value{js.ValueOf(map[string]any{"message": "permission denied"})}, "promise rejected: permission denied"},
		{"string", []js.Value{js.ValueOf("invalid token")}, "promise rejected: invalid token"},
		{"number", []js.Value{js.ValueOf(42)}, "promise rejected: <number: 42>"},
		{"object without message", []js.Value{js.ValueOf(map[string]any{})}, "promise rejected: <object>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := jsError(tt.args).Error(); got != tt.want {
				t.Errorf("error = %q, want %q", got, tt.want)
			}
		})
	}
}

func jsTestFunc(t *testing.T, fn func(this js.Value, args []js.Value) any) js.Func {
	t.Helper()
	f := js.FuncOf(fn)
	t.Cleanup(f.Release)
	return f
}

func jsTestContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func transportMockFetch(t *testing.T, fn func(this js.Value, args []js.Value) any) {
	t.Helper()
	global := js.Global()
	original := global.Get("fetch")
	hadOwn := global.Get("Object").Get("prototype").Get("hasOwnProperty").Call("call", global, "fetch").Bool()
	f := jsTestFunc(t, fn)
	global.Set("fetch", f)
	t.Cleanup(func() {
		if hadOwn {
			global.Set("fetch", original)
		} else {
			global.Delete("fetch")
		}
	})
}

func transportResolved(value js.Value) js.Value {
	return js.Global().Get("Promise").Call("resolve", value)
}

func transportRejected(message string) js.Value {
	return js.Global().Get("Promise").Call("reject", js.Global().Get("Error").New(message))
}

func TestTransportInteract(t *testing.T) {
	transport := newJSAITransport()
	transport.setEndpoint("https://example.invalid/interaction")
	provider := jsTestFunc(t, func(this js.Value, args []js.Value) any {
		return transportResolved(js.ValueOf("test-token"))
	})
	if err := transport.setTokenProvider(provider.Value); err != nil {
		t.Fatal(err)
	}

	var url string
	var options js.Value
	calls := 0
	transportMockFetch(t, func(this js.Value, args []js.Value) any {
		calls++
		url, options = args[0].String(), args[1]
		return transportResolved(js.Global().Get("Response").New(`{"text":"向前走","commandName":"move","commandArgs":{"steps":10}}`))
	})
	req := ai.Request{
		Content:          "你好",
		Context:          map[string]any{"scene": "forest"},
		Role:             "guide",
		RoleContext:      map[string]any{"friendly": true},
		KnowledgeBase:    map[string]any{"goal": "find the key"},
		History:          []ai.Turn{{RequestContent: "where?", ResponseText: "here"}},
		ArchivedHistory:  "arrived at the forest",
		ContinuationTurn: 1,
	}
	got, err := transport.Interact(jsTestContext(t), req)
	if err != nil {
		t.Fatal(err)
	}
	want := ai.Response{Text: "向前走", CommandName: "move", CommandArgs: map[string]any{"steps": float64(10)}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Interact() = %#v, want %#v", got, want)
	}
	if calls != 1 || url != "https://example.invalid/interaction/turn" {
		t.Fatalf("fetch calls = %d, URL = %q", calls, url)
	}
	if got := options.Get("method").String(); got != "POST" {
		t.Errorf("method = %q, want POST", got)
	}
	headers := options.Get("headers")
	if got := headers.Get("Content-Type").String(); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	if got := headers.Get("Authorization").String(); got != "Bearer test-token" {
		t.Errorf("Authorization = %q, want Bearer test-token", got)
	}
	if signal := options.Get("signal"); !signal.InstanceOf(js.Global().Get("AbortSignal")) || signal.Get("aborted").Bool() {
		t.Errorf("fetch signal is not an active AbortSignal: %v", signal)
	}
	var sent ai.Request
	if err := json.Unmarshal([]byte(options.Get("body").String()), &sent); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(sent, req) {
		t.Errorf("sent request = %#v, want %#v", sent, req)
	}
}

func TestTransportArchive(t *testing.T) {
	transport := newJSAITransport()
	var url string
	var options js.Value
	transportMockFetch(t, func(this js.Value, args []js.Value) any {
		url, options = args[0].String(), args[1]
		return transportResolved(js.Global().Get("Response").New(`{"content":"找到了钥匙"}`))
	})
	turns := []ai.Turn{{RequestContent: "open the chest", ResponseText: "found the key", IsInitial: true}}
	got, err := transport.Archive(jsTestContext(t), turns, "entered the room")
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != "找到了钥匙" {
		t.Errorf("archive content = %q", got.Content)
	}
	if url != defaultAIInteractionEndpoint+"/archive" {
		t.Errorf("URL = %q, want %q", url, defaultAIInteractionEndpoint+"/archive")
	}
	if got := options.Get("headers").Get("Authorization"); !got.IsUndefined() {
		t.Errorf("Authorization = %v, want absent without a token provider", got)
	}
	var sent struct {
		Turns           []ai.Turn `json:"turns"`
		ExistingArchive string    `json:"existingArchive"`
	}
	if err := json.Unmarshal([]byte(options.Get("body").String()), &sent); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(sent.Turns, turns) || sent.ExistingArchive != "entered the room" {
		t.Errorf("sent archive = %#v", sent)
	}
}

func TestTransportHTTPErrors(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		statusText string
		retryAfter string
		wantRetry  time.Duration
	}{
		{name: "server error", status: 503, statusText: "Service Unavailable"},
		{name: "rate limited", status: 429, statusText: "Too Many Requests", retryAfter: "12", wantRetry: 12 * time.Second},
		{name: "rate limited without retry header", status: 429, statusText: "Too Many Requests"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transportMockFetch(t, func(this js.Value, args []js.Value) any {
				headers := map[string]any{}
				if tt.retryAfter != "" {
					headers["Retry-After"] = tt.retryAfter
				}
				return transportResolved(js.Global().Get("Response").New("try again later", map[string]any{
					"status": tt.status, "statusText": tt.statusText, "headers": headers,
				}))
			})
			got, err := newJSAITransport().Interact(jsTestContext(t), ai.Request{Content: "hello"})
			if err == nil || !strings.Contains(err.Error(), tt.statusText+": try again later") {
				t.Fatalf("Interact() error = %v, want status and response body", err)
			}
			if !reflect.DeepEqual(got, ai.Response{}) {
				t.Errorf("failed Interact() response = %#v, want zero value", got)
			}
			var rateLimit *ai.TooManyRequestsError
			if errors.As(err, &rateLimit) != (tt.status == 429) {
				t.Fatalf("rate-limit error = %v, status = %d", rateLimit, tt.status)
			}
			if rateLimit != nil && rateLimit.RetryAfter != tt.wantRetry {
				t.Errorf("RetryAfter = %v, want %v", rateLimit.RetryAfter, tt.wantRetry)
			}
		})
	}
}

func TestTransportPostJSONErrors(t *testing.T) {
	tests := []struct {
		name          string
		prepare       func(*testing.T, *jsAITransport) (any, func() js.Value)
		want          string
		wantFetchCall bool
	}{
		{
			name: "token provider rejects",
			prepare: func(t *testing.T, transport *jsAITransport) (any, func() js.Value) {
				provider := jsTestFunc(t, func(this js.Value, args []js.Value) any {
					return transportRejected("token expired")
				})
				if err := transport.setTokenProvider(provider.Value); err != nil {
					t.Fatal(err)
				}
				return nil, nil
			},
			want: "failed to get ai interaction api token: promise rejected: token expired",
		},
		{
			name: "request cannot be marshaled",
			prepare: func(t *testing.T, transport *jsAITransport) (any, func() js.Value) {
				return map[string]any{"unsupported": make(chan int)}, nil
			},
			want: "failed to marshal request: json: unsupported type: chan int",
		},
		{
			name: "fetch rejects",
			prepare: func(t *testing.T, transport *jsAITransport) (any, func() js.Value) {
				return nil, func() js.Value { return transportRejected("network unavailable") }
			},
			want:          "failed to fetch: promise rejected: network unavailable",
			wantFetchCall: true,
		},
		{
			name: "response body rejects",
			prepare: func(t *testing.T, transport *jsAITransport) (any, func() js.Value) {
				body := jsTestFunc(t, func(this js.Value, args []js.Value) any {
					return transportRejected("body unavailable")
				})
				return nil, func() js.Value {
					return transportResolved(js.ValueOf(map[string]any{"ok": true, "arrayBuffer": body}))
				}
			},
			want:          "failed to read response body: promise rejected: body unavailable",
			wantFetchCall: true,
		},
		{
			name: "response contains invalid JSON",
			prepare: func(t *testing.T, transport *jsAITransport) (any, func() js.Value) {
				return nil, func() js.Value { return transportResolved(js.Global().Get("Response").New("not JSON")) }
			},
			want:          "failed to unmarshal response json:",
			wantFetchCall: true,
		},
		{
			name: "HTTP error body rejects",
			prepare: func(t *testing.T, transport *jsAITransport) (any, func() js.Value) {
				body := jsTestFunc(t, func(this js.Value, args []js.Value) any {
					return transportRejected("body unavailable")
				})
				return nil, func() js.Value {
					return transportResolved(js.ValueOf(map[string]any{
						"ok": false, "status": 502, "statusText": "Bad Gateway", "text": body,
					}))
				}
			},
			want:          "failed to fetch with status 502 Bad Gateway (and failed to read error body: promise rejected: body unavailable)",
			wantFetchCall: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := newJSAITransport()
			payload, fetchResult := tt.prepare(t, transport)
			called := false
			transportMockFetch(t, func(this js.Value, args []js.Value) any {
				called = true
				if fetchResult == nil {
					return transportResolved(js.Global().Get("Response").New(`{}`))
				}
				return fetchResult()
			})
			var response ai.Response
			err := transport.postJSON(jsTestContext(t), "/turn", payload, &response)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("postJSON() error = %v, want %q", err, tt.want)
			}
			if called != tt.wantFetchCall {
				t.Errorf("fetch called = %v, want %v", called, tt.wantFetchCall)
			}
		})
	}
}

func TestTransportArchiveError(t *testing.T) {
	transportMockFetch(t, func(this js.Value, args []js.Value) any {
		return transportResolved(js.Global().Get("Response").New(`{"content":123}`))
	})
	got, err := newJSAITransport().Archive(jsTestContext(t), nil, "")
	var typeError *json.UnmarshalTypeError
	if !errors.As(err, &typeError) {
		t.Fatalf("Archive() error = %v, want JSON type error", err)
	}
	if got != (ai.ArchivedHistory{}) {
		t.Errorf("failed Archive() response = %#v, want zero value", got)
	}
}

func TestTransportCancellationAbortsFetch(t *testing.T) {
	ctx, cancel := context.WithCancel(jsTestContext(t))
	defer cancel()
	aborted := make(chan struct{}, 1)
	onAbort := jsTestFunc(t, func(this js.Value, args []js.Value) any {
		aborted <- struct{}{}
		return nil
	})
	// Leave this mock request pending so it has no asynchronous callbacks after
	// the test returns. The abort event independently verifies cancellation.
	executor := jsTestFunc(t, func(this js.Value, args []js.Value) any { return nil })
	transportMockFetch(t, func(this js.Value, args []js.Value) any {
		args[1].Get("signal").Call("addEventListener", "abort", onAbort, map[string]any{"once": true})
		cancel()
		return js.Global().Get("Promise").New(executor)
	})
	_, err := newJSAITransport().Interact(ctx, ai.Request{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Interact() error = %v, want context.Canceled", err)
	}
	select {
	case <-aborted:
	case <-time.After(time.Second):
		t.Fatal("fetch AbortSignal was not aborted after context cancellation")
	}
}
