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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/goplus/spx/v3/internal/strictjson"
	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

// SchemaV1 is the manifest schema implemented by this package. It is kept
// separate from the driver protocol version: a bundle can be consumed by
// more than one driver protocol generation.
const SchemaV1 = "runtimebundle/v1"

const (
	// MaxEntries is the maximum number of central-directory entries accepted
	// from an untrusted archive.
	MaxEntries = 10_000
	// MaxEntrySize is the maximum uncompressed size of one entry.
	MaxEntrySize int64 = 512 << 20
	// MaxTotalSize is the maximum total uncompressed size of an archive.
	MaxTotalSize int64 = 4 << 30
	// MaxArchiveBytes is the maximum compressed ZIP byte length inspected by
	// the verifier. It bounds central-directory/offset work independently of
	// the uncompressed quota.
	MaxArchiveBytes int64 = 8 << 30
	// MaxCompressionRatio is the maximum declared uncompressed/compressed
	// ratio. It is deliberately applied even when the archive is otherwise
	// below the byte quotas.
	MaxCompressionRatio uint64 = 200
)

// Limits bounds work performed while inspecting an untrusted ZIP. A zero
// field means the corresponding v1 default. Negative values are invalid.
type Limits struct {
	MaxEntries          int
	MaxEntrySize        int64
	MaxTotalSize        int64
	MaxArchiveBytes     int64
	MaxCompressionRatio uint64
}

// DefaultLimits is a conservative v1 limit set. Callers may copy it and
// tighten individual fields for a particular artifact class.
var DefaultLimits = Limits{
	MaxEntries:          MaxEntries,
	MaxEntrySize:        MaxEntrySize,
	MaxTotalSize:        MaxTotalSize,
	MaxArchiveBytes:     MaxArchiveBytes,
	MaxCompressionRatio: MaxCompressionRatio,
}

func (l Limits) withDefaults() (Limits, error) {
	if l.MaxEntries == 0 {
		l.MaxEntries = MaxEntries
	}
	if l.MaxEntrySize == 0 {
		l.MaxEntrySize = MaxEntrySize
	}
	if l.MaxTotalSize == 0 {
		l.MaxTotalSize = MaxTotalSize
	}
	if l.MaxArchiveBytes == 0 {
		l.MaxArchiveBytes = MaxArchiveBytes
	}
	if l.MaxCompressionRatio == 0 {
		l.MaxCompressionRatio = MaxCompressionRatio
	}
	if l.MaxEntries < 0 || l.MaxEntrySize < 0 || l.MaxTotalSize < 0 || l.MaxArchiveBytes < 0 {
		return Limits{}, fmt.Errorf("runtimebundle: negative archive limit")
	}
	if l.MaxEntries == 0 || l.MaxEntrySize == 0 || l.MaxTotalSize == 0 || l.MaxArchiveBytes == 0 || l.MaxCompressionRatio == 0 {
		return Limits{}, fmt.Errorf("runtimebundle: archive limits must be positive")
	}
	if l.MaxEntrySize > l.MaxTotalSize {
		return Limits{}, fmt.Errorf("runtimebundle: max entry size %d exceeds max total size %d", l.MaxEntrySize, l.MaxTotalSize)
	}
	return l, nil
}

// Namespace identifies an independent content-addressed cache namespace.
// Keeping namespaces in separate directories prevents an engine artifact from
// accidentally satisfying a bridge or project lookup with the same digest.
type Namespace string

const (
	NamespaceEngine  Namespace = "engine"
	NamespaceBridge  Namespace = "bridge"
	NamespaceProject Namespace = "project"
)

func (n Namespace) valid() bool {
	switch n {
	case NamespaceEngine, NamespaceBridge, NamespaceProject:
		return true
	default:
		return false
	}
}

