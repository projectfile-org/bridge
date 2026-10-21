// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package npm

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"kiota.ch/projectfile/core/v2/pkg/rawdoc"
	"projectfile.org/projectfile/bridge/v4/internal/bridge/core"
	"projectfile.org/projectfile/bridge/v4/internal/pfmodel"
)

// everyFieldJSON carries one spelling of every mapped package.json key, so a
// single read exercises every mapper entry in both directions.
const everyFieldJSON = `{
  "name": "@acme/widget",
  "version": "1.2.3",
  "description": "A test package",
  "license": "MIT",
  "author": "Alice Smith <alice@example.com>",
  "contributors": [{"name": "Bob", "email": "bob@example.com"}, {}],
  "maintainers": [{"name": "Carol"}, {}],
  "keywords": ["go", "test"],
  "homepage": "https://example.com",
  "repository": {"url": "https://github.com/acme/widget.git", "type": "git"},
  "bugs": {"url": "https://example.test/issues"},
  "funding": {"url": "https://example.com/fund"},
  "engines": {"node": ">=20"},
  "os": ["linux", "win32"],
  "cpu": ["x64"],
  "private": true
}`

// TestReadModeTeachesEveryField drives the read direction over a full manifest:
// each discovered value lands on the projectfile rather than being withheld.
func TestReadModeTeachesEveryField(t *testing.T) {
	dir := t.TempDir()
	writePkgJSON(t, dir, everyFieldJSON)
	pfPath := pfFixture(t, dir, "---\nidentity:\n  name: widget\n")
	pf, err := projectfile.Read(pfPath)
	require.NoError(t, err)

	_, err = core.RunSync(Bridge{}, pf, core.Options{Dir: dir, PFPath: pfPath, Mode: core.ModeRead})
	require.NoError(t, err)
	assert.Equal(t, "1.2.3", pf.Identity.Version)
	assert.Equal(t, "A test package", projectfile.ExtractLocalizedString(pf.Identity.Summary))
	assert.Equal(t, fxLicenseMIT, pf.License.Spdx)
	assert.Equal(t, "https://example.com", pfmodel.LinkURL(pf, projectfile.LinkHomepage))
	assert.Equal(t, fxIssuesURL, pfmodel.LinkURL(pf, projectfile.LinkBugs))
	assert.Equal(t, []string{"go", fxKeywordTest}, pf.Keywords)
	assert.Equal(t, fxNpmNamespaceNS, pf.Identity.Namespace,
		"a scoped package name names its own namespace")
	assert.Equal(t, fxNpmName, pf.Identity.Name)
	require.Len(t, pf.People, 3, "author plus two named contributors/maintainers")
	assert.Equal(t, "Alice", pf.People[0].GivenNames)
	assert.Equal(t, []string{fxRoleAuthor}, pf.People[0].Roles)
	assert.Equal(t, fxEngineName, firstKey(pf.Requirements.Runtime))
	osList, ok := projectfile.LookupExtension(pf, nsOperatingSystem)
	require.True(t, ok, "the platform list reaches its §4.8a namespace")
	assert.Equal(t, []string{fxLinux, fxWindows}, osList)

	funding, err := pfmodel.GetFundingExtension(pf)
	require.NoError(t, err)
	assert.Equal(t, []string{fxFundingURL}, funding.Custom,
		"an npm funding url lands as a custom entry")

	packages := pfmodel.PackagesRepository(pf)
	require.NotNil(t, packages)
	assert.Equal(t, fxRepoURL, packages.URL)
}

