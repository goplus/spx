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

// Package projectassets resolves the typed filesystem resource references in
// an SPX project. It mirrors the runtime project's packed/source precedence
// rules and deliberately ignores arbitrary JSON strings.
package projectassets

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sort"
	"strings"
)

const (
	packedIndexFile = "index_pack.json"
	maxConfigBytes  = int64(16 << 20)
)

// Config describes an SPX project and its asset pack.
type Config struct {
	ProjectDir string
	PackDir    string
	PackIndex  string
}

// Resolution is the stable result of resolving a project's typed resource
// configuration. Files contains sorted slash paths below ProjectDir that are
// outside PackDir; PackDir itself is collected separately by callers.
type Resolution struct {
	Files          []string
	HasSourceIndex bool
	HasPackedIndex bool
}

// The asset collector deliberately decodes only the resource-bearing portion
// of the project schema. Keeping this projection here makes project asset
// validation independent of the Engine-backed runtime project package while
// still rejecting malformed values for every field that can name a file.
type resourceConfig struct {
	Path string `json:"path"`
}

type projectConfig struct {
	Backdrops   []*resourceConfig `json:"backdrops"`
	Bgm         string            `json:"bgm"`
	TilemapPath string            `json:"tilemapPath"`
}

type spriteConfig struct {
	Costumes     []*resourceConfig `json:"costumes"`
	CostumeSet   *resourceConfig   `json:"costumeSet"`
	CostumeMPSet *resourceConfig   `json:"costumeMPSet"`
}

type soundConfig struct {
	Path string `json:"path"`
}

type fontFamilyConfig struct {
	Faces []resourceConfig `json:"faces"`
}

type packedConfig struct {
	project  projectConfig
	sprites  map[string]spriteConfig
	sounds   map[string]soundConfig
	fonts    map[string]fontFamilyConfig
	hasFonts bool
}

type resolver struct {
	root       *os.Root
	packDir    string
	referenced map[string]struct{}
}

// Resolve validates the pack configuration and returns its typed references.
// At least one of PackIndex or index_pack.json must exist. When both exist,
// packed root fields override source root fields; packed sprite and sound
// entries override source entries of the same name; and a packed fonts member
// is the authoritative font catalog, including when it is empty or null.
func Resolve(cfg Config) (Resolution, error) {
	if err := validateConfig(cfg); err != nil {
		return Resolution{}, err
	}
	root, err := openProjectRoot(cfg.ProjectDir)
	if err != nil {
		return Resolution{}, err
	}
	defer root.Close()

	if _, ok, err := readPinnedDir(root, cfg.PackDir); err != nil {
		return Resolution{}, err
	} else if !ok {
		return Resolution{}, fmt.Errorf("projectassets: PackDir %q is missing", cfg.PackDir)
	}

	sourceName := path.Join(cfg.PackDir, cfg.PackIndex)
	packedName := path.Join(cfg.PackDir, packedIndexFile)
	var sourceData []byte
	var hasSource bool
	if sourceName != packedName {
		sourceData, hasSource, err = readPinnedFile(root, sourceName)
		if err != nil {
			return Resolution{}, err
		}
	}
	packedData, hasPacked, err := readPinnedFile(root, packedName)
	if err != nil {
		return Resolution{}, err
	}
	if !hasSource && !hasPacked {
		if sourceName == packedName {
			return Resolution{}, fmt.Errorf("projectassets: required config %q is missing", packedName)
		}
		return Resolution{}, fmt.Errorf("projectassets: PackDir %q contains neither %q nor %q", cfg.PackDir, cfg.PackIndex, packedIndexFile)
	}

	resolved, err := parseConfig(sourceName, sourceData, packedName, packedData)
	if err != nil {
		return Resolution{}, err
	}
	collector := resolver{
		root:       root,
		packDir:    cfg.PackDir,
		referenced: make(map[string]struct{}),
	}
	rootSource := sourceName
	if hasPacked {
		rootSource = packedName
	}
	if err := collector.collectProject(rootSource, resolved.project); err != nil {
		return Resolution{}, err
	}
	if err := collector.collectSprites(resolved.sprites); err != nil {
		return Resolution{}, err
	}
	if err := collector.collectSounds(resolved.sounds); err != nil {
		return Resolution{}, err
	}
	if err := collector.collectFonts(resolved.fonts, resolved.hasFonts); err != nil {
		return Resolution{}, err
	}

	files := make([]string, 0, len(collector.referenced))
	for name := range collector.referenced {
		files = append(files, name)
	}
	sort.Strings(files)
	return Resolution{
		Files:          files,
		HasSourceIndex: hasSource,
		HasPackedIndex: hasPacked,
	}, nil
}

