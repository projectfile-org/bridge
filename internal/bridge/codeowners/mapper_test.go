// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package codeowners

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/v4/internal/bridge/core"
	"projectfile.org/projectfile/bridge/v4/internal/pfmodel"
)

const (
	testPatternSrc    = "/src/"
	testPatternDeploy = "/deploy/"
	testOwnerAda      = "@ada-l"
	testOwnerGrace    = "@grace-h"
	testOwnerOps      = "@ops-team"
	testCommentOwners = " owners team"
)

// entriesMapper returns the single bidirectional mapper buildMappers wires.
func entriesMapper(t *testing.T, extDoc *Document, pf *projectfile.Document) core.FieldMapper {
	t.Helper()
	mappers := buildMappers(extDoc, pf)
	require.Len(t, mappers, 1, "the codeowners bridge maps exactly one field")
	return mappers[0]
}

// pfWithEntries paints entries back onto a bare document the way a prior sync left it.
func pfWithEntries(entries ...pfmodel.CodeOwnersEntry) *projectfile.Document {
	pf := &projectfile.Document{}
	setPFEntries(pf, entries)
	return pf
}

func TestEntriesMapperKeysMirrorTheNamespace(t *testing.T) {
	t.Parallel()

	m := entriesMapper(t, &Document{}, &projectfile.Document{})
	assert.Equal(t, "["+pfmodel.CodeOwnersExtensionNS+"].entries", m.PFKey)
	assert.True(t, strings.HasSuffix(m.PFKey, m.ExtKey), "the external key is the tail of the projectfile key")
}

func TestBuildMappersToleratesANilDocument(t *testing.T) {
	t.Parallel()

	pf := &projectfile.Document{}
	var mappers core.MapperList
	require.NotPanics(t, func() { mappers = buildMappers(nil, pf) }, "a nil external document builds a fresh one")
	assert.Len(t, mappers, 1)
}

func TestBuildMappersPaintsTheReuseHeader(t *testing.T) {
	t.Parallel()

	pf := &projectfile.Document{License: &projectfile.License{Spdx: "MIT"}}
	extDoc := &Document{}
	entriesMapper(t, extDoc, pf)
	assert.Contains(t, extDoc.ReuseHeader, "SPDX-License-Identifier: MIT", "the driver stamps the header it writes")
}

// ── to-pf direction ────────────────────────────────────────────────────────

func TestEntriesToPFFillsAnEmptyProjectfile(t *testing.T) {
	t.Parallel()

	doc := &Document{Lines: []Line{
		{Kind: KindEntry, Pattern: testPatternSrc, Owners: []string{testOwnerAda}},
		{Kind: KindEntry, Pattern: testPatternDeploy, Owners: []string{testOwnerOps}},
	}}
	pf := &projectfile.Document{}
	assert.NotEmpty(t, entriesMapper(t, doc, pf).ToPF(false))

	ext, err := pfmodel.GetCodeOwnersExtension(pf)
	require.NoError(t, err)
	require.NotNil(t, ext)
	assert.Equal(t, []pfmodel.CodeOwnersEntry{
		{Pattern: testPatternSrc, Owners: []string{testOwnerAda}},
		{Pattern: testPatternDeploy, Owners: []string{testOwnerOps}},
	}, ext.Entries, "pf carries the file's ordered entries")
}

func TestEntriesToPFHoldsAPopulatedProjectfile(t *testing.T) {
	t.Parallel()

	pf := pfWithEntries(pfmodel.CodeOwnersEntry{Pattern: testPatternSrc, Owners: []string{testOwnerAda}})
	doc := &Document{Lines: []Line{{Kind: KindEntry, Pattern: testPatternDeploy, Owners: []string{testOwnerOps}}}}
	assert.Empty(t, entriesMapper(t, doc, pf).ToPF(false), "a default sync never overwrites populated pf state")

	ext, err := pfmodel.GetCodeOwnersExtension(pf)
	require.NoError(t, err)
	require.Len(t, ext.Entries, 1)
	assert.Equal(t, testPatternSrc, ext.Entries[0].Pattern)
}

