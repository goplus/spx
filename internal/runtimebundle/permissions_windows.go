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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

// windowsPermissions creates cache directories with a protected DACL. The
// ACEs are inheritable so files and child directories receive the same policy
// at creation time; no post-create chmod/security window is used.
type windowsPermissions struct{}

func (windowsPermissions) EnsureDir(path string) error {
	if path == "" {
		return fmt.Errorf("runtimebundle: empty Windows cache directory")
	}
	path = filepath.Clean(path)
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return fmt.Errorf("%w: Windows cache directory is not a real directory: %s", ErrUnsafeArchive, path)
		}
		return verifyProtectedPrivateDACL(path)
	} else if !os.IsNotExist(err) {
		return err
	}
	parent := filepath.Dir(path)
	if parent == path {
		return fmt.Errorf("runtimebundle: Windows cache directory parent does not exist: %s", path)
	}
	if err := (windowsPermissions{}).EnsureDir(parent); err != nil {
		// Existing non-cache ancestors such as the user profile or temp root
		// are allowed to keep their own ACL; only missing components are made
		// private by this primitive.
		info, statErr := os.Lstat(parent)
		if statErr != nil || info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return err
		}
	}
	attrs, err := privateSecurityAttributes()
	if err != nil {
		return err
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	err = windows.CreateDirectory(name, attrs)
	if err != nil && !errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		return err
	}
	return verifyPrivateDACL(path)
}

func (windowsPermissions) EnsureFile(path string, executable bool) error {
	if path == "" {
		return fmt.Errorf("runtimebundle: empty Windows cache file")
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("%w: Windows cache file is not regular: %s", ErrUnsafeArchive, path)
	}
	return verifyPrivateDACL(path)
}

func privateSecurityAttributes() (*windows.SecurityAttributes, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, fmt.Errorf("runtimebundle: resolve current Windows user SID: %w", err)
	}
	system, err := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	if err != nil {
		return nil, fmt.Errorf("runtimebundle: resolve LocalSystem SID: %w", err)
	}
	entries := []windows.EXPLICIT_ACCESS{privateExplicitAccess(user.User.Sid, windows.TRUSTEE_IS_USER)}
	if !user.User.Sid.Equals(system) {
		entries = append(entries, privateExplicitAccess(system, windows.TRUSTEE_IS_WELL_KNOWN_GROUP))
	}
	acl, err := windows.ACLFromEntries(entries, nil)
	if err != nil {
		return nil, fmt.Errorf("runtimebundle: build private Windows DACL: %w", err)
	}
	sd, err := windows.NewSecurityDescriptor()
	if err != nil {
		return nil, fmt.Errorf("runtimebundle: initialize Windows security descriptor: %w", err)
	}
	if err := sd.SetDACL(acl, true, false); err != nil {
		return nil, fmt.Errorf("runtimebundle: set private Windows DACL: %w", err)
	}
	if err := sd.SetControl(windows.SE_DACL_PROTECTED, windows.SE_DACL_PROTECTED); err != nil {
		return nil, fmt.Errorf("runtimebundle: protect Windows DACL: %w", err)
	}
	return &windows.SecurityAttributes{
		Length:             uint32(unsafe.Sizeof(windows.SecurityAttributes{})),
		SecurityDescriptor: sd,
		InheritHandle:      0,
	}, nil
}

func privateExplicitAccess(sid *windows.SID, trusteeType windows.TRUSTEE_TYPE) windows.EXPLICIT_ACCESS {
	return windows.EXPLICIT_ACCESS{
		AccessPermissions: windows.GENERIC_ALL,
		AccessMode:        windows.SET_ACCESS,
		Inheritance:       windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeType:  trusteeType,
			TrusteeValue: windows.TrusteeValueFromSID(sid),
		},
	}
}

func verifyProtectedPrivateDACL(path string) error {
	return verifyPrivateDACLMode(path, true)
}

func verifyPrivateDACL(path string) error {
	return verifyPrivateDACLMode(path, false)
}

func verifyPrivateDACLMode(path string, requireProtected bool) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return fmt.Errorf("runtimebundle: resolve current Windows user SID: %w", err)
	}
	system, err := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	if err != nil {
		return fmt.Errorf("runtimebundle: resolve LocalSystem SID: %w", err)
	}
	actual, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return fmt.Errorf("runtimebundle: inspect Windows cache DACL: %w", err)
	}
	return verifyPrivateSecurityDescriptor(path, actual, user.User.Sid, system, requireProtected, info.IsDir())
}