// Entry is one regular-file or directory entry in a bundle manifest. SHA256
// is always the lower-case, full 64-hex-character SHA-256 digest of the
// uncompressed file bytes. Directory entries have size zero and the SHA-256
// digest of the empty byte sequence.
type Entry struct {
	Name   string `json:"name"`
	Mode   uint32 `json:"mode"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// Bundle is the self-describing manifest used by the runtime cache. Digest is
// metadata and is intentionally excluded from the identity calculation, so a
// manifest cannot hash itself. A missing Schema is accepted when reading a
// v1 fixture and is normalized to SchemaV1.
type Bundle struct {
	Schema    string    `json:"schema,omitempty"`
	Namespace Namespace `json:"namespace,omitempty"`
	Entries   []Entry   `json:"entries"`
	Digest    string    `json:"digest,omitempty"`
}

// Manifest and BundleManifest are compatibility aliases for callers that
// prefer to call the same value a manifest.
type Manifest = Bundle
type BundleManifest = Bundle

var (
	ErrInvalidManifest             = errors.New("runtimebundle: invalid manifest")
	ErrInvalidEntryName            = errors.New("runtimebundle: invalid entry name")
	ErrDigestMismatch              = errors.New("runtimebundle: digest mismatch")
	ErrArchiveLimit                = errors.New("runtimebundle: archive limit exceeded")
	ErrUnsafeArchive               = errors.New("runtimebundle: unsafe archive")
	ErrUnsupportedArchiveEntry     = errors.New("runtimebundle: unsupported archive entry")
	ErrCrossProcessLockUnsupported = errors.New("runtimebundle: cross-process locking is unavailable on this platform")
	ErrPrivateDACLUnsupported      = errors.New("runtimebundle: private Windows DACL is unavailable")
	ErrGCUnsupported               = errors.New("runtimebundle: cache GC is not implemented")
)

func (e Entry) isDir() bool {
	return e.Mode&uint32(fs.ModeDir) != 0
}

func (e Entry) normalized() (Entry, string, error) {
	if e.Size < 0 {
		return Entry{}, "", fmt.Errorf("%w: %q has negative size", ErrInvalidManifest, e.Name)
	}
	if e.isDir() && e.Size != 0 {
		return Entry{}, "", fmt.Errorf("%w: directory %q has size %d", ErrInvalidManifest, e.Name, e.Size)
	}
	if e.Mode&^uint32(fs.ModeDir|0o777) != 0 {
		return Entry{}, "", fmt.Errorf("%w: entry %q has unsupported mode %#o", ErrInvalidManifest, e.Name, e.Mode)
	}
	if err := validateSHA256(e.SHA256); err != nil {
		return Entry{}, "", fmt.Errorf("%w: entry %q: %v", ErrInvalidManifest, e.Name, err)
	}
	if e.isDir() {
		empty := sha256.Sum256(nil)
		if e.SHA256 != hex.EncodeToString(empty[:]) {
			return Entry{}, "", fmt.Errorf("%w: directory %q must use the empty-file SHA-256", ErrInvalidManifest, e.Name)
		}
	}
	name, key, isDir, err := normalizeEntryName(e.Name)
	if err != nil {
		return Entry{}, "", err
	}
	if isDir != e.isDir() {
		return Entry{}, "", fmt.Errorf("%w: entry %q directory mode/name mismatch", ErrInvalidManifest, e.Name)
	}
	e.Name = name
	return e, key, nil
}

func validateSHA256(value string) error {
	if len(value) != sha256.Size*2 {
		return fmt.Errorf("sha256 must be a full %d-hex-character digest", sha256.Size*2)
	}
	if _, err := hex.DecodeString(value); err != nil || value != strings.ToLower(value) {
		return fmt.Errorf("sha256 must be lower-case hexadecimal")
	}
	return nil
}

// Validate checks structure, names, modes, duplicate/case-fold collisions,
// sizes and all entry digests. It does not access the files named by the
// manifest.
func (b Bundle) Validate() error {
	return b.ValidateWithLimits(DefaultLimits)
}

// ValidateWithLimits is Validate with caller-supplied size and count limits.
func (b Bundle) ValidateWithLimits(limits Limits) error {
	limits, err := limits.withDefaults()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidManifest, err)
	}
	if b.Schema != "" && b.Schema != SchemaV1 {
		return fmt.Errorf("%w: unsupported schema %q", ErrInvalidManifest, b.Schema)
	}
	if b.Namespace != "" && !b.Namespace.valid() {
		return fmt.Errorf("%w: unsupported namespace %q", ErrInvalidManifest, b.Namespace)
	}
	if len(b.Entries) > limits.MaxEntries {
		return fmt.Errorf("%w: %d entries exceeds limit %d", ErrArchiveLimit, len(b.Entries), limits.MaxEntries)
	}
	seen := make(map[string]Entry, len(b.Entries))
	seenNames := make(map[string]string, len(b.Entries))
	var total int64
	for _, original := range b.Entries {
		entry, key, err := original.normalized()
		if err != nil {
			return err
		}
		if entry.Size > limits.MaxEntrySize {
			return fmt.Errorf("%w: entry %q size %d exceeds limit %d", ErrArchiveLimit, entry.Name, entry.Size, limits.MaxEntrySize)
		}
		if entry.Size > limits.MaxTotalSize-total {
			return fmt.Errorf("%w: total size exceeds limit %d", ErrArchiveLimit, limits.MaxTotalSize)
		}
		total += entry.Size
		if previous, ok := seenNames[key]; ok {
			if previous == entry.Name {
				return fmt.Errorf("%w: duplicate entry %q", ErrUnsafeArchive, entry.Name)
			}
			return fmt.Errorf("%w: case-fold/normalization collision between %q and %q", ErrUnsafeArchive, previous, entry.Name)
		}
		seen[key] = entry
		seenNames[key] = entry.Name
	}
	for key, entry := range seen {
		parent := key
		for {
			index := strings.LastIndexByte(parent, '/')
			if index < 0 {
				break
			}
			parent = parent[:index]
			if parentEntry, ok := seen[parent]; ok && !parentEntry.isDir() {
				return fmt.Errorf("%w: file %q is also a parent of %q", ErrUnsafeArchive, parentEntry.Name, entry.Name)
			}
		}
	}
	if b.Digest != "" {
		if err := validateSHA256(b.Digest); err != nil {
			return fmt.Errorf("%w: bundle digest: %v", ErrInvalidManifest, err)
		}
		digest, err := b.IdentityDigest()
		if err != nil {
			return err
		}
		if b.Digest != digest {
			return fmt.Errorf("%w: manifest digest %s does not match identity %s", ErrDigestMismatch, b.Digest, digest)
		}
	}
	return nil
}

type canonicalBundle struct {
	Schema    string    `json:"schema"`
	Namespace Namespace `json:"namespace,omitempty"`
	Entries   []Entry   `json:"entries"`
}

func (b Bundle) canonical() (canonicalBundle, error) {
	// Digest is a checksum over this canonical form, so it must not be
	// validated while constructing the form itself (otherwise validation would
	// recurse through IdentityDigest indefinitely).
	withoutDigest := b
	withoutDigest.Digest = ""
	if err := withoutDigest.ValidateWithLimits(DefaultLimits); err != nil {
		return canonicalBundle{}, err
	}
	out := canonicalBundle{Schema: b.Schema, Namespace: b.Namespace, Entries: make([]Entry, 0, len(b.Entries))}
	if out.Schema == "" {
		out.Schema = SchemaV1
	}
	for _, original := range b.Entries {
		entry, _, err := original.normalized()
		if err != nil {
			return canonicalBundle{}, err
		}
		out.Entries = append(out.Entries, entry)
	}
	sort.Slice(out.Entries, func(i, j int) bool {
		return out.Entries[i].Name < out.Entries[j].Name
	})
	return out, nil
}

// CanonicalBytes returns deterministic manifest bytes. Digest is omitted from
// these bytes by design; callers can put the resulting digest in Bundle.Digest
// after calculating it.
func (b Bundle) CanonicalBytes() ([]byte, error) {
	canonical, err := b.canonical()
	if err != nil {
		return nil, err
	}
	return json.Marshal(canonical)
}

// IdentityDigest computes the full SHA-256 identity of the canonical bundle
// manifest. It is not truncated and does not include Bundle.Digest.
func (b Bundle) IdentityDigest() (string, error) {
	data, err := b.CanonicalBytes()
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// Identity is a convenience form of IdentityDigest. Invalid manifests return
// an empty string; code that needs diagnostics should use IdentityDigest.
func (b Bundle) Identity() string {
	digest, _ := b.IdentityDigest()
	return digest
}

// WithDigest returns a normalized copy with its identity digest populated.
func (b Bundle) WithDigest() (Bundle, error) {
	withoutDigest := b
	withoutDigest.Digest = ""
	if err := withoutDigest.ValidateWithLimits(DefaultLimits); err != nil {
		return Bundle{}, err
	}
	digest, err := b.IdentityDigest()
	if err != nil {
		return Bundle{}, err
	}
	if b.Schema == "" {
		b.Schema = SchemaV1
	}
	b.Digest = digest
	return b, nil
}

// MarshalCanonicalJSON is an alias useful to callers writing a manifest
// beside a materialized cache entry.
func (b Bundle) MarshalCanonicalJSON() ([]byte, error) {
	return b.CanonicalBytes()
}

// ParseManifest parses and validates a v1 manifest. Manifest files are
// untrusted identity inputs, so unknown fields, duplicate object keys and
// trailing JSON values are rejected instead of being silently ignored.
func ParseManifest(data []byte) (Bundle, error) {
	var decoded *Bundle
	if err := strictjson.Decode(data, &decoded); err != nil {
		return Bundle{}, fmt.Errorf("%w: decode JSON: %v", ErrInvalidManifest, err)
	}
	if decoded == nil {
		return Bundle{}, fmt.Errorf("%w: decode JSON: manifest top-level value must be an object", ErrInvalidManifest)
	}
	b := *decoded
	if err := b.Validate(); err != nil {
		return Bundle{}, err
	}
	if b.Schema == "" {
		b.Schema = SchemaV1
	}
	return b, nil
}

func normalizeEntryName(name string) (normalized, folded string, isDir bool, err error) {
	if name == "" || !utf8.ValidString(name) {
		return "", "", false, fmt.Errorf("%w: %q", ErrInvalidEntryName, name)
	}
	if strings.IndexByte(name, 0) >= 0 {
		return "", "", false, fmt.Errorf("%w: %q contains NUL", ErrInvalidEntryName, name)
	}
	if strings.ContainsRune(name, '\\') {
		return "", "", false, fmt.Errorf("%w: %q contains backslash", ErrInvalidEntryName, name)
	}
	if strings.ContainsRune(name, ':') {
		// Reject drive paths and NTFS alternate-data-stream spellings on every
		// platform so a bundle has one portable lexical policy.
		return "", "", false, fmt.Errorf("%w: volume/alternate-data-stream path %q", ErrInvalidEntryName, name)
	}
	if strings.HasPrefix(name, "/") || strings.HasPrefix(name, "//") || isDriveAbsolute(name) {
		return "", "", false, fmt.Errorf("%w: absolute path %q", ErrInvalidEntryName, name)
	}
	isDir = strings.HasSuffix(name, "/")
	base := name
	if isDir {
		base = strings.TrimSuffix(base, "/")
	}
	if base == "" || strings.HasSuffix(base, "/") || strings.Contains(base, "//") {
		return "", "", false, fmt.Errorf("%w: non-canonical path %q", ErrInvalidEntryName, name)
	}
	if base == CompleteMarkerName || base == CacheManifestName {
		return "", "", false, fmt.Errorf("%w: reserved cache metadata name %q", ErrUnsafeArchive, name)
	}
	parts := strings.Split(base, "/")
	for _, part := range parts {
		if part == "" || part == "." || part == ".." {
			return "", "", false, fmt.Errorf("%w: traversal or empty path component in %q", ErrInvalidEntryName, name)
		}
		if err := validatePortableComponent(part); err != nil {
			return "", "", false, fmt.Errorf("%w: %q: %v", ErrInvalidEntryName, name, err)
		}
	}
	normalized = base
	if isDir {
		normalized += "/"
	}
	// NFC plus Unicode case folding catches both common case-insensitive filesystem
	// aliases and composed/decomposed Unicode aliases. This key is only used
	// for collision detection; original UTF-8 names remain in the manifest.
	folded = cases.Fold().String(norm.NFC.String(base))
	return normalized, folded, isDir, nil
}

func isDriveAbsolute(name string) bool {
	if len(name) < 3 {
		return false
	}
	c := name[0]
	return ((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')) && name[1] == ':' && name[2] == '/'
}

func validatePortableComponent(component string) error {
	if strings.HasSuffix(component, ".") || strings.HasSuffix(component, " ") {
		return fmt.Errorf("trailing dot/space is not portable on Windows")
	}
	if strings.ContainsAny(component, `<>:"|?*`) {
		return fmt.Errorf("contains a Windows-reserved character")
	}
	for _, character := range component {
		if character < 0x20 {
			return fmt.Errorf("contains a control character")
		}
	}
	folded := cases.Fold().String(component)
	base := folded
	if index := strings.IndexByte(base, '.'); index >= 0 {
		base = base[:index]
	}
	base = strings.TrimRight(base, " .")
	switch base {
	case "con", "prn", "aux", "nul", "clock$", "conin$", "conout$":
		return fmt.Errorf("reserved Windows device name")
	}
	if len(base) == 4 && (strings.HasPrefix(base, "com") || strings.HasPrefix(base, "lpt")) && base[3] >= '1' && base[3] <= '9' {
		return fmt.Errorf("reserved Windows device name")
	}
	for _, prefix := range []string{"com", "lpt"} {
		for _, digit := range []string{"¹", "²", "³"} {
			if base == prefix+digit {
				return fmt.Errorf("reserved Windows device name")
			}
		}
	}
	return nil
}

func entryPathKey(name string) string {
	_, key, _, err := normalizeEntryName(name)
	if err != nil {
		return ""
	}
	return key
}

func manifestEntriesEqual(a, b Bundle) error {
	left, err := a.canonical()
	if err != nil {
		return err
	}
	right, err := b.canonical()
	if err != nil {
		return err
	}
	if left.Schema != right.Schema || left.Namespace != right.Namespace || len(left.Entries) != len(right.Entries) {
		return fmt.Errorf("%w: manifest identity fields differ", ErrDigestMismatch)
	}
	for i := range left.Entries {
		if left.Entries[i] != right.Entries[i] {
			return fmt.Errorf("%w: entry %q differs", ErrDigestMismatch, left.Entries[i].Name)
		}
	}
	return nil
}

// Ensure fs.FileMode remains the source of truth for mode type bits. This
// assignment also makes accidental widening to platform-specific bits fail at
// compile time if the mode representation changes.
var _ fs.FileMode = fs.ModeDir
