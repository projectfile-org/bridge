// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pyproject

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"

	"kiota.ch/projectfile/core/v2/pkg/rawdoc"
)

func FullPath(dir string) string {
	abs, err := filepath.Abs(filepath.Join(dir, Filename))
	if err != nil {
		return filepath.Join(dir, Filename)
	}
	return filepath.Clean(abs)
}

func Exists(dir string) bool {
	_, err := os.Stat(FullPath(dir))
	return err == nil
}

// Read parses pyproject.toml into the typed Document AND captures the full
// top-level key order + foreign tables into Document.Rest. Both views are
// populated for every successful read — Write paints the typed [project]
// table onto the raw canvas so [build-system], [tool.*], [dependency-groups]
// survive a round-trip. The source bytes are kept too: Write splices the
// re-rendered [project] zone into them, preserving comments go-toml drops.
func Read(dir string) (*Document, error) {
	path := FullPath(dir)
	data, err := os.ReadFile(path) // #nosec G304 -- known filename under user-provided dir
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	doc := &Document{}
	if err := toml.Unmarshal(data, doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	rest, err := rawdoc.TOMLFromBytes(data)
	if err != nil {
		return nil, fmt.Errorf("parse %s raw view: %w", path, err)
	}
	doc.Rest = rest
	doc.Source = data

	return doc, nil
}
