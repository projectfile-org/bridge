// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package cff

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"kiota.ch/projectfile/core/v2/pkg/rawdoc"
	"projectfile.org/projectfile/bridge/v4/internal/bridge/core"
)

// mutatedValue is the sentinel written into a clone to prove it does not alias the original.
const mutatedValue = "mutated"

const validCFFBody = `cff-version: 1.2.0
message: Please cite this software using the metadata from this file.
title: Bridge Tests
abstract: A short abstract.
authors:
  - family-names: Búho
    given-names: Damián
license: MIT
repository-code: https://github.com/projectfile-org/bridge
`

// TestBridgeSurface pins the Syncer interface every bridge implements, so a rename or a signature drift fails here rather than at registration.
func TestBridgeSurface(t *testing.T) {
	b := Bridge{}
	assert.Equal(t, "cff", b.Name())
	assert.Equal(t, CFFPath, b.Filename())
	assert.Empty(t, b.Aliases())
	label, provenance := b.Labels()
	assert.Equal(t, CFFPath, label)
	assert.Equal(t, "projectfile", provenance)
	assert.Equal(t, core.Policy{}, b.Policy())

	dir := t.TempDir()
	assert.False(t, b.Exists(dir))
	require.NoError(t, os.WriteFile(filepath.Join(dir, CFFPath), []byte(validCFFBody), 0o644))
	assert.True(t, b.Exists(dir))

	// FullPath resolves absolute against dir regardless of the document.
	assert.Equal(t, FullPath(dir), b.FullPath(dir, nil))

	// Read returns the typed document with its raw canvas attached.
	doc, err := b.Read(dir)
	require.NoError(t, err)
	typed, ok := doc.(*Document)
	require.True(t, ok)
	assert.Equal(t, "Bridge Tests", typed.Title)
	require.NotNil(t, typed.Rest)

	// NewEmpty carries the spec defaults.
	empty, ok := b.NewEmpty().(*Document)
	require.True(t, ok)
	assert.Equal(t, DefaultCFFVersion, empty.CFFVersion)
	assert.Equal(t, DefaultMessage, empty.Message)
	assert.Equal(t, DefaultType, empty.Type)
}

// TestBridgeWriteRejectsForeignDocument pins the type guard: a document that is not the cff one must fail loudly instead of being silently marshalled.
func TestBridgeWriteRejectsForeignDocument(t *testing.T) {
	err := Bridge{}.Write(t.TempDir(), "not a cff document")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected document type")

	got := Bridge{}.Clone("not a cff document")
	assert.Equal(t, "not a cff document", got)
}

// TestBridgeWriteAndCloneRoundTrip pins the Syncer write path and that Clone returns an independent deep copy rather than a shared reference.
func TestBridgeWriteAndCloneRoundTrip(t *testing.T) {
	dir := t.TempDir()
	_, err := Bridge{}.Read(dir)
	require.Error(t, err, "no CITATION.cff yet")

	doc := minimalValidDoc()
	doc.Keywords = []string{"go"}
	doc.Identifiers = []Identifier{{Type: cffKeyDOI, Value: testDOIValue}}
	doc.References = []Reference{{Type: "book", Title: "A Reference", Authors: []PersonEntity{{FamilyNames: testFamilyName}}}}
	doc.PreferredCitation = &Reference{Type: "article", Title: testPaperTitle, Authors: []PersonEntity{{FamilyNames: testFamilyName}}}
	doc.Contributors = nil

	require.NoError(t, Bridge{}.Write(dir, doc))

	reread, err := Bridge{}.Read(dir)
	require.NoError(t, err)
	typed, ok := reread.(*Document)
	require.True(t, ok)
	assert.Equal(t, []string{"go"}, typed.Keywords)

	// Clone must not alias the original's slices or raw canvas.
	clone, ok := Bridge{}.Clone(typed).(*Document)
	require.True(t, ok)
	assert.NotSame(t, typed.Rest, clone.Rest)
	clone.Keywords[0] = mutatedValue
	clone.Identifiers[0].Value = mutatedValue
	clone.PreferredCitation.Title = mutatedValue
	assert.Equal(t, "go", typed.Keywords[0])
	assert.Equal(t, testDOIValue, typed.Identifiers[0].Value)
	assert.Equal(t, testPaperTitle, typed.PreferredCitation.Title)

	// A fresh document clones without a canvas and stays independent.
	fresh := Bridge{}.Clone(New()).(*Document)
	assert.Nil(t, fresh.Rest)
	assert.Equal(t, DefaultCFFVersion, fresh.CFFVersion)
}