// Collect is the compatibility collector API used by project bundling.
func Collect(cfg Config) ([]string, error) {
	resolved, err := Resolve(cfg)
	if err != nil {
		return nil, err
	}
	return resolved.Files, nil
}

func validateConfig(cfg Config) error {
	if cfg.ProjectDir == "" {
		return errors.New("projectassets: ProjectDir is empty")
	}
	if cfg.PackDir == "" || cfg.PackDir == "." || strings.Contains(cfg.PackDir, "\\") || path.IsAbs(cfg.PackDir) || path.Clean(cfg.PackDir) != cfg.PackDir || cfg.PackDir == ".." || strings.HasPrefix(cfg.PackDir, "../") {
		return fmt.Errorf("projectassets: PackDir must be a clean non-empty relative slash path: %q", cfg.PackDir)
	}
	if cfg.PackIndex == "" || cfg.PackIndex == "." || cfg.PackIndex == ".." || strings.ContainsAny(cfg.PackIndex, "/\\\x00") {
		return fmt.Errorf("projectassets: PackIndex must be a plain file name: %q", cfg.PackIndex)
	}
	return nil
}

func parseConfig(sourceName string, sourceData []byte, packedName string, packedData []byte) (packedConfig, error) {
	sourceRoot, err := parseRoot(sourceName, sourceData)
	if err != nil {
		return packedConfig{}, err
	}
	packedRoot, err := parseRoot(packedName, packedData)
	if err != nil {
		return packedConfig{}, err
	}

	merged := make(map[string]json.RawMessage, len(sourceRoot)+len(packedRoot))
	for key, value := range sourceRoot {
		merged[key] = value
	}
	for key, value := range packedRoot {
		merged[key] = value
	}
	var config packedConfig
	if err := decodeProjectConfig(merged, &config.project); err != nil {
		name := sourceName
		if len(packedData) != 0 {
			name = packedName
		}
		return packedConfig{}, fmt.Errorf("projectassets: decode merged project config at %q: %w", name, err)
	}
	config.sprites, err = decodePackedObjects[spriteConfig](packedName, "sprites", packedRoot["sprites"])
	if err != nil {
		return packedConfig{}, err
	}
	config.sounds, err = decodePackedObjects[soundConfig](packedName, "sounds", packedRoot["sounds"])
	if err != nil {
		return packedConfig{}, err
	}
	_, config.hasFonts = packedRoot["fonts"]
	config.fonts, err = decodePackedObjects[fontFamilyConfig](packedName, "fonts", packedRoot["fonts"])
	if err != nil {
		return packedConfig{}, err
	}
	return config, nil
}

func parseRoot(name string, data []byte) (map[string]json.RawMessage, error) {
	if data == nil {
		return nil, nil
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("projectassets: decode %q: %w", name, err)
	}
	return root, nil
}

func decodeProjectConfig(root map[string]json.RawMessage, dest *projectConfig) error {
	data, err := json.Marshal(root)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, dest)
}

func decodePackedObjects[T any](indexName, section string, raw json.RawMessage) (map[string]T, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var entries map[string]json.RawMessage
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("projectassets: decode %q section %q: %w", indexName, section, err)
	}
	names := make([]string, 0, len(entries))
	for name := range entries {
		names = append(names, name)
	}
	sort.Strings(names)
	configs := make(map[string]T, len(entries))
	for _, name := range names {
		if err := validateConfigEntryName(section, name); err != nil {
			return nil, err
		}
		var config T
		if err := json.Unmarshal(entries[name], &config); err != nil {
			return nil, fmt.Errorf("projectassets: decode %q section %q entry %q: %w", indexName, section, name, err)
		}
		configs[name] = config
	}
	return configs, nil
}

