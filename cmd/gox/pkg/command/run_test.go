package command

import (
	"errors"
	"reflect"
	"testing"
)

func TestRunWebCommandRunsExportBeforeServer(t *testing.T) {
	var calls []string

	err := runWebCommand(
		func() error {
			calls = append(calls, "export")
			return nil
		},
		func() error {
			calls = append(calls, "serve")
			return nil
		},
	)
	if err != nil {
		t.Fatalf("runWebCommand returned error: %v", err)
	}

	want := []string{"export", "serve"}
	if !reflect.DeepEqual(calls, want) {
		t.Fatalf("unexpected call order: got %v, want %v", calls, want)
	}
}

func TestRunWebCommandStopsOnExportError(t *testing.T) {
	wantErr := errors.New("export failed")
	serverCalled := false

	err := runWebCommand(
		func() error {
			return wantErr
		},
		func() error {
			serverCalled = true
			return nil
		},
	)
	if !errors.Is(err, wantErr) {
		t.Fatalf("runWebCommand error = %v, want %v", err, wantErr)
	}
	if serverCalled {
		t.Fatal("server should not start when export fails")
	}
}

func TestIsRuntimeModeCommand(t *testing.T) {
	tests := []struct {
		name    string
		cmdName string
		want    bool
	}{
		{name: "run", cmdName: "run", want: true},
		{name: "runweb", cmdName: "runweb", want: true},
		{name: "runwebworker", cmdName: "runwebworker", want: true},
		{name: "exportweb", cmdName: "exportweb", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isRuntimeModeCommand(tt.cmdName); got != tt.want {
				t.Fatalf("isRuntimeModeCommand(%q) = %v, want %v", tt.cmdName, got, tt.want)
			}
		})
	}
}
