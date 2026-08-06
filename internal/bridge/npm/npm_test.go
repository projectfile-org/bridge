// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package npm_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/npm"
)

const testAliceSmith = "Alice Smith"

const testLinux = "linux"

// fixtureJSON includes known fields AND unknown keys (scripts, volta) to verify
// that the round-trip canvas preserves them.
const fixtureJSON = `{
  "name": "my-package",
  "version": "1.2.3",
  "description": "A test package",
  "license": "MIT",
  "author": "Alice Smith <alice@example.com>",
  "keywords": ["go", "test"],
  "homepage": "https://example.com",
  "bugs": {"url": "https://github.com/acme/my-package/issues"},
  "scripts": {"build": "tsc", "test": "jest"},
  "volta": {"node": "20.0.0"}
}
`

func writePackageJSON(t *testing.T, dir, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(content), 0o644))
}

// ── Read ─────────────────────────────────────────────────────────────────────

func TestReadTypedFields(t *testing.T) {
	dir := t.TempDir()
	writePackageJSON(t, dir, fixtureJSON)

	doc, err := npm.Read(dir)
	require.NoError(t, err)
	assert.Equal(t, "my-package", doc.Name)
	assert.Equal(t, "1.2.3", doc.Version)
	assert.Equal(t, "A test package", doc.Description)
	assert.Equal(t, "MIT", doc.License)
	assert.Equal(t, []string{"go", "test"}, doc.Keywords)
	assert.Equal(t, "https://example.com", doc.Homepage)
}

func TestReadRestCanvasPresent(t *testing.T) {
	dir := t.TempDir()
	writePackageJSON(t, dir, fixtureJSON)

	doc, err := npm.Read(dir)
	require.NoError(t, err)
	require.NotNil(t, doc.Rest, "Rest canvas must be populated on Read")
	assert.True(t, doc.Rest.Has("scripts"), "unknown key 'scripts' must be in Rest")
	assert.True(t, doc.Rest.Has("volta"), "unknown key 'volta' must be in Rest")
}

func TestReadMissingFile(t *testing.T) {
	dir := t.TempDir()
	_, err := npm.Read(dir)
	assert.Error(t, err)
}

// ── Write round-trip ─────────────────────────────────────────────────────────

func TestWriteRoundTripPreservesUnknownKeys(t *testing.T) {
	dir := t.TempDir()
	writePackageJSON(t, dir, fixtureJSON)

	doc, err := npm.Read(dir)
	require.NoError(t, err)

	// Mutate a known field.
	doc.Version = "2.0.0"
	require.NoError(t, npm.Write(dir, doc))

	// Re-read and verify known field updated and unknown keys survived.
	got, err := npm.Read(dir)
	require.NoError(t, err)
	assert.Equal(t, "2.0.0", got.Version)
	assert.Equal(t, "my-package", got.Name, "unmodified field must be unchanged")
	assert.True(t, got.Rest.Has("scripts"), "'scripts' must survive round-trip")
	assert.True(t, got.Rest.Has("volta"), "'volta' must survive round-trip")
}

func TestWriteNewDocNoRest(t *testing.T) {
	dir := t.TempDir()
	doc := &npm.Document{
		Name:    "fresh-pkg",
		Version: "0.1.0",
		License: "Apache-2.0",
	}
	require.NoError(t, npm.Write(dir, doc))

	got, err := npm.Read(dir)
	require.NoError(t, err)
	assert.Equal(t, "fresh-pkg", got.Name)
	assert.Equal(t, "0.1.0", got.Version)
}

// ── BuildMappers ─────────────────────────────────────────────────────────────

func TestBuildMappersFromPFForce(t *testing.T) {
	dir := t.TempDir()
	writePackageJSON(t, dir, fixtureJSON)
	doc, err := npm.Read(dir)
	require.NoError(t, err)

	pf := &projectfile.Document{
		Identity: projectfile.Identity{Name: "overridden-name", Version: "9.9.9"},
	}
	bridge := npm.Bridge{}
	mappers := bridge.BuildMappers(doc, pf)

	// Run FromPF(force=true) for each mapper.
	for _, m := range mappers {
		if m.FromPF != nil {
			m.FromPF(true)
		}
	}
	assert.Equal(t, "overridden-name", doc.Name)
}

func TestBuildMappersFromPFNoForceSkipsExisting(t *testing.T) {
	dir := t.TempDir()
	writePackageJSON(t, dir, fixtureJSON)
	doc, err := npm.Read(dir)
	require.NoError(t, err)

	pf := &projectfile.Document{
		Identity: projectfile.Identity{Name: "new-name"},
	}
	bridge := npm.Bridge{}
	mappers := bridge.BuildMappers(doc, pf)

	// force=false: existing doc.Name="my-package" must not be overwritten.
	for _, m := range mappers {
		if m.FromPF != nil {
			m.FromPF(false)
		}
	}
	assert.Equal(t, "my-package", doc.Name, "force=false must not overwrite existing ext field")
}

// ── ParsePerson ──────────────────────────────────────────────────────────────

func TestParsePersonString(t *testing.T) {
	cases := []struct {
		input string
		name  string
		email string
	}{
		{testAliceSmith, testAliceSmith, ""},
		{testAliceSmith + " <alice@example.com>", testAliceSmith, "alice@example.com"},
		{"Bob <bob@example.com> (https://bob.dev)", "Bob", "bob@example.com"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			p := npm.ParsePerson(tc.input)
			assert.Equal(t, tc.name, p.Name)
			assert.Equal(t, tc.email, p.Email)
		})
	}
}

