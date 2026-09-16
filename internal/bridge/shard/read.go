// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package shard

import (
	"fmt"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"

	"kiota.ch/projectfile/core/v2/pkg/rawdoc"
)

// Filename is the canonical on-disk name mandated by the shards specification.
const Filename = "shard.yml"

// AliasYAML is the accepted alternate spelling from the feature request.
const AliasYAML = "shard.yaml"

// FullPath resolves the shard manifest under dir, preferring the spelling
func FullPath(dir string) string {
	for _, name := range []string{Filename, AliasYAML} {
		candidate := filepath.Join(dir, name)
		if _, err := os.Stat(candidate); err == nil {
			if abs, err := filepath.Abs(candidate); err == nil {
				return filepath.Clean(abs)
			}
			return candidate
		}
	}
	abs, err := filepath.Abs(filepath.Join(dir, Filename))
	if err != nil {
		return filepath.Join(dir, Filename)
	}
	return filepath.Clean(abs)
}

// Exists reports whether either spelling of the shard manifest is present.
func Exists(dir string) bool {
	for _, name := range []string{Filename, AliasYAML} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}
	return false
}

// Read parses the shard manifest into the typed view plus the raw canvas.
func Read(dir string) (*Document, error) {
	path := FullPath(dir)
	data, err := os.ReadFile(path) // #nosec G304 -- known filename under user-provided dir
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	doc := &Document{}
	if err := yaml.Unmarshal(data, doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	rest, err := rawdoc.FromBytes(data)
	if err != nil {
		return nil, fmt.Errorf("parse %s raw view: %w", path, err)
	}
	doc.Rest = rest
	return doc, nil
}
