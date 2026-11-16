// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package cargo

import (
	"fmt"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/v4/internal/bridge/core"
)

// Bridge implements core.Syncer for Cargo.toml.
type Bridge struct{}

// Name is the stable dispatcher id for this bridge.
func (Bridge) Name() string { return "cargo" }

// Filename is the manifest name Cargo reads.
func (Bridge) Filename() string { return Filename }

// Aliases is empty: Cargo accepts one spelling.
func (Bridge) Aliases() []string { return nil }

// Labels renders the file and projectfile names in change reports.
func (Bridge) Labels() (string, string) { return Filename, "projectfile" }

// Policy carries no write gate: the manifest is hand-edited data with no detach marker.
func (Bridge) Policy() core.Policy { return core.Policy{} }

// Stacks scopes this bridge to Rust projects for fan-out filtering.
func (Bridge) Stacks() []string { return []string{"rust"} }

// Exists reports whether the manifest is present.
func (Bridge) Exists(dir string) bool { return Exists(dir) }

// FullPath resolves the manifest path.
func (Bridge) FullPath(dir string, _ *projectfile.Document) string { return FullPath(dir) }

// Read parses the manifest.
func (Bridge) Read(dir string) (any, error) { return Read(dir) }

// NewEmpty returns a blank manifest for the create-from-scratch path.
func (Bridge) NewEmpty() any { return &Document{} }

// Write persists the manifest, editing only the lines that changed.
func (Bridge) Write(dir string, doc any) error {
	d, ok := doc.(*Document)
	if !ok {
		return fmt.Errorf("cargo bridge: unexpected document type %T", doc)
	}
	return Write(dir, d)
}

// Clone deep-copies the document for dry-run planning.
func (Bridge) Clone(doc any) any {
	d, ok := doc.(*Document)
	if !ok || d == nil {
		return doc
	}
	return d.Clone()
}

// BuildMappers closes the fieldmapper set over the live document pair.
func (Bridge) BuildMappers(extDoc any, pf *projectfile.Document) core.MapperList {
	d, _ := extDoc.(*Document)
	if d == nil {
		d = &Document{}
	}
	return buildMappers(d, pf)
}