func (r *resolver) collectProject(source string, config projectConfig) error {
	for _, backdrop := range config.Backdrops {
		if backdrop != nil {
			if err := r.addReference(source, r.packDir, backdrop.Path); err != nil {
				return err
			}
		}
	}
	for _, reference := range []string{config.Bgm, config.TilemapPath} {
		if err := r.addReference(source, r.packDir, reference); err != nil {
			return err
		}
	}
	return nil
}

func (r *resolver) collectSprites(packed map[string]spriteConfig) error {
	if err := forEachSorted(packed, func(name string, config spriteConfig) error {
		return r.addSpriteReferences(path.Join(r.packDir, packedIndexFile)+"#sprites/"+name, path.Join(r.packDir, "sprites", name), config)
	}); err != nil {
		return err
	}
	return collectSourceSection(r, "sprites", packed, func(source, base string, data []byte) error {
		var config spriteConfig
		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("projectassets: decode %q: %w", source, err)
		}
		return r.addSpriteReferences(source, base, config)
	})
}

func (r *resolver) addSpriteReferences(source, base string, config spriteConfig) error {
	for _, costume := range config.Costumes {
		if costume != nil {
			if err := r.addReference(source, base, costume.Path); err != nil {
				return err
			}
		}
	}
	if config.CostumeSet != nil {
		if err := r.addReference(source, base, config.CostumeSet.Path); err != nil {
			return err
		}
	}
	if config.CostumeMPSet != nil {
		if err := r.addReference(source, base, config.CostumeMPSet.Path); err != nil {
			return err
		}
	}
	return nil
}

func (r *resolver) collectSounds(packed map[string]soundConfig) error {
	if err := forEachSorted(packed, func(name string, config soundConfig) error {
		return r.addReference(path.Join(r.packDir, packedIndexFile)+"#sounds/"+name, path.Join(r.packDir, "sounds", name), config.Path)
	}); err != nil {
		return err
	}
	return collectSourceSection(r, "sounds", packed, func(source, base string, data []byte) error {
		var config soundConfig
		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("projectassets: decode %q: %w", source, err)
		}
		return r.addReference(source, base, config.Path)
	})
}

func (r *resolver) collectFonts(packed map[string]fontFamilyConfig, hasPackedCatalog bool) error {
	if hasPackedCatalog {
		return forEachSorted(packed, func(name string, config fontFamilyConfig) error {
			return r.addFontReferences(path.Join(r.packDir, packedIndexFile)+"#fonts/"+name, path.Join(r.packDir, "fonts", name), config)
		})
	}
	return collectSourceSection(r, "fonts", map[string]fontFamilyConfig(nil), func(source, base string, data []byte) error {
		var config fontFamilyConfig
		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("projectassets: decode %q: %w", source, err)
		}
		return r.addFontReferences(source, base, config)
	})
}

func (r *resolver) addFontReferences(source, base string, config fontFamilyConfig) error {
	for _, face := range config.Faces {
		if err := r.addReference(source, base, face.Path); err != nil {
			return err
		}
	}
	return nil
}

func collectSourceSection[T any](r *resolver, section string, packed map[string]T, collect func(source, base string, data []byte) error) error {
	directory := path.Join(r.packDir, section)
	entries, ok, err := readPinnedDir(r.root, directory)
	if err != nil || !ok {
		return err
	}
	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("projectassets: config directory %q is a symlink", path.Join(directory, entry.Name()))
		}
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if err := validateConfigEntryName(section, name); err != nil {
			return err
		}
		if _, overridden := packed[name]; overridden {
			continue
		}
		base := path.Join(directory, name)
		source := path.Join(base, "index.json")
		data, found, err := readPinnedFile(r.root, source)
		if err != nil {
			return err
		}
		if !found {
			continue
		}
		if err := collect(source, base, data); err != nil {
			return err
		}
	}
	return nil
}

func validateConfigEntryName(section, name string) error {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\\x00") {
		return fmt.Errorf("projectassets: section %q has unsafe entry name %q", section, name)
	}
	return nil
}