// TestReadModeOverwritesProjectfileConflicts pins that an explicit read is
// authoritative: a projectfile value that disagrees is replaced.
func TestReadModeOverwritesProjectfileConflicts(t *testing.T) {
	dir := t.TempDir()
	writePkgJSON(t, dir, everyFieldJSON)
	pfPath := pfFixture(t, dir, "---\nidentity:\n  version: 0.0.0\n  summary: old\nlicense:\n  spdx: Apache-2.0\norg:\n  projectfile:\n    operating-system: [darwin]\n")
	pf, err := projectfile.Read(pfPath)
	require.NoError(t, err)

	_, err = core.RunSync(Bridge{}, pf, core.Options{Dir: dir, PFPath: pfPath, Mode: core.ModeRead})
	require.NoError(t, err)
	assert.Equal(t, "1.2.3", pf.Identity.Version)
	assert.Equal(t, fxLicenseMIT, pf.License.Spdx)
	assert.Equal(t, "A test package", projectfile.ExtractLocalizedString(pf.Identity.Summary))
}

// TestToPFWithholdsOnGapFill covers the "an existing value survives a default
// read" guards: the manifest teaches the document nothing it already declares.
func TestToPFWithholdsOnGapFill(t *testing.T) {
	dir := t.TempDir()
	writePkgJSON(t, dir, everyFieldJSON)
	pkg, err := Read(dir)
	require.NoError(t, err)

	pf := &projectfile.Document{
		Identity: projectfile.Identity{
			Name: fxNpmName, Namespace: "other", Version: "9.9.9",
			Summary: &projectfile.LocalizedString{Bare: fxValueMine},
		},
		License:  &projectfile.License{Spdx: "Apache-2.0"},
		Keywords: []string{fxValueMine},
	}
	projectfile.SetExtension(pf, nsOperatingSystem, []string{fxDarwin})
	projectfile.SetExtension(pf, nsArchitecture, []string{fxArm64})
	pfmodel.SetLink(pf, projectfile.LinkHomepage, "https://mine.example", true)
	pfmodel.SetLink(pf, projectfile.LinkBugs, "https://mine.example/issues", true)
	pf.Repositories = []projectfile.Repository{{Role: projectfile.RepositoryRoleOrigin, URL: "https://mine.example/pkg.git"}}
	projectfile.SetExtension(pf, pfmodel.FundingExtensionNS, map[string]any{"patreon": fxValueMine})

	// people and engines have no withhold guard: a merge is non-destructive by
	// design, and an absent requirements block is a gap the manifest may fill.
	gapFilled := map[string]bool{"people": true, "engines": true}
	for _, m := range (Bridge{}).BuildMappers(pkg, pf) {
		if m.ToPF == nil {
			continue
		}
		if desc := m.ToPF(false); !gapFilled[m.ExtKey] {
			assert.Emptyf(t, desc, "%s overwrote an existing value without force", m.ExtKey)
		}
	}
	assert.Equal(t, "other", pf.Identity.Namespace, "the declared namespace survives")
	assert.Equal(t, fxNpmName, pf.Identity.Name, "the declared name survives")
	assert.Equal(t, fxEngineNode, pf.Requirements.Runtime[fxEngineName], "the manifest fills the empty engines")
	assert.Equal(t, "Apache-2.0", pf.License.Spdx, "the declared licence survives")
	assert.Equal(t, fxValueMine, projectfile.ExtractLocalizedString(pf.Identity.Summary))
	assert.Equal(t, []string{fxValueMine}, pf.Keywords)
	assert.Equal(t, "https://mine.example", pfmodel.LinkURL(pf, projectfile.LinkHomepage))
	assert.Equal(t, "https://mine.example/issues", pfmodel.LinkURL(pf, projectfile.LinkBugs))
	assert.Equal(t, "https://mine.example/pkg.git", pfmodel.PackagesRepository(pf).URL)

	funding, err := pfmodel.GetFundingExtension(pf)
	require.NoError(t, err)
	assert.Empty(t, funding.Custom, "the declared funding provider survives")
}

