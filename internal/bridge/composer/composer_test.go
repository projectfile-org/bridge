// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package composer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/v4/internal/bridge/core"
)

// fixtureJSON carries every mapped field AND unknown keys (autoload, scripts,
// volta-free equivalents) so the round-trip canvas is exercised end to end.
const fixtureJSON = `{
    "name": "acme/widget",
    "description": "A widget",
    "version": "1.2.3",
    "type": "library",
    "keywords": ["php", "widget"],
    "homepage": "https://example.test",
    "time": "2026-01-02T03:04:05+00:00",
    "license": ["MIT", "Apache-2.0"],
    "authors": [
        {"name": "Ada Lovelace", "email": "ada@example.test", "homepage": "https://ada.example.test", "role": "Developer"}
    ],
    "support": {
        "issues": "https://example.test/issues",
        "source": "https://example.test/source.git",
        "docs": "https://example.test/docs",
        "chat": "https://example.test/chat",
        "forum": "https://example.test/forum",
        "email": "help@example.test",
        "wiki": "https://example.test/wiki"
    },
    "funding": [
        {"type": "custom", "url": "https://example.test/fund"}
    ],
    "require": {"php": ">=8.2", "ext-json": "*", "acme/other": "^1.0"},
    "require-dev": {"phpunit/phpunit": "^10"},
    "minimum-stability": "stable",
    "prefer-stable": true,
    "abandoned": false,
    "autoload": {"psr-4": {"Acme\\": "src/"}},
    "scripts": {"test": "phpunit"}
}
`

const (
	testComposerName     = "acme/widget"
	testVendorAcme       = "acme"
	testVendorWidget     = "widget"
	testDescription      = "A widget"
	testVersionOne       = "1.0.0"
	testVersionTwo       = "2.0.0"
	testTimeJan          = "2026-01-01"
	testTimeFeb          = "2026-02-02"
	testSPDXMIT          = "MIT"
	testSPDXApache       = "Apache-2.0"
	testSPDXGPL          = "GPL-3.0"
	testTypeLibrary      = "library"
	testPHPFloor         = ">=8.2"
	testPHPFloorNew      = ">=9.0"
	testAcmeConstraint   = "^1.0"
	testFundingType      = "custom"
	testAdaLovelace      = "Ada Lovelace"
	testAda              = "Ada"
	testLovelace         = "Lovelace"
	testGraceHopper      = "Grace Hopper"
	testGrace            = "Grace"
	testRoleAuthor       = "author"
	testRoleMaintainer   = "maintainer"
	testRoleDeveloper    = "Developer"
	testKeywordOne       = "one"
	testKeywordTwo       = "two"
	testMutated          = "mutated"
	testWidgetKeyword    = "widget"
	testPHPKey           = "php"
	testNodeKey          = "node"
	testKeyAbandoned     = "abandoned"
	testKeySupportChat   = "support.chat"
	testExtJSON          = "ext-json"
	testExtGD            = "ext-gd"
	testURLNew           = "https://new.test"
	testURLOld           = "https://old.test"
	testAdaEmail         = "ada@example.test"
	testAdaHomepage      = "https://ada.example.test"
	testElsewhereIssues  = "https://elsewhere.test/issues"
	testChangedName      = "changed/name"
	testAcmePackage      = "acme/other"
	testComposerNS       = "composer.acme"
	testKeyType          = "type"
	testKeySupportForum  = "support.forum"
	testKeySupportIssues = "support.issues"
	testKeyMinStability  = "minimum-stability"
	testKeyPreferStable  = "prefer-stable"
	testMinStability     = "stable"
	testFundFixtureURL   = "https://example.test/fund"
)

func writeComposer(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, Filename), []byte(content), 0o644))
	return dir
}

func readFile(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, Filename))
	require.NoError(t, err)
	return string(data)
}

// ── Read / Exists / FullPath ──────────────────────────────────────────────────

