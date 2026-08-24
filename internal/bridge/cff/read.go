// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package cff

import (
	"fmt"
	"os"
	"path/filepath"

	"go.yaml.in/yaml/v3"

	"kiota.ch/projectfile/core/v2/pkg/rawdoc"
)

const DefaultCFFVersion = "1.2.0"

const DefaultMessage = "Please cite this software using the metadata from this file."

const DefaultType = "software"

var CFFPath = "CITATION.cff"

func FullPath(dir string) string {
	abs, err := filepath.Abs(filepath.Join(dir, CFFPath))
	if err != nil {
		return filepath.Join(dir, CFFPath)
	}
	return filepath.Clean(abs)
}

func Exists(dir string) bool {
	_, err := os.Stat(FullPath(dir))
	return err == nil
}

// Read parses CITATION.cff into the typed Document AND captures the full
// node tree (with comments, key order, unknown keys) into Document.Rest. Both
// views are populated for every successful read so Write can paint the typed
// fields back onto the raw canvas non-destructively.
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

// New returns a fresh CITATION.cff document with the spec defaults applied.
func New() *Document {
	return &Document{
		CFFVersion: DefaultCFFVersion,
		Message:    DefaultMessage,
		Type:       DefaultType,
	}
}