func TestEntriesToPFOverwritesOnlyWithForce(t *testing.T) {
	t.Parallel()

	pf := pfWithEntries(pfmodel.CodeOwnersEntry{Pattern: testPatternSrc, Owners: []string{testOwnerAda}})
	doc := &Document{Lines: []Line{{Kind: KindEntry, Pattern: testPatternDeploy, Owners: []string{testOwnerOps}}}}
	assert.NotEmpty(t, entriesMapper(t, doc, pf).ToPF(true))

	ext, err := pfmodel.GetCodeOwnersExtension(pf)
	require.NoError(t, err)
	require.Len(t, ext.Entries, 1)
	assert.Equal(t, testPatternDeploy, ext.Entries[0].Pattern, "force replaces the whole ordered list")
}

func TestEntriesToPFSilentWhenBothAgree(t *testing.T) {
	t.Parallel()

	pf := pfWithEntries(pfmodel.CodeOwnersEntry{Pattern: testPatternSrc, Owners: []string{testOwnerAda}})
	doc := &Document{Lines: []Line{{Kind: KindEntry, Pattern: testPatternSrc, Owners: []string{testOwnerAda}}}}
	assert.Empty(t, entriesMapper(t, doc, pf).ToPF(true), "agreeing state reports no change")
	assert.Empty(t, entriesMapper(t, doc, pf).ToPF(false))
}

func TestEntriesToPFSilentOnAnEmptyFile(t *testing.T) {
	t.Parallel()

	pf := &projectfile.Document{}
	assert.Empty(t, entriesMapper(t, &Document{}, pf).ToPF(true))
	_, present := pf.Extensions[pfmodel.CodeOwnersExtensionNS]
	assert.False(t, present, "an empty file declares no namespace at all")
}

// ── from-pf direction ──────────────────────────────────────────────────────

func TestEntriesFromPFWritesTheFileLines(t *testing.T) {
	t.Parallel()

	pf := pfWithEntries(
		pfmodel.CodeOwnersEntry{Pattern: testPatternSrc, Owners: []string{testOwnerAda}},
		pfmodel.CodeOwnersEntry{Pattern: testPatternDeploy, Owners: []string{testOwnerOps}},
	)
	doc := &Document{Lines: []Line{{Kind: KindComment, Text: testCommentOwners}}}
	assert.NotEmpty(t, entriesMapper(t, doc, pf).FromPF(true))
	require.Len(t, doc.Lines, 2, "the file body is replaced wholesale, comments included")
	assert.Equal(t, testPatternSrc, doc.Lines[0].Pattern)
	assert.Equal(t, []string{testOwnerAda}, doc.Lines[0].Owners, "owners are copied, never aliased")
	assert.Equal(t, testPatternDeploy, doc.Lines[1].Pattern)
	assert.Equal(t, []string{testOwnerOps}, doc.Lines[1].Owners)
}

func TestEntriesFromPFDerivesDefaultOwners(t *testing.T) {
	t.Parallel()

	pf := &projectfile.Document{People: []projectfile.Person{
		{Alias: "ada-l", Roles: []string{ownerTestRole}},
	}}
	doc := &Document{}
	assert.NotEmpty(t, entriesMapper(t, doc, pf).FromPF(true))
	require.Len(t, doc.Lines, 1, "no declared entries falls back to the catch-all rule")
	assert.Equal(t, KindEntry, doc.Lines[0].Kind)
	assert.Equal(t, []string{testOwnerAda}, doc.Lines[0].Owners)
}

func TestEntriesFromPFSilentWhenNothingToWrite(t *testing.T) {
	t.Parallel()

	pf := &projectfile.Document{}
	doc := &Document{}
	assert.Empty(t, entriesMapper(t, doc, pf).FromPF(true), "no entries and no people leave the file empty")
	assert.Empty(t, doc.Lines)
}

func TestEntriesFromPFHoldsAnExistingFileWithoutForce(t *testing.T) {
	t.Parallel()

	pf := &projectfile.Document{}
	doc := &Document{Lines: []Line{{Kind: KindEntry, Pattern: testPatternDeploy, Owners: []string{testOwnerOps}}}}
	assert.Empty(t, entriesMapper(t, doc, pf).FromPF(false), "pf has no entries, so a default sync leaves the file alone")
	require.Len(t, doc.Lines, 1)
}

