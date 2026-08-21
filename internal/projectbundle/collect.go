/*
 * Copyright (c) 2021 The XGo Authors (xgo.dev). All rights reserved.
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

package projectbundle

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type collector struct {
	limits    resolvedLimits
	entries   []collectedEntry
	total     int64
	exact     map[string]string
	canonical map[string]string
	folded    map[string]string
}

func newCollector(limits resolvedLimits) *collector {
	return &collector{
		limits:    limits,
		exact:     make(map[string]string),
		canonical: make(map[string]string),
		folded:    make(map[string]string),
	}
}

func (c *collector) addFile(root *safeDir, sourcePath, archiveName string) error {
	if err := c.reserveEntry(archiveName); err != nil {
		return err
	}

	file, err := root.OpenFile(filepath.FromSlash(sourcePath))
	if err != nil {
		return fmt.Errorf("projectbundle: open source %q: %w", sourcePath, err)
	}
	defer file.Close()

	data, err := readRegularFile(file, sourcePath, c.limits.maxFileBytes, c.limits.maxTotalBytes-c.total)
	if err != nil {
		return err
	}
	return c.appendEntry(archiveName, data, false)
}

func (c *collector) addData(archiveName string, data []byte) error {
	if err := c.reserveEntry(archiveName); err != nil {
		return err
	}
	return c.appendEntry(archiveName, data, true)
}

func (c *collector) reserveEntry(name string) error {
	name, err := validateRelativePath(name, "archive entry")
	if err != nil {
		return err
	}
	if err := c.reserveName(name); err != nil {
		return err
	}
	if len(c.entries) >= c.limits.maxEntries {
		return fmt.Errorf("%w: more than %d entries", ErrLimit, c.limits.maxEntries)
	}
	return nil
}

func (c *collector) appendEntry(name string, data []byte, clone bool) error {
	size := int64(len(data))
	if size > c.limits.maxFileBytes {
		return fmt.Errorf("%w: source %q exceeds %d bytes", ErrLimit, name, c.limits.maxFileBytes)
	}
	if size > c.limits.maxTotalBytes-c.total {
		return fmt.Errorf("%w: total input exceeds %d bytes", ErrLimit, c.limits.maxTotalBytes)
	}
	contents := data
	if clone {
		contents = append([]byte(nil), data...)
		if data != nil && contents == nil {
			contents = []byte{}
		}
	}
	c.total += size
	c.entries = append(c.entries, collectedEntry{name: name, data: contents})
	return nil
}

func (c *collector) addTree(root *safeDir, prefix string) error {
	entries, err := root.ReadDir()
	if err != nil {
		return fmt.Errorf("projectbundle: read PackDir %q: %w", prefix, err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		name := prefix + "/" + entry.Name()
		if _, err := validateRelativePath(name, "PackDir entry"); err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("%w: PackDir entry %q is a symbolic link", ErrUnsafeFile, name)
		}
		if entry.IsDir() {
			child, err := root.OpenDir(entry.Name())
			if err != nil {
				return fmt.Errorf("projectbundle: open PackDir directory %q: %w", name, err)
			}
			err = c.addTree(child, name)
			closeErr := child.Close()
			if err != nil {
				return err
			}
			if closeErr != nil {
				return fmt.Errorf("projectbundle: close PackDir directory %q: %w", name, closeErr)
			}
			continue
		}
		if err := c.addFile(root, entry.Name(), name); err != nil {
			return err
		}
	}
	return nil
}