const fileAllAccessMask windows.ACCESS_MASK = windows.STANDARD_RIGHTS_REQUIRED | windows.SYNCHRONIZE | 0x1ff

func verifyPrivateSecurityDescriptor(path string, descriptor *windows.SECURITY_DESCRIPTOR, user, system *windows.SID, requireProtected, isDirectory bool) error {
	control, _, err := descriptor.Control()
	if err != nil {
		return privateDACLError(path, "read Windows security descriptor control", err)
	}
	if control&windows.SE_DACL_PRESENT == 0 {
		return privateDACLError(path, "cache path has no Windows DACL", nil)
	}
	if requireProtected && control&windows.SE_DACL_PROTECTED == 0 {
		return privateDACLError(path, "cache path DACL is not protected", nil)
	}

	dacl, _, err := descriptor.DACL()
	if err != nil {
		return privateDACLError(path, "read Windows DACL", err)
	}
	if dacl == nil {
		return privateDACLError(path, "cache path has a NULL Windows DACL", nil)
	}

	wantACEs := uint16(2)
	if user.Equals(system) {
		wantACEs = 1
	}
	if dacl.AceCount != wantACEs {
		return privateDACLError(path, "cache path has an unexpected Windows ACE count", nil)
	}

	seenUser := false
	seenSystem := false
	for index := uint16(0); index < dacl.AceCount; index++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, uint32(index), &ace); err != nil {
			return privateDACLError(path, "enumerate Windows DACL", err)
		}
		if ace == nil || ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE {
			return privateDACLError(path, "cache path has an unexpected Windows ACE type", nil)
		}
		if ace.Mask != windows.GENERIC_ALL && ace.Mask != fileAllAccessMask {
			return privateDACLError(path, "cache path Windows ACE does not grant exactly full control", nil)
		}
		if !validPrivateACEFlags(ace.Header.AceFlags, isDirectory) {
			return privateDACLError(path, "cache path has unexpected Windows ACE inheritance", nil)
		}

		trustee, ok := accessAllowedACESID(ace)
		if !ok {
			return privateDACLError(path, "cache path has a malformed Windows ACE trustee", nil)
		}
		matchesUser := trustee.Equals(user)
		matchesSystem := trustee.Equals(system)
		if !matchesUser && !matchesSystem {
			return privateDACLError(path, "cache path grants an unexpected Windows principal", nil)
		}
		if (matchesUser && seenUser) || (matchesSystem && seenSystem) {
			return privateDACLError(path, "cache path has a duplicate Windows principal", nil)
		}
		seenUser = seenUser || matchesUser
		seenSystem = seenSystem || matchesSystem
	}
	if !seenUser || !seenSystem {
		return privateDACLError(path, "cache path is missing a required Windows principal", nil)
	}
	return nil
}

func validPrivateACEFlags(flags uint8, isDirectory bool) bool {
	const (
		inheritChildren = uint8(windows.OBJECT_INHERIT_ACE | windows.CONTAINER_INHERIT_ACE)
		inherited       = uint8(windows.INHERITED_ACE)
	)
	if isDirectory {
		return flags == inheritChildren || flags == inheritChildren|inherited
	}
	return flags == 0 || flags == inherited || flags == inheritChildren
}

func accessAllowedACESID(ace *windows.ACCESS_ALLOWED_ACE) (*windows.SID, bool) {
	const minimumSIDSize = 8
	sidOffset := unsafe.Offsetof(ace.SidStart)
	if uintptr(ace.Header.AceSize) < sidOffset+minimumSIDSize {
		return nil, false
	}
	sidBytes := unsafe.Slice((*byte)(unsafe.Pointer(&ace.SidStart)), int(uintptr(ace.Header.AceSize)-sidOffset))
	sidLength := minimumSIDSize + 4*int(sidBytes[1])
	if sidLength != len(sidBytes) {
		return nil, false
	}
	sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
	return sid, sid.IsValid() && sid.Len() == sidLength
}

func privateDACLError(path, reason string, err error) error {
	if err != nil {
		return fmt.Errorf("%w: %s: %v: %s", ErrPrivateDACLUnsupported, reason, err, path)
	}
	return fmt.Errorf("%w: %s: %s", ErrPrivateDACLUnsupported, reason, path)
}

func platformPermissions() Permissions { return windowsPermissions{} }

func platformMkdirPrivateRootChild(root *os.Root, name string) error {
	return (windowsPermissions{}).EnsureDir(filepath.Join(root.Name(), filepath.FromSlash(name)))
}

func platformVerifyPrivateRootPath(root *os.Root, name string) error {
	return verifyPrivateDACL(filepath.Join(root.Name(), name))
}
