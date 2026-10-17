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
	"projectfile.org/projectfile/bridge/v4/internal/bridge/npm"
)

const testAliceSmith = "Alice Smith"

const testLinux = "linux"

const testAMD64 = "amd64"

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

func TestBuildMappersFromPFSkipsNonSemVer(t *testing.T) {
	for _, version := range []string{"24.04", "16.2"} {
		dir := t.TempDir()
		writePackageJSON(t, dir, fixtureJSON)
		doc, err := npm.Read(dir)
		require.NoError(t, err)
		before := doc.Version
		pf := &projectfile.Document{Identity: projectfile.Identity{Name: "x", Version: version}}
		for _, m := range (npm.Bridge{}).BuildMappers(doc, pf) {
			if m.FromPF != nil {
				m.FromPF(true)
			}
		}
		assert.Equal(t, before, doc.Version, version)
	}
}

func TestBuildMappersFromPFRepositoryFollowsPackages(t *testing.T) {
	dir := t.TempDir()
	writePackageJSON(t, dir, fixtureJSON)
	doc, err := npm.Read(dir)
	require.NoError(t, err)

	pf := &projectfile.Document{
		Repositories: []projectfile.Repository{
			{URL: "ssh://git@kiota.ch/acme/pkg.git", Role: projectfile.RepositoryRoleOrigin},
			{URL: "https://github.com/acme/pkg.git", Packages: true},
		},
	}
	for _, m := range (npm.Bridge{}).BuildMappers(doc, pf) {
		if m.FromPF != nil {
			m.FromPF(true)
		}
	}
	assert.Equal(t, "https://github.com/acme/pkg.git", npm.ParseRepository(doc.Repository).URL)
}

func TestBuildMappersPlatformFollowsArtifact(t *testing.T) {
	dir := t.TempDir()
	writePackageJSON(t, dir, fixtureJSON)
	doc, err := npm.Read(dir)
	require.NoError(t, err)

	pf := &projectfile.Document{}
	projectfile.SetExtension(pf, "org.projectfile.architecture", []string{testAMD64})
	projectfile.SetExtension(pf, "org.projectfile.operating-system", []string{testLinux})
	projectfile.SetExtension(pf, "org.projectfile.artifacts", map[string]any{
		"npm-package": map[string]any{
			"kind": "package", "registry": "npm",
			"architecture":     []any{testAMD64, "arm64"},
			"operating-system": []any{testLinux, "darwin"},
		},
	})
	for _, m := range (npm.Bridge{}).BuildMappers(doc, pf) {
		if m.FromPF != nil {
			m.FromPF(true)
		}
	}
	assert.Equal(t, []string{"x64", "arm64"}, doc.CPU)
	assert.Equal(t, []string{testLinux, "darwin"}, doc.OS)

	doc.CPU = []string{"riscv64"}
	for _, m := range (npm.Bridge{}).BuildMappers(doc, pf) {
		if m.ToPF != nil {
			m.ToPF(true)
		}
	}
	got, _ := projectfile.LookupExtension(pf, "org.projectfile.architecture")
	assert.Equal(t, []string{testAMD64}, got, "package.json must not overwrite the project-level list")
}

// A forced FromPF pass over a document the previous pass just wrote must be
// silent — otherwise every --check reports drift on an in-sync project.
func TestBuildMappersFromPFIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	writePackageJSON(t, dir, `{"name": "my-package"}`)
	pfPath := filepath.Join(dir, "projectfile.yaml")
	require.NoError(t, os.WriteFile(pfPath, []byte(`---
identity:
  name: my-package
  version: 1.2.3
  summary: A test package
keywords: [go, test]
repositories:
  - role: origin
    type: git
    url: https://github.com/acme/my-package.git
links:
  - type: bugs
    url: https://github.com/acme/my-package/issues
  - type: homepage
    url: https://example.com
`), 0o644))
	pf, err := projectfile.Read(pfPath)
	require.NoError(t, err)
	doc, err := npm.Read(dir)
	require.NoError(t, err)
	bridge := npm.Bridge{}

	for _, m := range bridge.BuildMappers(doc, pf) {
		if m.FromPF != nil {
			m.FromPF(true)
		}
	}
	for _, m := range bridge.BuildMappers(doc, pf) {
		if m.FromPF == nil {
			continue
		}
		assert.Emptyf(t, m.FromPF(true), "%s reported a change on an already-synced document", m.ExtKey)
	}
}