func forEachSorted[T any](values map[string]T, visit func(string, T) error) error {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := visit(name, values[name]); err != nil {
			return err
		}
	}
	return nil
}

func (r *resolver) addReference(source, base, reference string) error {
	if reference == "" {
		return nil
	}
	if strings.ContainsAny(reference, "\\\x00") {
		return fmt.Errorf("projectassets: config %q has unsafe resource path %q", source, reference)
	}

	var name string
	switch {
	case strings.HasPrefix(reference, "res://"):
		name = strings.TrimPrefix(reference, "res://")
		if name == "" || path.IsAbs(name) {
			return fmt.Errorf("projectassets: config %q references absolute resource path %q", source, reference)
		}
		if strings.Contains(name, ":") {
			return fmt.Errorf("projectassets: config %q has unsupported resource path %q", source, reference)
		}
	case strings.Contains(reference, ":"):
		return fmt.Errorf("projectassets: config %q has unsupported resource path %q", source, reference)
	case path.IsAbs(reference):
		return fmt.Errorf("projectassets: config %q references absolute resource path %q", source, reference)
	default:
		name = path.Join(base, reference)
	}
	name = path.Clean(name)
	if name == "." || name == ".." || strings.HasPrefix(name, "../") || path.IsAbs(name) {
		return fmt.Errorf("projectassets: config %q resource %q escapes ProjectDir (outside project directory)", source, reference)
	}
	if err := pinRegularFile(r.root, name); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("projectassets: config %q resource %q is missing", source, reference)
		}
		return fmt.Errorf("projectassets: config %q resource %q: %w", source, reference, err)
	}
	if name != r.packDir && !strings.HasPrefix(name, r.packDir+"/") {
		r.referenced[name] = struct{}{}
	}
	return nil
}

func openProjectRoot(name string) (*os.Root, error) {
	before, err := os.Lstat(name)
	if err != nil {
		return nil, fmt.Errorf("projectassets: inspect ProjectDir: %w", err)
	}
	if before.Mode()&os.ModeSymlink != 0 || !before.IsDir() {
		return nil, fmt.Errorf("projectassets: ProjectDir %q is not a real directory", name)
	}
	root, err := os.OpenRoot(name)
	if err != nil {
		return nil, fmt.Errorf("projectassets: open ProjectDir: %w", err)
	}
	probe, err := root.Open(".")
	if err != nil {
		root.Close()
		return nil, fmt.Errorf("projectassets: pin ProjectDir: %w", err)
	}
	opened, statErr := probe.Stat()
	closeErr := probe.Close()
	if statErr != nil || closeErr != nil || !opened.IsDir() || !os.SameFile(before, opened) {
		root.Close()
		return nil, fmt.Errorf("projectassets: ProjectDir %q changed while opening", name)
	}
	return root, nil
}

func readPinnedFile(root *os.Root, name string) ([]byte, bool, error) {
	before, err := inspectPath(root, name, false)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, false, fmt.Errorf("projectassets: open %q: %w", name, err)
	}
	opened, statErr := file.Stat()
	if statErr != nil || !opened.Mode().IsRegular() || !os.SameFile(before[len(before)-1], opened) {
		file.Close()
		return nil, false, fmt.Errorf("projectassets: %q changed while opening", name)
	}
	data, readErr := io.ReadAll(io.LimitReader(file, maxConfigBytes+1))
	afterOpened, statErr := file.Stat()
	closeErr := file.Close()
	after, inspectErr := inspectPath(root, name, false)
	if readErr != nil {
		return nil, false, fmt.Errorf("projectassets: read %q: %w", name, readErr)
	}
	if statErr != nil {
		return nil, false, fmt.Errorf("projectassets: stat %q: %w", name, statErr)
	}
	if closeErr != nil {
		return nil, false, fmt.Errorf("projectassets: close %q: %w", name, closeErr)
	}
	if int64(len(data)) > maxConfigBytes {
		return nil, false, fmt.Errorf("projectassets: %q exceeds %d bytes", name, maxConfigBytes)
	}
	if inspectErr != nil || int64(len(data)) != opened.Size() || !samePathSnapshot(before, after) || !sameStableFile(opened, afterOpened) || !os.SameFile(opened, afterOpened) {
		return nil, false, fmt.Errorf("projectassets: %q changed while reading", name)
	}
	return data, true, nil
}

