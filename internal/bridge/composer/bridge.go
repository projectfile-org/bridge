// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package composer

import (
	"fmt"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
)

// Bridge implements core.Syncer for composer.json.
type Bridge struct{}

func (Bridge) Name() string             { return "composer" }
func (Bridge) Filename() string         { return Filename }
func (Bridge) Aliases() []string        { return nil }
func (Bridge) Labels() (string, string) { return Filename, "projectfile" }
func (Bridge) Policy() core.Policy      { return core.Policy{} }
func (Bridge) Stacks() []string         { return []string{"php"} }

func (Bridge) Exists(dir string) bool { return Exists(dir) }

func (Bridge) FullPath(dir string, _ *projectfile.Document) string {
	return FullPath(dir)
}

func (Bridge) Read(dir string) (any, error) {
	return Read(dir)
}

func (Bridge) NewEmpty() any { return &Document{} }

func (Bridge) Write(dir string, doc any) error {
	d, ok := doc.(*Document)
	if !ok {
		return fmt.Errorf("composer bridge: unexpected document type %T", doc)
	}
	return Write(dir, d)
}

func (Bridge) Clone(doc any) any {
	d, ok := doc.(*Document)
	if !ok || d == nil {
		return doc
	}
	return d.Clone()
}

func (Bridge) BuildMappers(extDoc any, pf *projectfile.Document) core.MapperList {
	d, _ := extDoc.(*Document)
	if d == nil {
		d = &Document{}
	}
	return buildMappers(d, pf)
}
