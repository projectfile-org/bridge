// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package cargo

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
	"kiota.ch/projectfile/core/v2/pkg/genlog"
)

// FullPath resolves the manifest path under dir.
func FullPath(dir string) string {
	abs, err := filepath.Abs(filepath.Join(dir, Filename))
	if err != nil {
		return filepath.Join(dir, Filename)
	}
	return filepath.Clean(abs)
}

// Exists reports whether dir holds a manifest.
func Exists(dir string) bool {
	_, err := os.Stat(FullPath(dir))
	return err == nil
}

// Read parses the manifest into the typed view, each field read from the table defining it.
func Read(dir string) (*Document, error) {
	path := FullPath(dir)
	data, err := os.ReadFile(path) // #nosec G304 -- known filename under user-provided dir
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	l, err := parseLayout(data)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	doc := &Document{Source: data, Foreign: map[string]bool{}}
	for _, key := range ownedKeys {
		table := l.table(key)
		if table == "" {
			genlog.Debug("Cargo.toml key defined outside this manifest", "key", key)
			doc.Foreign[key] = true
			continue
		}
		v := l.get(table, key)
		if p := doc.stringField(key); p != nil {
			*p, _ = v.(string)
		} else if p := doc.listField(key); p != nil {
			*p = stringsOf(v)
		}
	}
	return doc, nil
}

// layout is the parsed shape of a manifest: which package tables it declares.
type layout struct {
	// pkg is the [package] table, nil when the manifest declares none.
	pkg map[string]any
	// wsPkg is the [workspace.package] table, nil when the manifest declares none.
	wsPkg map[string]any
	// workspace reports a [workspace] table, so this manifest is a workspace root.
	workspace bool
}

// parseLayout decodes the package and workspace tables of src.
func parseLayout(src []byte) (layout, error) {
	var raw struct {
		Package   map[string]any `toml:"package"`
		Workspace *struct {
			Package map[string]any `toml:"package"`
		} `toml:"workspace"`
	}
	if err := toml.Unmarshal(src, &raw); err != nil {
		return layout{}, err
	}
	l := layout{pkg: raw.Package, workspace: raw.Workspace != nil}
	if raw.Workspace != nil {
		l.wsPkg = raw.Workspace.Package
	}
	return l, nil
}

// table names the table defining key in this manifest, "" when a workspace elsewhere does.
func (l layout) table(key string) string {
	switch {
	case l.pkg != nil && !inherited(l.pkg[key]):
		return tablePackage
	case l.pkg != nil && !l.workspace:
		return ""
	case l.pkg == nil && l.workspace && key == keyName:
		return ""
	case l.pkg == nil && !l.workspace:
		return tablePackage
	}
	return tableWorkspacePackage
}

// get reads key from the named table.
func (l layout) get(table, key string) any {
	if table == tablePackage {
		return l.pkg[key]
	}
	return l.wsPkg[key]
}

// inherited reports whether a [package] value is `{ workspace = true }`.
func inherited(v any) bool {
	m, ok := v.(map[string]any)
	return ok && m["workspace"] == true
}

// stringsOf converts a decoded TOML array to strings, skipping non-string items.
func stringsOf(v any) []string {
	items, _ := v.([]any)
	var out []string
	for _, item := range items {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