// TestBridgeBuildMappersForeignDocument pins the type guard on the mapper entry point.
func TestBridgeBuildMappersForeignDocument(t *testing.T) {
	mappers := Bridge{}.BuildMappers("not a cff document", &projectfile.Document{})
	assert.Len(t, mappers, 12)
}

// TestReadErrorPaths pins the two failure shapes Read reports.
func TestReadErrorPaths(t *testing.T) {
	dir := t.TempDir()
	_, err := Read(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read "+FullPath(dir))

	broken := filepath.Join(dir, CFFPath)
	require.NoError(t, os.WriteFile(broken, []byte("title: [unclosed\n"), 0o644))
	_, err = Read(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse ")
}

// TestPersonEntityDiscriminators pins the person/entity rule the shape-aware mapper copy depends on: family-names decides, a bare name is an entity.
func TestPersonEntityDiscriminators(t *testing.T) {
	person := &PersonEntity{FamilyNames: testFamilyName, GivenNames: testGivenName}
	entity := &PersonEntity{Name: testOrgName}
	neither := &PersonEntity{Email: testAuthorEmail}

	assert.True(t, person.IsPerson())
	assert.False(t, person.IsEntity())
	assert.True(t, entity.IsEntity())
	assert.False(t, entity.IsPerson())
	assert.False(t, neither.IsPerson())
	assert.False(t, neither.IsEntity())
}

// TestWriteRejectsSchemaInvalidDocument pins the runtime gate: Write refuses a document the embedded CFF 1.2.0 schema rejects, and names the schema URL.
func TestWriteRejectsSchemaInvalidDocument(t *testing.T) {
	dir := t.TempDir()
	doc := minimalValidDoc()
	doc.Title = "Mixed Shape"
	doc.Abstract = "Both name shapes on one record."
	doc.Authors = []PersonEntity{{FamilyNames: "Búho", Name: "Wrongly-Set"}}

	err := Write(dir, doc)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "schema validation failed")
	assert.Contains(t, err.Error(), cffSchemaURL)

	_, statErr := os.Stat(filepath.Join(dir, CFFPath))
	assert.True(t, os.IsNotExist(statErr), "a rejected write must leave no file")
}

// TestWriteMergeIsWrite pins the retained legacy alias behaves identically.
func TestWriteMergeIsWrite(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, WriteMerge(dir, minimalValidDoc()))
	assert.True(t, Exists(dir))

	require.NoError(t, WriteMerge(dir, minimalValidDoc()))
	got, err := Read(dir)
	require.NoError(t, err)
	assert.Equal(t, minimalValidDoc().Title, got.Title)
}