func TestReadParsesEveryMappedField(t *testing.T) {
	dir := writeComposer(t, fixtureJSON)

	doc, err := Read(dir)
	require.NoError(t, err)
	assert.Equal(t, testComposerName, doc.Name)
	assert.Equal(t, "1.2.3", doc.Version)
	assert.Equal(t, testDescription, doc.Description)
	assert.Equal(t, testTypeLibrary, doc.Type)
	assert.Equal(t, []string{testPHPKey, testWidgetKeyword}, doc.Keywords)
	assert.Equal(t, "2026-01-02T03:04:05+00:00", doc.Time)
	assert.Equal(t, "stable", doc.MinimumStability)
	assert.Equal(t, []Person{{Name: testAdaLovelace, Email: testAdaEmail, Homepage: testAdaHomepage, Role: testRoleDeveloper}}, doc.Authors)
	require.NotNil(t, doc.Support)
	assert.Equal(t, "https://example.test/issues", doc.Support.Issues)
	assert.Equal(t, "help@example.test", doc.Support.Email, "an unmapped support key still parses")
	assert.Equal(t, []FundingEntry{{Type: testFundingType, URL: testFundFixtureURL}}, doc.Funding)
	assert.Equal(t, map[string]string{testPHPKey: testPHPFloor, testExtJSON: "*", testAcmePackage: testAcmeConstraint}, doc.Require)
}

func TestReadKeepsRawCanvas(t *testing.T) {
	dir := writeComposer(t, fixtureJSON)

	doc, err := Read(dir)
	require.NoError(t, err)
	require.NotNil(t, doc.Rest)
	assert.True(t, doc.Rest.Has("autoload"), "unknown key must survive in Rest")
	assert.True(t, doc.Rest.Has("scripts"), "unknown key must survive in Rest")
	assert.Contains(t, doc.Rest.Keys(), "require-dev")
}

func TestReadMissingFile(t *testing.T) {
	_, err := Read(t.TempDir())
	assert.Error(t, err)
}

func TestReadMalformedJSON(t *testing.T) {
	_, err := Read(writeComposer(t, `{"name": `))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse")
}

func TestExistsAndFullPath(t *testing.T) {
	empty := t.TempDir()
	assert.False(t, Exists(empty))

	dir := writeComposer(t, "{}\n")
	assert.True(t, Exists(dir))
	assert.Equal(t, filepath.Join(dir, Filename), FullPath(dir))
	assert.True(t, filepath.IsAbs(FullPath(dir)), "an absolute path is composed from any input")

	rel, err := filepath.Rel(mustGetwd(t), dir)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, Filename), FullPath(rel), "a relative dir is resolved to the same file")
}

func mustGetwd(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	require.NoError(t, err)
	return wd
}

// ── License value shapes ──────────────────────────────────────────────────────

func TestLicenseStringShapes(t *testing.T) {
	cases := map[string]struct {
		in   any
		want string
	}{
		"string":  {in: " MIT ", want: testSPDXMIT},
		"strings": {in: []string{testSPDXMIT, testSPDXApache}, want: testSPDXMIT + " OR " + testSPDXApache},
		"anys":    {in: []any{testSPDXMIT, "", testSPDXGPL}, want: testSPDXMIT + " OR " + testSPDXGPL},
		"blank":   {in: []any{testSPDXMIT, "  ", testSPDXGPL}, want: testSPDXMIT + " OR    OR " + testSPDXGPL},
		"number":  {in: 7, want: ""},
		"nil":     {in: nil, want: ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, LicenseString(tc.in))
		})
	}
}

func TestParseLicenseShapes(t *testing.T) {
	assert.Nil(t, ParseLicense("  "))
	assert.Equal(t, testSPDXMIT, ParseLicense(" "+testSPDXMIT+" "))
	assert.Equal(t, []string{testSPDXMIT, testSPDXApache}, ParseLicense(testSPDXMIT+" OR "+testSPDXApache))
	assert.Equal(t, []string{testSPDXMIT, testSPDXGPL}, ParseLicense(testSPDXMIT+" OR  OR "+testSPDXGPL))
	assert.Equal(t, []string{testSPDXMIT, testSPDXApache + " OR"}, ParseLicense(testSPDXMIT+" OR "+testSPDXApache+" OR "), "the leading trim eats the space an empty trailing arm needs")
	assert.Equal(t, testSPDXMIT+" OR", ParseLicense(testSPDXMIT+" OR "), "no complete separator arm leaves the expression a plain id")
}