func TestEntriesFromPFClearsTheFileOnlyWithForce(t *testing.T) {
	t.Parallel()

	pf := &projectfile.Document{}
	doc := &Document{Lines: []Line{{Kind: KindEntry, Pattern: testPatternDeploy, Owners: []string{testOwnerOps}}}}
	assert.NotEmpty(t, entriesMapper(t, doc, pf).FromPF(true), "force clears entries the projectfile no longer declares")
	assert.Empty(t, doc.Lines)
}

func TestEntriesFromPFSilentWhenBothAgree(t *testing.T) {
	t.Parallel()

	pf := pfWithEntries(pfmodel.CodeOwnersEntry{Pattern: testPatternSrc, Owners: []string{testOwnerAda}})
	doc := &Document{Lines: []Line{
		{Kind: KindComment, Text: testCommentOwners},
		{Kind: KindEntry, Pattern: testPatternSrc, Owners: []string{testOwnerAda}},
	}}
	assert.Empty(t, entriesMapper(t, doc, pf).FromPF(true), "agreeing state keeps the file byte-stable, comments included")
	require.Len(t, doc.Lines, 2)
	assert.Equal(t, KindComment, doc.Lines[0].Kind)
}

func TestEntriesFromPFRewritesOnReorderedEntries(t *testing.T) {
	t.Parallel()

	pf := pfWithEntries(
		pfmodel.CodeOwnersEntry{Pattern: testPatternDeploy, Owners: []string{testOwnerOps}},
		pfmodel.CodeOwnersEntry{Pattern: testPatternSrc, Owners: []string{testOwnerAda}},
	)
	doc := &Document{Lines: []Line{
		{Kind: KindEntry, Pattern: testPatternSrc, Owners: []string{testOwnerAda}},
		{Kind: KindEntry, Pattern: testPatternDeploy, Owners: []string{testOwnerOps}},
	}}
	assert.NotEmpty(t, entriesMapper(t, doc, pf).FromPF(false), "order is semantic — the last match wins")
	require.Len(t, doc.Lines, 2)
	assert.Equal(t, testPatternDeploy, doc.Lines[0].Pattern, "the file takes pf's declared order")
}

// ── entry-set helpers ──────────────────────────────────────────────────────

func TestDocEntriesSkipsCommentsAndBlanks(t *testing.T) {
	t.Parallel()

	got := docEntries(&Document{Lines: []Line{
		{Kind: KindComment, Text: testCommentOwners},
		{Kind: KindBlank},
		{Kind: KindEntry, Pattern: testPatternSrc, Owners: []string{testOwnerAda}},
	}})
	assert.Equal(t, []pfmodel.CodeOwnersEntry{{Pattern: testPatternSrc, Owners: []string{testOwnerAda}}}, got)
	assert.Nil(t, docEntries(nil), "a nil document carries no entries")
}

func TestPFEntriesReadsTheDeclaredNamespace(t *testing.T) {
	t.Parallel()

	flat := pfWithEntries(pfmodel.CodeOwnersEntry{Pattern: testPatternSrc, Owners: []string{testOwnerAda, testOwnerGrace}})
	assert.Equal(t, []pfmodel.CodeOwnersEntry{{Pattern: testPatternSrc, Owners: []string{testOwnerAda, testOwnerGrace}}}, pfEntries(flat))
	assert.Nil(t, pfEntries(&projectfile.Document{}), "no namespace means no entries")
}

func TestSetPFEntriesPrunesTheNestedTableForm(t *testing.T) {
	t.Parallel()

	pf := &projectfile.Document{Extensions: map[string]any{
		"org": map[string]any{"projectfile": map[string]any{
			"codeowners": map[string]any{"entries": []any{map[string]any{"pattern": "stale", "owners": []any{"@stale"}}}},
		}},
	}}
	setPFEntries(pf, []pfmodel.CodeOwnersEntry{{Pattern: testPatternSrc, Owners: []string{testOwnerAda}}})
	_, nested := pf.Extensions["org"]
	assert.False(t, nested, "the dotted header is pruned so the section is not doubled")
	require.Contains(t, pf.Extensions, pfmodel.CodeOwnersExtensionNS)

	ext, err := pfmodel.GetCodeOwnersExtension(pf)
	require.NoError(t, err)
	require.Len(t, ext.Entries, 1)
	assert.Equal(t, testPatternSrc, ext.Entries[0].Pattern)
}