func TestBuildMappersFromPFNoForceConverges(t *testing.T) {
	dir := t.TempDir()
	writePackageJSON(t, dir, fixtureJSON)
	doc, err := npm.Read(dir)
	require.NoError(t, err)

	pf := &projectfile.Document{
		Identity: projectfile.Identity{Name: "new-name"},
	}
	bridge := npm.Bridge{}
	mappers := bridge.BuildMappers(doc, pf)

	// force=false: a modeled pf value still converges, only clears are withheld.
	for _, m := range mappers {
		if m.FromPF != nil {
			m.FromPF(false)
		}
	}
	assert.Equal(t, "new-name", doc.Name, "force=false must converge a modeled pf value")
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
	assert.Equal(t, []string{testAMD64}, gotArch, "npm x64 normalises to amd64")
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
			"org.projectfile.architecture":     []any{testAMD64},
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

// TestWriteDropsClearedMappedKey pins that a cleared mapped field leaves package.json while foreign keys stay.
// `private: false` is a MAPPED key resolved to "public" (spec §4.3), so it goes with bugs rather than surviving as foreign.
func TestWriteDropsClearedMappedKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "package.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"name":"x","private":false,"publishConfig":{"registry":"https://registry.example"},"bugs":{"url":"https://stale.test/issues"},"scripts":{"t":"true"}}`), 0o644))
	doc, err := npm.Read(dir)
	require.NoError(t, err)
	doc.Bugs = nil
	require.NoError(t, npm.Write(dir, doc))

	var got map[string]any
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &got))
	assert.NotContains(t, got, "bugs")
	assert.NotContains(t, got, "private")
	assert.Contains(t, got, "scripts")
	assert.Contains(t, got, "publishConfig")
}

// npmArtifactDoc declares the npm package artifact the visibility projection is scoped to.
func npmArtifactDoc(visibility string) *projectfile.Document {
	return &projectfile.Document{
		Identity: projectfile.Identity{Name: "my-package", Visibility: visibility},
		Extensions: map[string]any{
			"org.projectfile.artifacts": map[string]any{
				"npm-package": map[string]any{"kind": "package", "registry": "npm", "name": "my-package"},
			},
		},
	}
}

// TestVisibilityProjectedOntoPrivateKey pins the three spec states through the writer:
// an explicit public drops the key, private and undeclared both write it.
func TestVisibilityProjectedOntoPrivateKey(t *testing.T) {
	states := map[string]struct {
		declared string
		wantKey  bool
	}{
		"public":     {declared: projectfile.VisibilityPublic, wantKey: false},
		"private":    {declared: projectfile.VisibilityPrivate, wantKey: true},
		"undeclared": {declared: "", wantKey: true},
	}
	for name, tc := range states {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			writePackageJSON(t, dir, fixtureJSON)
			pkg, err := npm.Read(dir)
			require.NoError(t, err)
			pf := npmArtifactDoc(tc.declared)

			for _, m := range (npm.Bridge{}).BuildMappers(pkg, pf) {
				if m.FromPF != nil {
					m.FromPF(false)
				}
			}
			require.NoError(t, npm.Write(dir, pkg))

			var got map[string]any
			data, err := os.ReadFile(filepath.Join(dir, "package.json"))
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(data, &got))
			guard, present := got["private"]
			assert.Equal(t, tc.wantKey, present, "private key present")
			if present {
				assert.Equal(t, true, guard, "private value")
			}
		})
	}
}