func TestParsePersonMap(t *testing.T) {
	m := map[string]any{"name": "Charlie", "email": "charlie@example.com", "url": "https://charlie.dev"}
	p := npm.ParsePerson(m)
	assert.Equal(t, "Charlie", p.Name)
	assert.Equal(t, "charlie@example.com", p.Email)
}

// ── ParseRepository ──────────────────────────────────────────────────────────

func TestParseRepositoryGitHubShorthand(t *testing.T) {
	r := npm.ParseRepository("github:acme/my-pkg")
	assert.Equal(t, "https://github.com/acme/my-pkg", r.URL)
}

func TestParseRepositoryMap(t *testing.T) {
	m := map[string]any{"url": "https://github.com/acme/pkg.git", "type": "git"}
	r := npm.ParseRepository(m)
	assert.Equal(t, "https://github.com/acme/pkg.git", r.URL)
	assert.Equal(t, "git", r.Type)
}

// ── Output structure ─────────────────────────────────────────────────────────

func TestWriteOutputIsValidJSON(t *testing.T) {
	dir := t.TempDir()
	doc := &npm.Document{Name: "check-json", Version: "1.0.0"}
	require.NoError(t, npm.Write(dir, doc))

	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	require.NoError(t, err)
	var raw map[string]any
	assert.NoError(t, json.Unmarshal(data, &raw), "output must be valid JSON")
}

// ── Dependencies are npm's, not the projectfile's ────────────────────────────

const scopedDepsJSON = `{
  "name": "my-package",
  "version": "1.2.3",
  "dependencies": {"@astrojs/starlight": "0.41.3", "astro": "7.1.0"},
  "devDependencies": {"@types/node": "26.1.1"}
}
`

// A dependency set belongs to the package manager and its lockfile. The
// projectfile could only ever mirror the DIRECT deps — never a peer, never a
// transitive — so it read as a complete manifest while being a lossy copy that
// still had to be hand-synced. No mapper reads or writes package.json
// dependencies now: they survive a full sync as ordinary round-tripped content.
func TestDependenciesRoundTripUntouchedByMappers(t *testing.T) {
	dir := t.TempDir()
	writePackageJSON(t, dir, scopedDepsJSON)
	doc, err := npm.Read(dir)
	require.NoError(t, err)

	pf := &projectfile.Document{Identity: projectfile.Identity{Name: "my-package"}}
	for _, m := range (npm.Bridge{}).BuildMappers(doc, pf) {
		if m.ToPF != nil {
			m.ToPF(true)
		}
		if m.FromPF != nil {
			m.FromPF(true)
		}
	}
	require.NoError(t, npm.Write(dir, doc))

	got, err := npm.Read(dir)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"@astrojs/starlight": "0.41.3", "astro": "7.1.0"}, got.Dependencies,
		"dependencies must survive a force sync byte-for-byte")
	assert.Equal(t, map[string]string{"@types/node": "26.1.1"}, got.DevDependencies)
}

// ── Platform targeting → §4.8a extension fields ──────────────────────────────

// Platform targeting is a build concern, so spec §4.8a puts it in the
// org.projectfile.operating-system / .architecture extension fields rather than
// in requirements. Both directions are exercised: the list leaves as []string
// and comes back off a parsed document as []any, and only one of those two
// spellings would be caught by a one-way test.
func TestPlatformMapsToExtensionFields(t *testing.T) {
	dir := t.TempDir()
	writePackageJSON(t, dir, `{"name":"p","version":"1.0.0","os":["`+testLinux+`","win32"],"cpu":["x64"]}`)
	doc, err := npm.Read(dir)
	require.NoError(t, err)

	pf := &projectfile.Document{Identity: projectfile.Identity{Name: "p"}}
	for _, m := range (npm.Bridge{}).BuildMappers(doc, pf) {
		if m.ToPF != nil {
			m.ToPF(true)
		}
	}

	gotOS, ok := projectfile.LookupExtension(pf, "org.projectfile.operating-system")
	require.True(t, ok, "operating-system extension written")
	assert.Equal(t, []string{testLinux, "windows"}, gotOS, "npm win32 normalises to windows")
	gotArch, ok := projectfile.LookupExtension(pf, "org.projectfile.architecture")
	require.True(t, ok, "architecture extension written")
	assert.Equal(t, []string{"amd64"}, gotArch, "npm x64 normalises to amd64")
	assert.Nil(t, pf.Requirements, "nothing lands in requirements")
}

// The reverse direction reads the extension off a parsed document, where the
// list is []any rather than []string.
func TestPlatformFromParsedExtensionFields(t *testing.T) {
	dir := t.TempDir()
	writePackageJSON(t, dir, `{"name":"p","version":"1.0.0"}`)
	doc, err := npm.Read(dir)
	require.NoError(t, err)

	pf := &projectfile.Document{
		Identity: projectfile.Identity{Name: "p"},
		Extensions: map[string]any{
			"org.projectfile.operating-system": []any{testLinux, "windows"},
			"org.projectfile.architecture":     []any{"amd64"},
		},
	}
	for _, m := range (npm.Bridge{}).BuildMappers(doc, pf) {
		if m.FromPF != nil {
			m.FromPF(true)
		}
	}

	assert.Equal(t, []string{testLinux, "win32"}, doc.OS)
	assert.Equal(t, []string{"x64"}, doc.CPU)
}