func readPinnedDir(root *os.Root, name string) ([]os.DirEntry, bool, error) {
	before, err := inspectPath(root, name, true)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	directory, err := root.Open(name)
	if err != nil {
		return nil, false, fmt.Errorf("projectassets: open directory %q: %w", name, err)
	}
	opened, statErr := directory.Stat()
	if statErr != nil || !opened.IsDir() || !os.SameFile(before[len(before)-1], opened) {
		directory.Close()
		return nil, false, fmt.Errorf("projectassets: directory %q changed while opening", name)
	}
	entries, readErr := directory.ReadDir(-1)
	afterOpened, statErr := directory.Stat()
	closeErr := directory.Close()
	after, inspectErr := inspectPath(root, name, true)
	if readErr != nil {
		return nil, false, fmt.Errorf("projectassets: read directory %q: %w", name, readErr)
	}
	if statErr != nil {
		return nil, false, fmt.Errorf("projectassets: stat directory %q: %w", name, statErr)
	}
	if closeErr != nil {
		return nil, false, fmt.Errorf("projectassets: close directory %q: %w", name, closeErr)
	}
	if inspectErr != nil || !samePathSnapshot(before, after) || !sameStableFile(opened, afterOpened) || !os.SameFile(opened, afterOpened) {
		return nil, false, fmt.Errorf("projectassets: directory %q changed while reading", name)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	return entries, true, nil
}

func pinRegularFile(root *os.Root, name string) error {
	before, err := inspectPath(root, name, false)
	if err != nil {
		return err
	}
	file, err := root.Open(name)
	if err != nil {
		return err
	}
	opened, statErr := file.Stat()
	afterOpened, secondStatErr := file.Stat()
	closeErr := file.Close()
	after, inspectErr := inspectPath(root, name, false)
	if statErr != nil {
		return statErr
	}
	if secondStatErr != nil {
		return secondStatErr
	}
	if closeErr != nil {
		return closeErr
	}
	if inspectErr != nil || !opened.Mode().IsRegular() || !os.SameFile(before[len(before)-1], opened) || !os.SameFile(opened, afterOpened) || !sameStableFile(opened, afterOpened) || !samePathSnapshot(before, after) {
		return fmt.Errorf("projectassets: %q is not a stable regular non-symlink file", name)
	}
	return nil
}

func inspectPath(root *os.Root, name string, finalDirectory bool) ([]os.FileInfo, error) {
	if name == "" || name == "." || name == ".." || path.IsAbs(name) || path.Clean(name) != name || strings.HasPrefix(name, "../") || strings.ContainsAny(name, "\\\x00") {
		return nil, fmt.Errorf("projectassets: unsafe project path %q", name)
	}
	parts := strings.Split(name, "/")
	infos := make([]os.FileInfo, 0, len(parts))
	for i := range parts {
		candidate := path.Join(parts[:i+1]...)
		info, err := root.Lstat(candidate)
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("projectassets: %q is not a regular non-symlink path (symlink at %q)", name, candidate)
		}
		last := i == len(parts)-1
		if !last || finalDirectory {
			if !info.IsDir() {
				return nil, fmt.Errorf("projectassets: %q contains a non-directory component %q", name, candidate)
			}
		} else if !info.Mode().IsRegular() {
			return nil, fmt.Errorf("projectassets: %q is not a regular non-symlink file", name)
		}
		infos = append(infos, info)
	}
	return infos, nil
}

func samePathSnapshot(before, after []os.FileInfo) bool {
	if len(before) != len(after) {
		return false
	}
	for i := range before {
		if !os.SameFile(before[i], after[i]) || !sameStableFile(before[i], after[i]) {
			return false
		}
	}
	return true
}

func sameStableFile(before, after os.FileInfo) bool {
	return before.Mode() == after.Mode() && before.Size() == after.Size() && before.ModTime() == after.ModTime()
}
