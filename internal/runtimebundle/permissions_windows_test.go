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
		entries := testPrivateACEs(user, system)
		for i := range entries {
			entries[i].AccessPermissions = fileAllAccessMask
			entries[i].Inheritance = windows.INHERITED_ACCESS_ENTRY
		}
		descriptor := testPrivateSecurityDescriptor(t, false, entries)
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