// TestValidateRejectsUnparsableYAML pins the pre-schema parse failure path.
func TestValidateRejectsUnparsableYAML(t *testing.T) {
	err := Validate([]byte("title: [unclosed\n"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse CFF for validation")
}

// TestValidateAcceptsConformingDocument pins the schema's happy path through the full YAML→JSON normalisation.
func TestValidateAcceptsConformingDocument(t *testing.T) {
	require.NoError(t, Validate([]byte(validCFFBody)))
}

// TestValidateEcma262RegexpEngine pins the regexp engine Validate installs: the compiler-facing contract is MatchString plus the pattern source.
func TestValidateEcma262RegexpEngine(t *testing.T) {
	re, err := cffEcma262Regexp(`^\d{4}-\d{2}-\d{2}$`)
	require.NoError(t, err)
	assert.Equal(t, `^\d{4}-\d{2}-\d{2}$`, re.String())
	assert.True(t, re.MatchString("2026-01-15"))
	assert.False(t, re.MatchString("not-a-date"))

	// An ECMA-invalid pattern is reported rather than silently dropped.
	_, err = cffEcma262Regexp("[unclosed")
	require.Error(t, err)
}

// TestLicenseToStringUnknownShape pins that an unrecognised licence shape normalises to the empty SPDX expression rather than a partial read.
func TestLicenseToStringUnknownShape(t *testing.T) {
	assert.Equal(t, "", LicenseToString(42))
	assert.Equal(t, "", LicenseToString(map[string]any{"spdx": "MIT"}))
	// A mixed slice keeps only its string elements.
	assert.Equal(t, testMIT, LicenseToString([]any{42, testMIT}))
	assert.Equal(t, testMITOrApache, LicenseToString([]any{testMIT, "", testApache}))
	assert.Equal(t, "", LicenseToString([]any{42, true}))
}

// TestCitExtMapIgnoresMalformedNamespace pins that a citation extension of the wrong shape reads as absent instead of failing the sync.
func TestCitExtMapIgnoresMalformedNamespace(t *testing.T) {
	pf := &projectfile.Document{Extensions: map[string]any{"org.projectfile.citation": "not-a-map"}}
	assert.Empty(t, citExtString(pf, cffKeyDOI))
	assert.False(t, hasCitExtKey(pf, cffKeyDOI))

	setCitExtString(pf, cffKeyDOI, testDOIValue)
	assert.Equal(t, testDOIValue, citExtString(pf, cffKeyDOI))
	assert.True(t, hasCitExtKey(pf, cffKeyDOI))
}

// TestCloneDocumentDeepCopies pins that every slice, the preferred citation and the raw canvas are independent of the original.
func TestCloneDocumentDeepCopies(t *testing.T) {
	original := &Document{
		Title:             testTitle,
		Authors:           []PersonEntity{{FamilyNames: testFamilyName}},
		Contributors:      nil,
		Maintainers:       nil,
		Contact:           []PersonEntity{{Name: testOrgName}},
		Keywords:          []string{"go"},
		Identifiers:       []Identifier{{Type: cffKeyDOI, Value: testDOIValue}},
		References:        []Reference{{Title: "A Reference"}},
		PreferredCitation: &Reference{Title: testPaperTitle},
		Rest:              rawdoc.NewYAMLNode(),
	}

	cp := cloneDocument(original)
	require.NotNil(t, cp)
	assert.NotSame(t, original.PreferredCitation, cp.PreferredCitation)
	assert.NotSame(t, original.Rest, cp.Rest)
	assert.Nil(t, cp.Contributors, "a nil slice clones to nil, not to an empty slice")

	cp.Authors[0].FamilyNames = mutatedValue
	cp.Keywords[0] = mutatedValue
	cp.PreferredCitation.Title = mutatedValue
	assert.Equal(t, testFamilyName, original.Authors[0].FamilyNames)
	assert.Equal(t, "go", original.Keywords[0])
	assert.Equal(t, testPaperTitle, original.PreferredCitation.Title)
}

// TestFullPathIsAbsolute pins that FullPath resolves to an absolute, cleaned path under dir.
func TestFullPathIsAbsolute(t *testing.T) {
	got := FullPath("relative/dir")
	assert.True(t, filepath.IsAbs(got))
	assert.Equal(t, filepath.Clean(got), got)
	assert.Contains(t, got, "relative/dir")
}

// TestStripLeadingCommentsAndHeader pins the head-comment composition the writer relies on, including the REUSE-separator bare `#`.
func TestStripLeadingCommentsAndHeader(t *testing.T) {
	body := []byte("---\ntitle: " + testTitle + "\n")

	// Every comment and blank line between the marker and the first key is dropped.
	stripped := stripLeadingComments([]byte("---\n# stale comment\n\ntitle: " + testTitle + "\n"))
	assert.Equal(t, string(body), string(stripped), "the head is a Write-time output, never inherited")

	// A body that opens with data keeps it and drops nothing.
	assert.Equal(t, string(body), string(stripLeadingComments(body)))

	// An empty head leaves the body untouched.
	assert.Equal(t, string(body), string(prependHeadComment([]byte{}, body)))

	// REUSE-IgnoreStart — the literal SPDX strings below are test fixtures, not a licensing declaration
	head := "# SPDX-FileCopyrightText: 2026 Test Holder\n#\n# SPDX-License-Identifier: MIT\n"
	// REUSE-IgnoreEnd
	assert.Equal(t, "---\n"+head+"\ntitle: "+testTitle+"\n", string(prependHeadComment([]byte(head), body)),
		"the head lands after the marker and the bare '#' separator survives")

	// A body with no document marker is returned unchanged.
	assert.Equal(t, "title: x\n", string(prependHeadComment([]byte(head), []byte("title: x\n"))))
}