// ── Write ────────────────────────────────────────────────────────────────────

func TestWriteRoundTripKeepsUnknownKeys(t *testing.T) {
	dir := writeComposer(t, fixtureJSON)

	doc, err := Read(dir)
	require.NoError(t, err)
	doc.Version = testVersionTwo
	doc.Description = "A better widget"
	require.NoError(t, Write(dir, doc))

	got, err := Read(dir)
	require.NoError(t, err)
	assert.Equal(t, testVersionTwo, got.Version)
	assert.Equal(t, "A better widget", got.Description)
	assert.Equal(t, testComposerName, got.Name, "an untouched field persists")
	assert.True(t, got.Rest.Has("autoload"), "unknown keys survive a write")
	assert.True(t, got.Rest.Has("scripts"), "unknown keys survive a write")
	assert.Equal(t, map[string]string{"phpunit/phpunit": "^10"}, got.RequireDev)
}

func TestWriteDropsAnEmptySupportObject(t *testing.T) {
	dir := writeComposer(t, fixtureJSON)
	doc, err := Read(dir)
	require.NoError(t, err)

	doc.Support = &Support{Issues: "", Source: "", Docs: "", Chat: "", Forum: "", Email: "", Wiki: ""}
	require.NoError(t, Write(dir, doc))

	got, err := Read(dir)
	require.NoError(t, err)
	assert.Nil(t, got.Support, "a support object with no mapped value must not be written")
	assert.True(t, got.Rest.Has("autoload"))
}

func TestWriteKeepsEmptyRequireAndPreferStable(t *testing.T) {
	dir := writeComposer(t, fixtureJSON)
	doc, err := Read(dir)
	require.NoError(t, err)

	doc.Require = nil
	doc.PreferStable = nil
	doc.Type = ""
	doc.Abandoned = nil
	require.NoError(t, Write(dir, doc))

	var keys map[string]any
	require.NoError(t, json.Unmarshal([]byte(readFile(t, dir)), &keys))
	assert.Contains(t, keys, "require", "an omitted typed require keeps the canvas entry")
	assert.Contains(t, keys, "require-dev", "an omitted typed require-dev keeps the canvas entry")
	assert.Contains(t, keys, "prefer-stable", "a nil prefer-stable keeps the canvas entry")
	assert.NotContains(t, keys, "type", "an emptied type drops from the canvas")
	assert.NotContains(t, keys, "abandoned", "a nil abandoned drops from the canvas")
}

func TestWriteFreshDocumentUsesCanonicalKeyOrder(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, Write(dir, &Document{
		Name:        "fresh/pkg",
		Version:     "0.1.0",
		Description: "Fresh",
		License:     testSPDXMIT,
		Keywords:    []string{testKeywordOne},
		Authors:     []Person{{Name: testGraceHopper}},
		Support:     &Support{Issues: "https://example.test/issues"},
		Funding:     []FundingEntry{{Type: testFundingType, URL: testFundFixtureURL}},
	}))

	got, err := Read(dir)
	require.NoError(t, err)
	assert.Equal(t, "fresh/pkg", got.Name)
	require.NotNil(t, got.Support)
	assert.Equal(t, "https://example.test/issues", got.Support.Issues)
	assert.Equal(t, []FundingEntry{{Type: testFundingType, URL: testFundFixtureURL}}, got.Funding)
	assert.Equal(t, []string{
		"name", "description", "version", composerKeywords,
		"license", "authors", "support", "funding",
	}, got.Rest.Keys(), "a fresh document follows the canonical key order")
}

