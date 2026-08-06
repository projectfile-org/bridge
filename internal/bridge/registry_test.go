// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package bridge_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
)

// ── Stub Bridge types ────────────────────────────────────────────────────────

// testBridgeSyncer is a minimal Syncer used only in this test package.
// Names use "zz-test-" prefix to avoid clashing with production bridge filenames.
type testBridgeSyncer struct {
	name, filename string
	aliases        []string
}

func (s testBridgeSyncer) Name() string             { return s.name }
func (s testBridgeSyncer) Filename() string         { return s.filename }
func (s testBridgeSyncer) Aliases() []string        { return s.aliases }
func (s testBridgeSyncer) Labels() (string, string) { return s.filename, "projectfile" }
func (s testBridgeSyncer) Policy() core.Policy      { return core.Policy{} }
func (s testBridgeSyncer) Exists(_ string) bool     { return false }
func (s testBridgeSyncer) FullPath(dir string, _ *projectfile.Document) string {
	return dir + "/" + s.filename
}
func (s testBridgeSyncer) NewEmpty() any               { return nil }
func (s testBridgeSyncer) Clone(doc any) any           { return doc }
func (s testBridgeSyncer) Read(_ string) (any, error)  { return nil, nil }
func (s testBridgeSyncer) Write(_ string, _ any) error { return nil }
func (s testBridgeSyncer) BuildMappers(_ any, _ *projectfile.Document) core.MapperList {
	return nil
}

type testBridgeRenderer struct{ name, filename string }

func (r testBridgeRenderer) Name() string             { return r.name }
func (r testBridgeRenderer) Filename() string         { return r.filename }
func (r testBridgeRenderer) Aliases() []string        { return nil }
func (r testBridgeRenderer) Labels() (string, string) { return r.filename, "projectfile" }
func (r testBridgeRenderer) Policy() core.Policy      { return core.Policy{} }
func (r testBridgeRenderer) Exists(_ string) bool     { return false }
func (r testBridgeRenderer) FullPath(dir string, _ *projectfile.Document) string {
	return dir + "/" + r.filename
}

func (r testBridgeRenderer) Render(_ *projectfile.Document, _ core.Options) (core.Output, error) {
	return core.Output{Files: map[string][]byte{r.filename: []byte("content")}}, nil
}

// ── Register stubs once at init time to avoid duplicate-registration panics ──

func init() {
	bridge.Register(testBridgeSyncer{
		name:     "zz-test-syncer",
		filename: "zz-test-syncer.stub",
	})
	bridge.Register(testBridgeRenderer{
		name:     "zz-test-renderer",
		filename: "zz-test-renderer.stub",
	})
	bridge.Register(testBridgeSyncer{
		name:     "zz-test-aliased",
		filename: "zz-test-aliased.stub",
		aliases:  []string{"zz-test-alias.stub"},
	})
}

// ── Tests ────────────────────────────────────────────────────────────────────

func TestLookupByFilename(t *testing.T) {
	b, ok := bridge.Lookup("zz-test-syncer.stub")
	require.True(t, ok)
	assert.Equal(t, "zz-test-syncer.stub", b.Filename())
}

func TestLookupByAlias(t *testing.T) {
	b, ok := bridge.Lookup("zz-test-alias.stub")
	require.True(t, ok)
	assert.Equal(t, "zz-test-aliased.stub", b.Filename(), "alias lookup must return canonical bridge")
}

func TestLookupUnknown(t *testing.T) {
	_, ok := bridge.Lookup("definitely-not-registered.xyz")
	assert.False(t, ok)
}

func TestListContainsRegisteredBridges(t *testing.T) {
	all := bridge.List()
	var filenames []string
	for _, b := range all {
		filenames = append(filenames, b.Filename())
	}
	assert.Contains(t, filenames, "zz-test-syncer.stub")
	assert.Contains(t, filenames, "zz-test-renderer.stub")
	assert.Contains(t, filenames, "zz-test-aliased.stub")
}

func TestListIsSortedByFilename(t *testing.T) {
	all := bridge.List()
	for i := 1; i < len(all); i++ {
		assert.LessOrEqual(t, all[i-1].Filename(), all[i].Filename(), "List must be sorted by Filename")
	}
}

func TestFilterSyncers(t *testing.T) {
	syncers := bridge.Filter(bridge.IsSyncer)
	for _, b := range syncers {
		assert.True(t, bridge.IsSyncer(b), "Filter(IsSyncer) must return only Syncer bridges")
	}
	var filenames []string
	for _, b := range syncers {
		filenames = append(filenames, b.Filename())
	}
	assert.Contains(t, filenames, "zz-test-syncer.stub")
	assert.Contains(t, filenames, "zz-test-aliased.stub")
	assert.NotContains(t, filenames, "zz-test-renderer.stub")
}

func TestFilterRenderers(t *testing.T) {
	renderers := bridge.Filter(bridge.IsRenderer)
	for _, b := range renderers {
		assert.True(t, bridge.IsRenderer(b), "Filter(IsRenderer) must return only Renderer bridges")
	}
	var filenames []string
	for _, b := range renderers {
		filenames = append(filenames, b.Filename())
	}
	assert.Contains(t, filenames, "zz-test-renderer.stub")
	assert.NotContains(t, filenames, "zz-test-syncer.stub")
}
