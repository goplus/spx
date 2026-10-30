//go:build windows

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

package runtimebundle

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

func TestWindowsPrivateDACLIsAppliedAtCreation(t *testing.T) {
	root := filepath.Join(t.TempDir(), "cache")
	permissions := windowsPermissions{}
	if err := permissions.EnsureDir(root); err != nil {
		t.Fatal(err)
	}
	if err := verifyProtectedPrivateDACL(root); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(root, string(NamespaceEngine))
	if err := permissions.EnsureDir(child); err != nil {
		t.Fatal(err)
	}
	if err := verifyProtectedPrivateDACL(child); err != nil {
		t.Fatal(err)
	}
	payload := filepath.Join(child, "payload")
	if err := os.WriteFile(payload, []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyPrivateDACL(payload); err != nil {
		t.Fatal(err)
	}

	key := filepath.Join(child, strings.Repeat("d", 64))
	lease, err := (CrossProcessLockProvider{}).AcquireExclusive(context.Background(), key)
	if err != nil {
		t.Fatal(err)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	if err := verifyPrivateDACL(key + ".lock"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(key + ".lock"); err != nil {
		t.Fatal(err)
	}
}

func TestAcquireCreatesProtectedWindowsCache(t *testing.T) {
	root := filepath.Join(t.TempDir(), "acquire-cache")
	path, err := acquirePath(context.Background(), root, FetchSpec{
		Name: "manifest.json", URL: "https://example.invalid/manifest.json",
		Size: 2, SHA256: testDigest("{}"),
		Fetch: func(_ context.Context, _ string, dst io.Writer) error {
			_, err := io.WriteString(dst, "{}")
			return err
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyProtectedPrivateDACL(root); err != nil {
		t.Fatal(err)
	}
	if err := verifyPrivateDACL(path); err != nil {
		t.Fatal(err)
	}
	if err := verifyPrivateDACL(path + ".lock"); err != nil {
		t.Fatal(err)
	}
}

type recordingWindowsPermissions struct {
	windowsPermissions
	directoryCalls int
	failDirectory  string
}

func (p *recordingWindowsPermissions) EnsureDir(path string) error {
	p.directoryCalls++
	if path == p.failDirectory {
		return os.ErrPermission
	}
	return p.windowsPermissions.EnsureDir(path)
}

func TestCacheUsesConfiguredWindowsPermissions(t *testing.T) {
	permissions := &recordingWindowsPermissions{}
	cache := NewProcessLocalCache(filepath.Join(t.TempDir(), "cache"))
	cache.Permissions = permissions
	zipPath := writeTestZip(t, testZipEntry{name: "runtime", mode: 0o755, data: "trusted"})
	target, err := materializeTestPath(cache, t.Context(), NamespaceEngine, zipPath, nil)
	if err != nil {
		t.Fatal(err)
	}
	if permissions.directoryCalls == 0 {
		t.Fatal("materialization did not use configured permissions")
	}
	for _, path := range []string{cache.Root, filepath.Dir(target), target, filepath.Join(target, "runtime")} {
		if err := verifyPrivateDACL(path); err != nil {
			t.Fatal(err)
		}
	}
	calls := permissions.directoryCalls
	if hit, err := materializeTestPath(cache, t.Context(), NamespaceEngine, zipPath, nil); err != nil || hit != target {
		t.Fatalf("cache hit = %q, %v; want %q", hit, err, target)
	}
	if permissions.directoryCalls <= calls {
		t.Fatal("cache hit bypassed configured permissions")
	}
	calls = permissions.directoryCalls
	if entries, err := cache.List(NamespaceEngine); err != nil || len(entries) != 1 || entries[0] != target {
		t.Fatalf("List = %v, %v", entries, err)
	}
	if permissions.directoryCalls != calls {
		t.Fatal("listing cache attempted to create directories")
	}
	if err := cache.Remove(NamespaceEngine, filepath.Base(target)); err == nil {
		t.Fatal("Remove deleted a valid cache target")
	}
	if permissions.directoryCalls <= calls || cache.Permissions != permissions {
		t.Fatal("cache operation replaced or bypassed configured permissions")
	}
	if data, err := os.ReadFile(filepath.Join(target, "runtime")); err != nil || string(data) != "trusted" {
		t.Fatalf("valid cache after Remove = %q, %v", data, err)
	}

	permissions.failDirectory = filepath.Dir(target)
	materialized, err := cache.Materialize(t.Context(), NamespaceEngine, zipPath, nil)
	if materialized != nil {
		_ = materialized.Close()
		t.Fatal("Materialize returned a result despite namespace permission failure")
	}
	if !errors.Is(err, os.ErrPermission) || !strings.Contains(err.Error(), "protect cache namespace") {
		t.Fatalf("Materialize error = %v; want wrapped namespace permission error", err)
	}
	if data, err := os.ReadFile(filepath.Join(target, "runtime")); err != nil || string(data) != "trusted" {
		t.Fatalf("valid cache after namespace permission failure = %q, %v", data, err)
	}
	if entries, err := cache.List(NamespaceEngine); err != nil || len(entries) != 1 || entries[0] != target {
		t.Fatalf("List after namespace permission failure = %v, %v", entries, err)
	}
}

func TestVerifyPrivateSecurityDescriptorUsesBinarySIDs(t *testing.T) {
	user := testWindowsSID(t, "S-1-5-21-111-222-333-1001")
	systemInACL := testWindowsSID(t, "S-1-5-18")
	system, err := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	if err != nil {
		t.Fatal(err)
	}
	descriptor := testPrivateSecurityDescriptor(t, true, []windows.EXPLICIT_ACCESS{
		privateExplicitAccess(user, windows.TRUSTEE_IS_USER),
		privateExplicitAccess(systemInACL, windows.TRUSTEE_IS_WELL_KNOWN_GROUP),
	})

	if err := verifyPrivateSecurityDescriptor("test", descriptor, user, system, true, true); err != nil {
		t.Fatalf("binary-equivalent LocalSystem SID was rejected: %v", err)
	}
}

func TestVerifyPrivateSecurityDescriptorAcceptsExpectedForms(t *testing.T) {
	user := testWindowsSID(t, "S-1-5-21-111-222-333-1001")
	system := testWindowsSID(t, "S-1-5-18")

	t.Run("unprotected when optional", func(t *testing.T) {
		descriptor := testPrivateSecurityDescriptor(t, false, testPrivateACEs(user, system))
		if err := verifyPrivateSecurityDescriptor("test", descriptor, user, system, false, true); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("inherited leaf with mapped mask", func(t *testing.T) {
		// Build inherited ACEs directly: ID marks inheritance and FA maps file full control.
		descriptor, err := windows.SecurityDescriptorFromString(
			"D:(A;ID;FA;;;" + user.String() + ")(A;ID;FA;;;" + system.String() + ")")
		if err != nil {
			t.Fatal(err)
		}
		if err := verifyPrivateSecurityDescriptor("test", descriptor, user, system, false, false); err != nil {
			t.Fatal(err)
		}
	})
}

func TestVerifyPrivateSecurityDescriptorRejectsUnexpectedACL(t *testing.T) {
	user := testWindowsSID(t, "S-1-5-21-111-222-333-1001")
	system := testWindowsSID(t, "S-1-5-18")
	other := testWindowsSID(t, "S-1-5-32-545")

	tests := []struct {
		name             string
		protected        bool
		requireProtected bool
		entries          func() []windows.EXPLICIT_ACCESS
	}{
		{
			name:             "unprotected",
			requireProtected: true,
			entries:          func() []windows.EXPLICIT_ACCESS { return testPrivateACEs(user, system) },
		},
		{
			name:      "missing principal",
			protected: true,
			entries: func() []windows.EXPLICIT_ACCESS {
				return []windows.EXPLICIT_ACCESS{privateExplicitAccess(user, windows.TRUSTEE_IS_USER)}
			},
		},
		{
			name:      "unexpected principal",
			protected: true,
			entries: func() []windows.EXPLICIT_ACCESS {
				return []windows.EXPLICIT_ACCESS{
					privateExplicitAccess(user, windows.TRUSTEE_IS_USER),
					privateExplicitAccess(other, windows.TRUSTEE_IS_GROUP),
				}
			},
		},
		{
			name:      "denied ACE",
			protected: true,
			entries: func() []windows.EXPLICIT_ACCESS {
				entries := testPrivateACEs(user, system)
				entries[0].AccessMode = windows.DENY_ACCESS
				return entries
			},
		},
		{
			name:      "partial mask",
			protected: true,
			entries: func() []windows.EXPLICIT_ACCESS {
				entries := testPrivateACEs(user, system)
				entries[0].AccessPermissions = windows.GENERIC_READ
				return entries
			},
		},
		{
			name:      "extra mask bits",
			protected: true,
			entries: func() []windows.EXPLICIT_ACCESS {
				entries := testPrivateACEs(user, system)
				entries[0].AccessPermissions = windows.GENERIC_ALL | windows.READ_CONTROL
				return entries
			},
		},
		{
			name:      "directory ACE does not inherit",
			protected: true,
			entries: func() []windows.EXPLICIT_ACCESS {
				entries := testPrivateACEs(user, system)
				entries[0].Inheritance = windows.NO_INHERITANCE
				return entries
			},
		},
		{
			name:      "directory ACE is inherit-only",
			protected: true,
			entries: func() []windows.EXPLICIT_ACCESS {
				entries := testPrivateACEs(user, system)
				entries[0].Inheritance |= windows.INHERIT_ONLY
				return entries
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			descriptor := testPrivateSecurityDescriptor(t, test.protected, test.entries())
			err := verifyPrivateSecurityDescriptor("test", descriptor, user, system, test.requireProtected, true)
			if !errors.Is(err, ErrPrivateDACLUnsupported) {
				t.Fatalf("error = %v; want ErrPrivateDACLUnsupported", err)
			}
		})
	}
}

func TestVerifyPrivateSecurityDescriptorRejectsMissingDACL(t *testing.T) {
	user := testWindowsSID(t, "S-1-5-21-111-222-333-1001")
	system := testWindowsSID(t, "S-1-5-18")
	descriptor, err := windows.NewSecurityDescriptor()
	if err != nil {
		t.Fatal(err)
	}

	err = verifyPrivateSecurityDescriptor("test", descriptor, user, system, false, true)
	if !errors.Is(err, ErrPrivateDACLUnsupported) {
		t.Fatalf("error = %v; want ErrPrivateDACLUnsupported", err)
	}
}

func testWindowsSID(t *testing.T, value string) *windows.SID {
	t.Helper()
	sid, err := windows.StringToSid(value)
	if err != nil {
		t.Fatal(err)
	}
	return sid
}

func testPrivateACEs(user, system *windows.SID) []windows.EXPLICIT_ACCESS {
	return []windows.EXPLICIT_ACCESS{
		privateExplicitAccess(user, windows.TRUSTEE_IS_USER),
		privateExplicitAccess(system, windows.TRUSTEE_IS_WELL_KNOWN_GROUP),
	}
}

func testPrivateSecurityDescriptor(t *testing.T, protected bool, entries []windows.EXPLICIT_ACCESS) *windows.SECURITY_DESCRIPTOR {
	t.Helper()
	acl, err := windows.ACLFromEntries(entries, nil)
	if err != nil {
		t.Fatal(err)
	}
	descriptor, err := windows.NewSecurityDescriptor()
	if err != nil {
		t.Fatal(err)
	}
	if err := descriptor.SetDACL(acl, true, false); err != nil {
		t.Fatal(err)
	}
	if protected {
		if err := descriptor.SetControl(windows.SE_DACL_PROTECTED, windows.SE_DACL_PROTECTED); err != nil {
			t.Fatal(err)
		}
	}
	return descriptor
}
