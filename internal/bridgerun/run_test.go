// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package bridgerun

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
)

// Fakes for the registry-driven describeLine: Bridge identity plus the one
// capability that decides the direction suffix. Names sort a/b/c so the
// expected line reads in registration-independent order.
type fakeRenderer struct{ filename string }

func (f fakeRenderer) Name() string      { return f.filename }
func (f fakeRenderer) Filename() string  { return f.filename }
func (f fakeRenderer) Aliases() []string { return nil }
func (f fakeRenderer) Labels() (string, string) {
	return f.filename, "projectfile"
}

func (f fakeRenderer) FullPath(dir string, _ *projectfile.Document) string {
	return dir + "/" + f.filename
}
func (f fakeRenderer) Exists(string) bool  { return false }
func (f fakeRenderer) Policy() core.Policy { return core.Policy{} }
func (f fakeRenderer) Render(*projectfile.Document, core.Options) (core.Output, error) {
	return core.Output{}, nil
}

type fakeSyncer struct{ fakeRenderer }

func (f fakeSyncer) NewEmpty() any            { return nil }
func (f fakeSyncer) Clone(doc any) any        { return doc }
func (f fakeSyncer) Read(string) (any, error) { return nil, nil }
func (f fakeSyncer) Write(string, any) error  { return nil }
func (f fakeSyncer) BuildMappers(any, *projectfile.Document) core.MapperList {
	return nil
}

type fakeDescriber struct{ fakeRenderer }

func (f fakeDescriber) Describe() string { return "custom line (one-way render)" }

func TestDescribeLine(t *testing.T) {
	bridge.Register(fakeDescriber{fakeRenderer{"a-fragments"}})
	bridge.Register(fakeRenderer{"b-readme.md"})
	bridge.Register(fakeRenderer{"b2-dockerignore"})

	// Files of one binary share one direction, so the suffix is stated once.
	assert.Equal(t,
		"custom line (one-way render), "+
			"b-readme.md, b2-dockerignore — one-way render",
		describeLine())

	// A syncer joining the set switches the whole binary to mixed direction.
	bridge.Register(fakeSyncer{fakeRenderer{"c-package.json"}})
	assert.Equal(t,
		"custom line (one-way render), "+
			"b-readme.md, b2-dockerignore, c-package.json — sync and render",
		describeLine())
}
