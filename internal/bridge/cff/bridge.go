// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package cff

import (
	"fmt"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
)

// Bridge implements core.Syncer for the CITATION.cff file format.
type Bridge struct{}

func (Bridge) Name() string             { return "cff" }
func (Bridge) Filename() string         { return CFFPath }
func (Bridge) Aliases() []string        { return nil }
func (Bridge) Labels() (string, string) { return CFFPath, "projectfile" }
func (Bridge) Policy() core.Policy      { return core.Policy{} }

func (Bridge) Exists(dir string) bool { return Exists(dir) }

func (Bridge) FullPath(dir string, _ *projectfile.Document) string {
	return FullPath(dir)
}

func (Bridge) Read(dir string) (any, error) {
	return Read(dir)
}

func (Bridge) NewEmpty() any { return New() }

func (Bridge) Write(dir string, doc any) error {
	d, ok := doc.(*Document)
	if !ok {
		return fmt.Errorf("cff bridge: unexpected document type %T", doc)
	}
	return Write(dir, d)
}

func (Bridge) Clone(doc any) any {
	d, ok := doc.(*Document)
	if !ok || d == nil {
		return doc
	}
	return cloneDocument(d)
}

func (Bridge) BuildMappers(extDoc any, pf *projectfile.Document) core.MapperList {
	d, _ := extDoc.(*Document)
	return buildMappers(d, pf)
}