func TestWriteToUnwritableDirFails(t *testing.T) {
	err := Write(filepath.Join(t.TempDir(), "missing"), &Document{Name: "x/y"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "composer.json")
}

// ── Clone ────────────────────────────────────────────────────────────────────

func TestCloneIsAIndependentCopy(t *testing.T) {
	assert.Nil(t, (*Document)(nil).Clone())

	dir := writeComposer(t, fixtureJSON)
	doc, err := Read(dir)
	require.NoError(t, err)

	cp := doc.Clone()
	require.NotNil(t, cp)
	cp.Name = testChangedName
	cp.Keywords[0] = testMutated
	cp.Require[testPHPKey] = testPHPFloorNew
	cp.Authors[0].Name = "Someone Else"
	cp.Funding[0].URL = "https://elsewhere.test"
	cp.License = testSPDXMIT
	cp.PreferStable = false
	require.NotNil(t, cp.Support)
	cp.Support.Issues = testElsewhereIssues

	assert.Equal(t, testComposerName, doc.Name, "a scalar change must not reach the original")
	assert.Equal(t, []string{testPHPKey, testWidgetKeyword}, doc.Keywords)
	assert.Equal(t, map[string]string{testPHPKey: testPHPFloor, testExtJSON: "*", testAcmePackage: testAcmeConstraint}, doc.Require)
	assert.Equal(t, []Person{{Name: testAdaLovelace, Email: testAdaEmail, Homepage: testAdaHomepage, Role: testRoleDeveloper}}, doc.Authors)
	assert.Equal(t, []FundingEntry{{Type: testFundingType, URL: testFundFixtureURL}}, doc.Funding)
	assert.Equal(t, []any{testSPDXMIT, testSPDXApache}, doc.License)
	assert.Equal(t, true, doc.PreferStable)
	assert.Equal(t, "https://example.test/issues", doc.Support.Issues)
	assert.True(t, cp.Rest.Has("autoload"), "the cloned canvas stays usable")
}

func TestCloneCopiesNestedContainers(t *testing.T) {
	doc := &Document{
		License:      map[string]any{"urls": []any{"https://a.test", "https://b.test"}},
		PreferStable: []any{"one", "two"},
		Abandoned:    []string{"x"},
	}

	cp := doc.Clone()
	nested := cp.License.(map[string]any)
	nested["urls"].([]any)[0] = "https://mutated.test"
	cp.PreferStable.([]any)[0] = testMutated
	cp.Abandoned.([]string)[0] = testMutated

	assert.Equal(t, "https://a.test", doc.License.(map[string]any)["urls"].([]any)[0])
	assert.Equal(t, testKeywordOne, doc.PreferStable.([]any)[0])
	assert.Equal(t, "x", doc.Abandoned.([]string)[0])
}

// ── Bridge surface ───────────────────────────────────────────────────────────

func TestBridgeSurface(t *testing.T) {
	b := Bridge{}
	assert.Equal(t, "composer", b.Name())
	assert.Equal(t, Filename, b.Filename())
	assert.Nil(t, b.Aliases())
	assert.Equal(t, []string{"php"}, b.Stacks())
	assert.Equal(t, core.Policy{}, b.Policy())

	ext, pf := b.Labels()
	assert.Equal(t, Filename, ext)
	assert.Equal(t, "projectfile", pf)

	dir := writeComposer(t, fixtureJSON)
	assert.True(t, b.Exists(dir))
	assert.Equal(t, FullPath(dir), b.FullPath(dir, nil))

	read, err := b.Read(dir)
	require.NoError(t, err)
	assert.Equal(t, testComposerName, read.(*Document).Name)
	assert.IsType(t, &Document{}, b.NewEmpty())
}

func TestBridgeWriteRejectsAForeignDocument(t *testing.T) {
	err := Bridge{}.Write(t.TempDir(), "not a composer document")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected document type")
}

func TestBridgeCloneAndMappers(t *testing.T) {
	b := Bridge{}
	doc := &Document{Name: testComposerName}
	assert.Equal(t, doc, b.Clone(doc))

	passthrough := b.Clone("anything else")
	assert.Equal(t, "anything else", passthrough)
	assert.Nil(t, b.Clone(nil))

	empty := b.BuildMappers(nil, &projectfile.Document{})
	require.Len(t, empty, 19, "nineteen mapped composer fields")
	for _, m := range empty {
		assert.NotNil(t, m.FromPF, m.ExtKey)
		assert.NotNil(t, m.ToPF, m.ExtKey)
	}
}