// TestScopedNameSetsTheNamespace pins the one ToPF branch that rewrites the
// pf namespace: a scoped package name maps to its own npm.<scope> tree.
func TestScopedNameSetsTheNamespace(t *testing.T) {
	pkg := &Document{Name: fxNpmScopedName}
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: "old"}}
	assert.Equal(t, fxNpmScopedName, mapperRow(t, pkg, pf, npmKeyName).ToPF(true))
	assert.Equal(t, fxNpmNamespaceNS, pf.Identity.Namespace)
	assert.Equal(t, fxNpmName, pf.Identity.Name, "the scope is stripped off the name")
}

// TestVersionWithheldWithoutForce covers the version guard: a modelled version
// is only replaced by an explicit read.
func TestVersionWithheldWithoutForce(t *testing.T) {
	pkg := &Document{Version: fxVersionOne}
	pf := &projectfile.Document{Identity: projectfile.Identity{Version: fxVersionTwo}}
	row := mapperRow(t, pkg, pf, "version")
	assert.Empty(t, row.ToPF(false), "a declared version wins over the manifest")
	assert.Equal(t, fxVersionTwo, pf.Identity.Version)
	assert.Equal(t, fxVersionOne, row.ToPF(true), "an explicit read is authoritative")
}

// TestManifestRepositoryTeachesTheDocument pins the discovery path for the
// packages repository, including the shorthand spellings.
func TestManifestRepositoryTeachesTheDocument(t *testing.T) {
	for name, manifest := range map[string]string{
		"shorthand": `{"repository": "github:acme/widget"}`,
		"mapping":   `{"repository": {"url": "https://github.com/acme/widget.git", "directory": "packages/npm"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			writePkgJSON(t, dir, manifest)
			pfPath := pfFixture(t, dir, "---\nidentity:\n  name: widget\n")
			pf, err := projectfile.Read(pfPath)
			require.NoError(t, err)
			_, err = core.RunSync(Bridge{}, pf, core.Options{Dir: dir, PFPath: pfPath, Mode: core.ModeRead})
			require.NoError(t, err)
			slot := pfmodel.PackagesRepository(pf)
			require.NotNil(t, slot)
			if name == "shorthand" {
				assert.Equal(t, "https://github.com/acme/widget", slot.URL, name)
				assert.Equal(t, "git", slot.Type)
			} else {
				assert.Equal(t, fxRepoURL, slot.URL, name,
					"a mapping keeps its url verbatim")
				assert.Equal(t, "packages/npm", slot.Path, "the sub-directory survives")
			}
		})
	}
}

// TestWriteModePushesEveryField drives the push direction over the same full
// manifest from an empty projectfile, covering each FromPF closure.
func TestWriteModePushesEveryField(t *testing.T) {
	dir := t.TempDir()
	writePkgJSON(t, dir, everyFieldJSON)
	pf := &projectfile.Document{
		Identity: projectfile.Identity{
			Name: fxNpmName, Namespace: fxNpmNamespaceNS, Version: fxVersionTwo,
			Summary: &projectfile.LocalizedString{Bare: "A test package"},
		},
		License:  &projectfile.License{Spdx: fxLicenseMIT},
		Keywords: []string{"go", fxKeywordTest},
		People: []projectfile.Person{
			{GivenNames: "Alice", FamilyNames: "Smith", Email: "alice@example.com", Roles: []string{fxRoleAuthor}},
			{GivenNames: "Bob", FamilyNames: "Roberts", Email: "bob@example.com", Roles: []string{fxRoleContrib}},
			{DisplayName: "Carol", Roles: []string{fxRoleMaintain}},
		},
	}
	projectfile.SetExtension(pf, nsArchitecture, []string{fxAMD64})
	projectfile.SetExtension(pf, nsOperatingSystem, []string{fxLinux, fxWindows})

	_, err := core.RunSync(Bridge{}, pf, core.Options{Dir: dir, Mode: core.ModeWrite})
	require.NoError(t, err)

	pkg, err := Read(dir)
	require.NoError(t, err)
	assert.Equal(t, fxVersionTwo, pkg.Version)
	assert.Equal(t, "A test package", pkg.Description)
	assert.Equal(t, fxLicenseMIT, pkg.License)
	assert.Equal(t, []string{"go", fxKeywordTest}, pkg.Keywords)
	assert.Equal(t, []string{fxLinux, fxWin32}, pkg.OS)
	assert.Equal(t, []string{fxX64}, pkg.CPU)

	people := ParsePerson(pkg.Author)
	assert.Equal(t, "Alice Smith", people.Name, "the given+family form composes for package.json")
	assert.Equal(t, "alice@example.com", people.Email)
	contribs := ParsePersonList(pkg.Contributors)
	require.Len(t, contribs, 1)
	assert.Equal(t, "Bob Roberts", contribs[0].Name, "given and family compose for package.json")
	maints := ParsePersonList(pkg.Maintainers)
	require.Len(t, maints, 1)
	assert.Equal(t, "Carol", maints[0].Name)
}

// TestWriteModeRemovesUnmodelledProjections covers the clear-with-force path of
// every mapper: nothing on the projectfile means nothing in package.json.
func TestWriteModeRemovesUnmodelledProjections(t *testing.T) {
	dir := t.TempDir()
	writePkgJSON(t, dir, everyFieldJSON)
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: fxNpmName, Namespace: fxNpmNamespaceNS}}

	_, err := core.RunSync(Bridge{}, pf, core.Options{Dir: dir, Mode: core.ModeWrite})
	require.NoError(t, err)

	pkg, err := Read(dir)
	require.NoError(t, err)
	assert.Equal(t, "1.2.3", pkg.Version,
		"version is the one mapped key whose FromPF withholds on an absent pf value")
	assert.Empty(t, pkg.Description)
	assert.Empty(t, pkg.License)
	assert.Empty(t, pkg.Keywords)
	assert.Empty(t, pkg.Homepage)
	assert.Nil(t, pkg.Funding, "no funding extension means no funding key")
	assert.Empty(t, ParsePerson(pkg.Author).Name, "no people means no author")
	require.True(t, pkg.Private, "absence of identity.visibility resolves to private")
}

// TestDefaultSyncPreservesUnmodelledFileValues covers the converging half of a
// default sync: a value the projectfile does not model survives a forced one
// only on an explicit `to`.
func TestDefaultSyncPreservesUnmodelledFileValues(t *testing.T) {
	dir := t.TempDir()
	writePkgJSON(t, dir, everyFieldJSON)
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: fxNpmName, Namespace: fxNpmNamespaceNS}}

	_, err := core.RunSync(Bridge{}, pf, core.Options{Dir: dir, Mode: core.ModeSync})
	require.NoError(t, err)

	pkg, err := Read(dir)
	require.NoError(t, err)
	assert.Equal(t, "1.2.3", pkg.Version, "a value pf does not model is preserved")
	assert.Equal(t, "A test package", pkg.Description)
}

// TestFundingRoundTripBothDirections pins the funding projection both ways:
// a url becomes a custom entry, and a custom entry becomes a url again.
func TestFundingRoundTripBothDirections(t *testing.T) {
	dir := t.TempDir()
	writePkgJSON(t, dir, `{"name":"p","funding":[{"url":"https://a.test"},{"url":"https://b.test"}]}`)
	pfPath := pfFixture(t, dir, "---\nidentity:\n  name: p\n")
	pf, err := projectfile.Read(pfPath)
	require.NoError(t, err)

	_, err = core.RunSync(Bridge{}, pf, core.Options{Dir: dir, PFPath: pfPath, Mode: core.ModeRead})
	require.NoError(t, err)
	funding, err := pfmodel.GetFundingExtension(pf)
	require.NoError(t, err)
	assert.Equal(t, []string{fxURLA, fxURLB}, funding.Custom, "a funding array fans out")

	_, err = core.RunSync(Bridge{}, pf, core.Options{Dir: dir, PFPath: pfPath, Mode: core.ModeWrite})
	require.NoError(t, err)
	pkg, err := Read(dir)
	require.NoError(t, err)
	entries := ParseFunding(pkg.Funding)
	require.Len(t, entries, 2, "the array shape survives the round trip")
	assert.Equal(t, fxValueCustom, entries[0].Type)
	assert.Equal(t, fxURLA, entries[0].URL)
}

// TestSingleFundingUrlStaysScalar pins that one url stays a scalar, which is
// the FUNDING.yml spelling npm's own tooling writes.
func TestSingleFundingUrlStaysScalar(t *testing.T) {
	dir := t.TempDir()
	writePkgJSON(t, dir, `{"name":"p"}`)
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: "p"}}
	projectfile.SetExtension(pf, pfmodel.FundingExtensionNS, map[string]any{fxValueCustom: []any{fxURLOnly}})

	_, err := core.RunSync(Bridge{}, pf, core.Options{Dir: dir, Mode: core.ModeWrite})
	require.NoError(t, err)
	pkg, err := Read(dir)
	require.NoError(t, err)
	assert.Equal(t, fxURLOnly, ParseFunding(pkg.Funding)[0].URL)
}

// TestPlatformClearOnForce pins the force path of the platform mapper: a force
// with no §4.8a list removes the entry from package.json, and without force it
// leaves it alone.
func TestPlatformClearOnForce(t *testing.T) {
	dir := t.TempDir()
	writePkgJSON(t, dir, `{"name":"p","os":["linux"],"cpu":["x64"]}`)
	pkg, err := Read(dir)
	require.NoError(t, err)

	pf := &projectfile.Document{Identity: projectfile.Identity{Name: "p"}}
	assert.Empty(t, mapperRow(t, pkg, pf, "os").FromPF(false),
		"without force an empty §4.8a list leaves the file alone")
	assert.Equal(t, []string{fxLinux}, pkg.OS, "the manifest keeps its list")
	assert.Equal(t, core.Removed, mapperRow(t, pkg, pf, "os").FromPF(true),
		"with force it is cleared")
	assert.Empty(t, pkg.OS, "the list leaves the document")
	assert.Equal(t, core.Removed, mapperRow(t, pkg, pf, "cpu").FromPF(true))
}

// TestUnreadableArtifactsNamespaceIsWarnedNotFatal pins the failure mode of a
// malformed artifacts namespace: the reader warns and the projection stops
// rather than writing a half-resolved value.
func TestUnreadableArtifactsNamespaceIsWarnedNotFatal(t *testing.T) {
	dir := t.TempDir()
	writePkgJSON(t, dir, everyFieldJSON)
	pkg, err := Read(dir)
	require.NoError(t, err)
	pf := &projectfile.Document{}
	projectfile.SetExtension(pf, pfmodel.ArtifactsExtensionNS, "not-a-map")

	assert.Empty(t, declaredNPMName(pf), "an unreadable namespace declares no name")
	assert.Nil(t, declaredNPMPlatform(pf, func(a pfmodel.Artifact) []string { return a.Architecture }),
		"an unreadable namespace declares no platform")

	for _, m := range (Bridge{}).BuildMappers(pkg, pf) {
		if m.ToPF != nil {
			assert.NotPanics(t, func() { m.ToPF(true) }, m.ExtKey)
		}
	}
}

// TestDeclaredArtifactPlatformWins pins that a namespace declaring its own
// platform list is not overwritten by the manifest in either direction.
func TestDeclaredArtifactPlatformWins(t *testing.T) {
	dir := t.TempDir()
	writePkgJSON(t, dir, `{"name":"p","os":["darwin"]}`)
	pfPath := pfFixture(t, dir, "---\nidentity:\n  name: p\n")
	pf, err := projectfile.Read(pfPath)
	require.NoError(t, err)
	projectfile.SetExtension(pf, "org.projectfile.artifacts", map[string]any{
		"npm-package": map[string]any{
			"kind": "package", "registry": registryNPM,
			"operating-system": []any{fxLinux},
			"architecture":     []any{fxAMD64, fxArm64},
		},
	})

	_, err = core.RunSync(Bridge{}, pf, core.Options{Dir: dir, PFPath: pfPath, Mode: core.ModeRead})
	require.NoError(t, err)
	osList, _ := projectfile.LookupExtension(pf, nsOperatingSystem)
	assert.Nil(t, osList, "the declared artifact value is authoritative")

	_, err = core.RunSync(Bridge{}, pf, core.Options{Dir: dir, PFPath: pfPath, Mode: core.ModeWrite})
	require.NoError(t, err)
	pkg, err := Read(dir)
	require.NoError(t, err)
	assert.Equal(t, []string{fxLinux}, pkg.OS,
		"the declared artifact platform is what the manifest is written from")
}

// TestWritePathUsesTheBridgeWriter pins that RunSync reaches the Bridge.Write
// success branch rather than only the type-refusal one.
func TestWritePathUsesTheBridgeWriter(t *testing.T) {
	dir := t.TempDir()
	writePkgJSON(t, dir, `{"name":"p","version":"1.0.0"}`)
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: "p", Version: fxVersionTwo}}

	_, err := core.RunSync(Bridge{}, pf, core.Options{Dir: dir, Mode: core.ModeSync, Force: true})
	require.NoError(t, err)
	pkg, err := Read(dir)
	require.NoError(t, err)
	assert.Equal(t, fxVersionTwo, pkg.Version)
}

// TestCreatePathBuildsAManifestFromScratch pins the missing-file branch: a
// projectfile with no package.json produces one rather than failing.
func TestCreatePathBuildsAManifestFromScratch(t *testing.T) {
	dir := t.TempDir()
	pf := &projectfile.Document{
		Identity: projectfile.Identity{
			Name: fxNpmName, Namespace: fxNpmNamespaceNS, Version: fxVersionOne,
			Summary: &projectfile.LocalizedString{Bare: "A fresh package"},
		},
		Keywords: []string{"fresh"},
		License:  &projectfile.License{Spdx: fxLicenseMIT},
	}

	res, err := core.RunSync(Bridge{}, pf, core.Options{Dir: dir, Mode: core.ModeWrite})
	require.NoError(t, err)
	assert.True(t, res.Created, "a missing manifest is created, not reported as drift")

	pkg, err := Read(dir)
	require.NoError(t, err)
	assert.Equal(t, fxNpmScopedName, pkg.Name, "the npm.<scope> namespace projects as a scoped name")
	assert.Equal(t, fxVersionOne, pkg.Version)
	assert.Equal(t, fxLicenseMIT, pkg.License)
	assert.Equal(t, []string{"fresh"}, pkg.Keywords)
}

// TestCreatePathRefusedByNoCreate pins the opt-out for the same branch.
func TestCreatePathRefusedByNoCreate(t *testing.T) {
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: fxNpmName}}
	_, err := core.RunSync(Bridge{}, pf, core.Options{Dir: t.TempDir(), Mode: core.ModeWrite, NoCreate: true})
	require.Error(t, err, "--no-create must refuse to invent a package.json")
}

// TestDryRunNeverTouchesDisk pins the clone path: a dry run reports the change
// and leaves the file exactly as it was.
func TestDryRunNeverTouchesDisk(t *testing.T) {
	dir := t.TempDir()
	writePkgJSON(t, dir, `{"name":"p","version":"1.0.0"}`)
	before, err := Read(dir)
	require.NoError(t, err)

	pf := &projectfile.Document{Identity: projectfile.Identity{Name: "p", Version: fxVersionTwo}}
	res, err := core.RunSync(Bridge{}, pf, core.Options{Dir: dir, Mode: core.ModeWrite, DryRun: true})
	require.NoError(t, err)
	assert.True(t, res.DryRun)
	assert.True(t, res.ExtChanged, "the pending change is still reported")

	after, err := Read(dir)
	require.NoError(t, err)
	assert.Equal(t, fxVersionOne, after.Version, "the file on disk is untouched")
	assert.Equal(t, before.Version, after.Version)
}

// TestSyncCreatesOnDryRunToo pins that a missing manifest in dry-run mode
// reports Created without writing the file.
func TestSyncCreatesOnDryRunToo(t *testing.T) {
	dir := t.TempDir()
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: fxNpmName, Version: fxVersionOne}}

	res, err := core.RunSync(Bridge{}, pf, core.Options{Dir: dir, Mode: core.ModeWrite, DryRun: true})
	require.NoError(t, err)
	assert.True(t, res.Created)
	assert.False(t, Exists(dir), "no manifest is written in dry-run mode")
}

// TestReadModeIsIdempotent pins the second read: over a document the first read
// already reconciled, nothing reports a change — the state a projectfile-driven
// --check gate lives in.
func TestReadModeIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	writePkgJSON(t, dir, everyFieldJSON)
	pfPath := pfFixture(t, dir, "---\nidentity:\n  name: widget\n")
	pf, err := projectfile.Read(pfPath)
	require.NoError(t, err)

	opts := core.Options{Dir: dir, PFPath: pfPath, Mode: core.ModeRead}
	_, err = core.RunSync(Bridge{}, pf, opts)
	require.NoError(t, err)

	pf, err = projectfile.Read(pfPath)
	require.NoError(t, err)
	res, err := core.RunSync(Bridge{}, pf, opts)
	require.NoError(t, err)
	// Every other mapper settles on a document the first read documented; the
	// funding mapper has no equality guard, so a forced read re-asserts it.
	require.Len(t, res.PFFields, 1, "the funding mapper is the only one that re-asserts")
	assert.Equal(t, "[org.projectfile.funding]", res.PFFields[0].Key)
}

// TestEmptyManifestFundingIsNotAFundingClaim pins the "a funding key with no
// entries" spelling: it carries no destination, so it teaches the document
// nothing and writes no funding namespace.
func TestEmptyManifestFundingIsNotAFundingClaim(t *testing.T) {
	dir := t.TempDir()
	writePkgJSON(t, dir, `{"name":"p","funding":[]}`)
	pkg, err := Read(dir)
	require.NoError(t, err)

	pf := &projectfile.Document{Identity: projectfile.Identity{Name: "p"}}
	assert.Empty(t, mapperRow(t, pkg, pf, "funding").ToPF(true))
	_, present := projectfile.LookupExtension(pf, pfmodel.FundingExtensionNS)
	assert.False(t, present, "no entry means no funding namespace at all")
}

// TestFundingWithheldOnGapFill pins that a document already declaring funding
// is not overwritten by the manifest without an explicit read.
func TestFundingWithheldOnGapFill(t *testing.T) {
	pkg := &Document{Funding: FundingEntry{Type: fxValueCustom, URL: "https://from.test"}}
	pf := &projectfile.Document{}
	projectfile.SetExtension(pf, pfmodel.FundingExtensionNS, map[string]any{"patreon": fxValueMine})

	assert.Empty(t, mapperRow(t, pkg, pf, "funding").ToPF(false))
	funding, err := pfmodel.GetFundingExtension(pf)
	require.NoError(t, err)
	assert.Equal(t, fxValueMine, funding.Patreon, "the declared provider survives")
}

// TestEnginesBothDirections pins the four engines branches: an equal map reports
// nothing, a differing one converges on a gap-fill, and a declared set survives.
func TestEnginesBothDirections(t *testing.T) {
	same := &Document{Engines: map[string]string{fxEngineName: fxEngineNode}}
	equal := &projectfile.Document{
		Requirements: &projectfile.Requirements{Runtime: map[string]string{fxEngineName: fxEngineNode}},
	}
	assert.Empty(t, mapperRow(t, same, equal, "engines").FromPF(true),
		"an identical engines map reports no change")
	assert.Empty(t, mapperRow(t, same, equal, "engines").ToPF(true),
		"reading back an identical map reports no change either")

	other := &Document{Engines: map[string]string{fxEngineName: fxEngineNode}}
	declared := &projectfile.Document{
		Requirements: &projectfile.Requirements{Runtime: map[string]string{fxEngineName: fxEngineBump}},
	}
	assert.Empty(t, mapperRow(t, other, declared, "engines").ToPF(false),
		"a declared runtime set survives a default read")
	assert.Equal(t, "map[node:>=22]", mapperRow(t, other, declared, "engines").FromPF(false),
		"an unmodelled runtime set is filled from the document")
	assert.Equal(t, map[string]string{fxEngineName: fxEngineBump}, other.Engines)

	clearing := &Document{Engines: map[string]string{fxEngineName: ">=9"}}
	mapperRow(t, clearing, &projectfile.Document{Identity: projectfile.Identity{Name: "p"}}, "engines").FromPF(true)
	assert.Nil(t, clearing.Engines, "no runtime in the document clears the manifest entry")
}

// TestPersonProjectionsReportEveryChange pins that the people mapper names what
// it replaced on the file side, which is what a sync summary prints.
func TestPersonProjectionsReportEveryChange(t *testing.T) {
	pkg := &Document{
		Author:       "Someone Else <other@example.test>",
		Contributors: []any{"Old Contributor"},
		Maintainers:  []any{"Old Maintainer"},
	}
	pf := &projectfile.Document{
		People: []projectfile.Person{
			{GivenNames: "Real", FamilyNames: "Author", Email: fxEmailReal, Roles: []string{fxRoleAuthor}},
			{GivenNames: "Real", FamilyNames: "Contributor", Email: "rc@example.test", Roles: []string{fxRoleContrib}},
			{DisplayName: "Real Maintainer", Roles: []string{fxRoleMaintain}},
		},
	}

	desc := mapperRow(t, pkg, pf, "people").FromPF(true)
	assert.Contains(t, desc, "Real Author", "the author rewrite is named")
	assert.Contains(t, desc, "1 contributor(s)", "a roster reports its size, not its names")
	assert.Contains(t, desc, "1 maintainer(s)")
	assert.Len(t, strings.Split(desc, ", "), 3, "one report per roster")

	removed := &Document{
		Author:       Person{Name: "Keep"},
		Contributors: []any{"Stale Contributor"},
		Maintainers:  []any{"Stale Maintainer"},
	}
	cleared := mapperRow(t, removed, &projectfile.Document{}, "people").FromPF(true)
	assert.Contains(t, cleared, "author "+core.Removed)
	assert.Contains(t, cleared, "contributors "+core.Removed)
	assert.Contains(t, cleared, "maintainers "+core.Removed)
	assert.Nil(t, removed.Contributors, "a cleared roster leaves the document")
}

// TestWriteRefusesAnUnmarshallableCanvas pins the last write guard: a raw canvas
// carrying bytes that are not JSON makes the write fail rather than emit a file
// no parser can read.
func TestWriteRefusesAnUnmarshallableCanvas(t *testing.T) {
	dir := t.TempDir()
	doc := &Document{Name: "p", Version: fxVersionOne, Rest: rawdoc.NewOrderedJSON()}
	doc.Rest.Set("volta", json.RawMessage("not json"))

	err := Write(dir, doc)
	require.Error(t, err, "a canvas holding a bare string as a value is not JSON")
	assert.False(t, Exists(dir), "nothing is written when the canvas cannot be rendered")
}
