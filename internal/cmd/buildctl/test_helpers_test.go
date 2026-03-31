package main

import "testing"

func mustDefaultRuntimeVersion(t *testing.T) string {
	t.Helper()

	version, err := defaultRuntimeVersion()
	if err != nil {
		t.Fatal(err)
	}
	return version
}