// TestPublicVisibilityRemovesAHandGuardedManifest pins that an explicit public overrides a guard written by hand.
func TestPublicVisibilityRemovesAHandGuardedManifest(t *testing.T) {
	dir := t.TempDir()
	writePackageJSON(t, dir, `{"name": "acme-tool", "private": true}`)
	pkg, err := npm.Read(dir)
	require.NoError(t, err)
	pf := npmArtifactDoc(projectfile.VisibilityPublic)

	for _, m := range (npm.Bridge{}).BuildMappers(pkg, pf) {
		if m.FromPF != nil {
			m.FromPF(false)
		}
	}
	require.NoError(t, npm.Write(dir, pkg))

	pkg, err = npm.Read(dir)
	require.NoError(t, err)
	assert.False(t, pkg.Private)
	assert.False(t, pkg.Rest.Has("private"), "the key itself leaves, npm has no use for \"private\": false")
}

// TestPrivateManifestTeachesTheDocument pins the discovery direction, and that an unguarded manifest teaches nothing.
func TestPrivateManifestTeachesTheDocument(t *testing.T) {
	guarded := func(t *testing.T, content string) string {
		t.Helper()
		dir := t.TempDir()
		writePackageJSON(t, dir, content)
		pkg, err := npm.Read(dir)
		require.NoError(t, err)
		pf := npmArtifactDoc("")
		for _, m := range (npm.Bridge{}).BuildMappers(pkg, pf) {
			if m.ToPF != nil {
				m.ToPF(false)
			}
		}
		return pf.Identity.Visibility
	}
	assert.Equal(t, projectfile.VisibilityPrivate, guarded(t, `{"name": "acme-tool", "private": true}`))
	assert.Equal(t, "", guarded(t, `{"name": "acme-tool"}`), "an unguarded manifest says nothing about publicity")
	assert.Equal(t, "", guarded(t, `{"name": "acme-tool", "private": false}`), "only a guard teaches the document")
}

// TestDeclaredArtifactNameWins pins that a declared npm package artifact names the package and is never read back into identity.
func TestDeclaredArtifactNameWins(t *testing.T) {
	dir := t.TempDir()
	writePackageJSON(t, dir, `{"name": "acme-tool"}`)
	pfPath := filepath.Join(dir, "projectfile.yaml")
	require.NoError(t, os.WriteFile(pfPath, []byte(`---
identity:
  name: tool
  namespace: org.acme
org:
  projectfile:
    artifacts:
      npm-package:
        kind: package
        registry: npm
        name: ${identity.name}
`), 0o644))
	pf, err := projectfile.Read(pfPath)
	require.NoError(t, err)
	doc, err := npm.Read(dir)
	require.NoError(t, err)

	for _, m := range (npm.Bridge{}).BuildMappers(doc, pf) {
		if m.ExtKey == "name" {
			assert.Empty(t, m.ToPF(true), "a declared artifact owns the npm name")
			assert.Equal(t, "tool", m.FromPF(true))
		}
	}
	assert.Equal(t, "tool", doc.Name)
	assert.Equal(t, "org.acme", pf.Identity.Namespace)
	assert.Equal(t, "tool", pf.Identity.Name)
}

// TestVisibilityIgnoresAnUndeclaredPackage pins the scoping: an app whose `private: true` guards the npm
// package keeps it, because the document declares no npm artifact and therefore speaks for neither side.
func TestVisibilityIgnoresAnUndeclaredPackage(t *testing.T) {
	dir := t.TempDir()
	writePackageJSON(t, dir, `{"name": "acme-web", "private": true}`)
	pkg, err := npm.Read(dir)
	require.NoError(t, err)
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: "acme-web", Visibility: projectfile.VisibilityPublic}}

	for _, m := range (npm.Bridge{}).BuildMappers(pkg, pf) {
		if m.FromPF != nil {
			m.FromPF(false)
		}
	}
	require.NoError(t, npm.Write(dir, pkg))

	pkg, err = npm.Read(dir)
	require.NoError(t, err)
	assert.True(t, pkg.Private, "a public web app's npm guard is not the project's visibility")
	assert.True(t, pkg.Rest.Has("private"))
}
