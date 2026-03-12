package engine

import (
	"testing"

	"github.com/goplus/spx/v2/internal/enginewrap"
)

func TestManagersDefaultsAndInjection(t *testing.T) {
	SetManagers(nil)
	defaultPtr := Managers()
	if defaultPtr == nil {
		t.Fatal("Managers() returned nil default manager set")
	}

	custom := &enginewrap.EngineManagers{}
	SetManagers(custom)
	if got := Managers(); got != custom {
		t.Fatalf("Managers() = %p, want %p", got, custom)
	}

	SetManagers(nil)
	if got := Managers(); got != defaultPtr {
		t.Fatalf("Managers() after reset = %p, want %p", got, defaultPtr)
	}
}