func TestSetPFEntriesAcceptsAnEmptyList(t *testing.T) {
	t.Parallel()

	pf := &projectfile.Document{}
	setPFEntries(pf, nil)
	ext, err := pfmodel.GetCodeOwnersExtension(pf)
	require.NoError(t, err)
	require.NotNil(t, ext)
	assert.Empty(t, ext.Entries)
}

// A person with neither alias nor email resolves no token, and a repeated
// handle is kept once — CODEOWNERS would reject a duplicated owner.
func TestDefaultOwnerEntriesSkipsEmptyAndRepeatedTokens(t *testing.T) {
	t.Parallel()

	pf := &projectfile.Document{People: []projectfile.Person{
		{FamilyNames: "Nameless"},
		{Alias: "dup", Roles: []string{ownerTestRole}},
		{Alias: "dup", Roles: []string{ownerTestRole}},
	}}
	entries := defaultOwnerEntries(pf)
	require.Len(t, entries, 1)
	assert.Equal(t, []string{"@dup"}, entries[0].Owners)
}

func TestDefaultOwnerEntriesFallbackIncludesOrganizations(t *testing.T) {
	t.Parallel()

	pf := &projectfile.Document{
		People:        []projectfile.Person{{Alias: "solo"}},
		Organizations: []projectfile.Organization{{Name: "Org", Alias: "team"}},
	}
	entries := defaultOwnerEntries(pf)
	require.Len(t, entries, 1)
	assert.Equal(t, []string{"@solo", "@team"}, entries[0].Owners, "the fallback takes both people and organisations")
}

func TestEntriesEqualCountsOrderAndOwners(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		a, b  []pfmodel.CodeOwnersEntry
		equal bool
	}{
		{name: "both empty", equal: true},
		{name: "one empty", a: nil, b: []pfmodel.CodeOwnersEntry{{Pattern: testPatternSrc}}, equal: false},
		{name: "different pattern", a: []pfmodel.CodeOwnersEntry{{Pattern: testPatternSrc}}, b: []pfmodel.CodeOwnersEntry{{Pattern: testPatternDeploy}}, equal: false},
		{name: "extra owner", a: []pfmodel.CodeOwnersEntry{{Pattern: testPatternSrc, Owners: []string{testOwnerAda}}}, b: []pfmodel.CodeOwnersEntry{{Pattern: testPatternSrc, Owners: []string{testOwnerAda, testOwnerOps}}}, equal: false},
		{name: "other owner", a: []pfmodel.CodeOwnersEntry{{Pattern: testPatternSrc, Owners: []string{testOwnerAda}}}, b: []pfmodel.CodeOwnersEntry{{Pattern: testPatternSrc, Owners: []string{testOwnerOps}}}, equal: false},
		{name: "reordered", a: []pfmodel.CodeOwnersEntry{{Pattern: testPatternSrc}, {Pattern: testPatternDeploy}}, b: []pfmodel.CodeOwnersEntry{{Pattern: testPatternDeploy}, {Pattern: testPatternSrc}}, equal: false},
		{name: "identical", a: []pfmodel.CodeOwnersEntry{{Pattern: testPatternSrc, Owners: []string{testOwnerAda}}}, b: []pfmodel.CodeOwnersEntry{{Pattern: testPatternSrc, Owners: []string{testOwnerAda}}}, equal: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.equal, entriesEqual(tc.a, tc.b))
		})
	}
}

func TestSummariseEntriesListsThePatterns(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "1 entr(y/ies): "+testPatternSrc+" → "+testOwnerAda,
		summariseEntries([]pfmodel.CodeOwnersEntry{{Pattern: testPatternSrc, Owners: []string{testOwnerAda}}}))
	assert.Equal(t, "0 entr(y/ies): ", summariseEntries(nil))
}
