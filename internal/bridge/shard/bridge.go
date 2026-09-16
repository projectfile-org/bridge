// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package shard

import (
	"fmt"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
)

// Bridge implements core.Syncer for shard.yml.
type Bridge struct{}

// Name is the stable dispatcher id for this bridge.
func (Bridge) Name() string { return "shard" }

// Filename is the canonical on-disk name from the shards specification.
func (Bridge) Filename() string { return Filename }

// Aliases lists the alternate spelling accepted on dispatch and on disk.
func (Bridge) Aliases() []string { return []string{AliasYAML} }

// Labels renders the file and projectfile names in change reports.
func (Bridge) Labels() (string, string) { return Filename, "projectfile" }

// Policy carries no write gate: shard.yml is pure data with no detach marker.
func (Bridge) Policy() core.Policy { return core.Policy{} }

// Stacks scopes this bridge to Crystal projects for fan-out filtering.
func (Bridge) Stacks() []string { return []string{"crystal"} }

// Exists reports whether either spelling of the manifest is present.
func (Bridge) Exists(dir string) bool { return Exists(dir) }

// FullPath resolves the manifest path, keeping an existing alias spelling.
func (Bridge) FullPath(dir string, _ *projectfile.Document) string {
	return FullPath(dir)
}

// Read parses the manifest into the typed view plus the raw canvas.
func (Bridge) Read(dir string) (any, error) {
	return Read(dir)
}

// NewEmpty returns a blank manifest for the create-from-scratch path.
func (Bridge) NewEmpty() any { return &Document{} }

// Write paints the typed view onto the canvas and persists the manifest.
func (Bridge) Write(dir string, doc any) error {
	d, ok := doc.(*Document)
	if !ok {
		return fmt.Errorf("shard bridge: unexpected document type %T", doc)
	}
	return Write(dir, d)
}

// Clone deep-copies the document for dry-run planning on a throwaway copy.
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
