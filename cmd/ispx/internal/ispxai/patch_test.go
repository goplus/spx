package ispxai

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/goplus/builder/tools/ai"
	"github.com/goplus/ixgo"
)

func TestRegisterPatchDispatchesInterpretedCommands(t *testing.T) {
	for _, function := range []string{"XGot_Player_XGox_OnCmd", "XGot_Player_XGox_OnCmd__0"} {
		t.Run(function, func(t *testing.T) {
			interp := newPatchedInterp(t, fmt.Sprintf(`
package main

import "github.com/goplus/builder/tools/ai"

type Move struct {
	Steps int
	Message string
}

func Register(player *ai.Player, handler func(int, string) error) {
	ai.%s(player, func(command Move) error {
		return handler(command.Steps, command.Message)
	})
}
`, function))
			for _, test := range []struct {
				name       string
				handlerErr error
				wantResult ai.CommandResult
			}{
				{name: "success", wantResult: ai.CommandResult{Success: true}},
				{name: "handler error", handlerErr: errors.New("move blocked"), wantResult: ai.CommandResult{ErrorMessage: "move blocked"}},
				{name: "break", handlerErr: ai.Break},
			} {
				t.Run(test.name, func(t *testing.T) {
					transport := &patchTestTransport{}
					previous := ai.DefaultTransport()
					ai.SetDefaultTransport(transport)
					t.Cleanup(func() { ai.SetDefaultTransport(previous) })

					var player ai.Player
					var interactionErr error
					player.OnErr__0(func(err error) { interactionErr = err })
					var calls, steps int
					var message string
					_, err := interp.RunFunc("Register", &player, func(n int, s string) error {
						calls++
						steps, message = n, s
						return test.handlerErr
					})
					if err != nil {
						t.Fatalf("register command: %v", err)
					}
					player.Think__1("move forward")
					if interactionErr != nil {
						t.Fatalf("run command: %v", interactionErr)
					}
					if calls != 1 || steps != 7 || message != "hello" {
						t.Fatalf("handler calls = %d, arguments = (%d, %q)", calls, steps, message)
					}
					wantRequests := 2
					if test.handlerErr == ai.Break {
						wantRequests = 1
					}
					if len(transport.requests) != wantRequests {
						t.Fatalf("transport requests = %d, want %d", len(transport.requests), wantRequests)
					}
					wantSpecs := []ai.CommandSpec{{
						Name:        "Move",
						Description: "Command Move",
						Parameters: []ai.CommandParamSpec{
							{Name: "Steps", Type: "int"},
							{Name: "Message", Type: "string"},
						},
					}}
					if got := transport.requests[0].CommandSpecs; !reflect.DeepEqual(got, wantSpecs) {
						t.Fatalf("command specs = %#v, want %#v", got, wantSpecs)
					}
					if wantRequests == 2 {
						history := transport.requests[1].History
						if len(history) != 1 || history[0].ExecutedCommandResult == nil {
							t.Fatalf("command history = %#v", history)
						}
						if got := *history[0].ExecutedCommandResult; got != test.wantResult {
							t.Fatalf("command result = %#v, want %#v", got, test.wantResult)
						}
					}
				})
			}
		})
	}
}

func TestRegisterPatchPreservesCommandValidation(t *testing.T) {
	for _, test := range []struct {
		commandType string
		wantError   string
	}{
		{commandType: "int", wantError: "AI command must be a struct type"},
		{commandType: "struct{ Steps int }", wantError: "AI command struct must have a name"},
	} {
		t.Run(test.commandType, func(t *testing.T) {
			interp := newPatchedInterp(t, fmt.Sprintf(`
package main

import "github.com/goplus/builder/tools/ai"

func Register(player *ai.Player) {
	ai.XGot_Player_XGox_OnCmd(player, func(command %s) error { return nil })
}
`, test.commandType))
			_, err := interp.RunFunc("Register", new(ai.Player))
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("register invalid command error = %v, want %q", err, test.wantError)
			}
		})
	}
}

func newPatchedInterp(t *testing.T, source string) *ixgo.Interp {
	t.Helper()
	ctx := ixgo.NewContext(ixgo.SupportMultipleInterp)
	if err := RegisterPatch(ctx); err != nil {
		t.Fatalf("register AI patch: %v", err)
	}
	interp, err := ctx.LoadInterp("commands.go", source)
	if err != nil {
		t.Fatalf("load interpreted command: %v", err)
	}
	t.Cleanup(interp.UnsafeRelease)
	return interp
}

type patchTestTransport struct {
	requests []ai.Request
}

func (transport *patchTestTransport) Interact(_ context.Context, request ai.Request) (ai.Response, error) {
	transport.requests = append(transport.requests, request)
	if len(transport.requests) == 1 {
		return ai.Response{
			CommandName: "Move",
			CommandArgs: map[string]any{"Steps": float64(7), "Message": "hello"},
		}, nil
	}
	return ai.Response{}, nil
}

func (*patchTestTransport) Archive(context.Context, []ai.Turn, string) (ai.ArchivedHistory, error) {
	return ai.ArchivedHistory{}, errors.New("unexpected history archive")
}
