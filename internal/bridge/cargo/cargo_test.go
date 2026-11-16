// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package cargo

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/v4/internal/bridge/core"
)

// packageManifest is a single-crate manifest with comments, odd quoting and a multi-line array.
const (
	keywordCLI = "cli"
	crateDemo  = "demo"
	mitHeader  = "# SPDX-License-" + "Identifier: MIT\n\n"
)

const packageManifest = `# SPDX-FileCopyrightText: 2026 Alice Example <alice@example.com>
#
# SPDX-License-Identifier: MIT

[package]
name = "demo" # crate name
version = '1.2.3'
edition = "2024"
description = "Brackets ] and # hashes stay inside strings"
keywords = [
    "cli",  # first
    "demo",
]
license = "MIT"
# trailing note about the package table

[dependencies]
serde = { version = "1", features = ["derive"] }

[package.metadata.docs.rs]
all-features = true
`

// workspaceManifest is a virtual workspace root carrying the shared package fields.
const workspaceManifest = `[workspace]
members = ["crates/*"]
resolver = "3"

[workspace.package]
version = "0.0.0"
edition = "2024"
license = "AGPL-3.0-only"

[workspace.dependencies]
log = "=0.4.35"
`

// memberManifest is a workspace member inheriting from a manifest elsewhere.
const memberManifest = `[package]
name = "member"
version.workspace = true
license = { workspace = true }
edition = "2024"
`

// rootWithWorkspace is a root crate that is also its own workspace.
const rootWithWorkspace = `[package]
name = "root"
version = { workspace = true }

[workspace]
members = ["sub"]
`

// writeManifest stores content as the manifest of a fresh temp dir.
func writeManifest(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(FullPath(dir), []byte(content), 0o644))
	return dir
}

// readManifest returns the manifest text under dir.
func readManifest(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(FullPath(dir))
	require.NoError(t, err)
	return string(data)
}

// renderString renders doc and fails the test on error.
func renderString(t *testing.T, doc *Document) string {
	t.Helper()
	out, err := render(doc)
	require.NoError(t, err)
	return string(out)
}

func TestReadPackageFields(t *testing.T) {
	doc, err := Read(writeManifest(t, packageManifest))
	require.NoError(t, err)
	assert.Equal(t, crateDemo, doc.Name)
	assert.Equal(t, "1.2.3", doc.Version)
	assert.Equal(t, "Brackets ] and # hashes stay inside strings", doc.Description)
	assert.Equal(t, []string{keywordCLI, crateDemo}, doc.Keywords)
	assert.Equal(t, "MIT", doc.License)
	assert.Empty(t, doc.Foreign)
}

func TestReadVirtualWorkspaceReadsWorkspacePackage(t *testing.T) {
	doc, err := Read(writeManifest(t, workspaceManifest))
	require.NoError(t, err)
	assert.Equal(t, "0.0.0", doc.Version)
	assert.Equal(t, "AGPL-3.0-only", doc.License)
	assert.True(t, doc.Foreign[keyName], "a workspace cannot carry a crate name")
	assert.True(t, doc.Owns(keyDescription), "a workspace owns every inheritable key")
}

func TestReadMemberMarksInheritedKeysForeign(t *testing.T) {
	doc, err := Read(writeManifest(t, memberManifest))
	require.NoError(t, err)
	assert.Equal(t, "member", doc.Name)
	assert.Empty(t, doc.Version)
	assert.True(t, doc.Foreign[keyVersion], "dotted version.workspace is inherited")
	assert.True(t, doc.Foreign[keyLicense], "inline { workspace = true } is inherited")
	assert.True(t, doc.Owns(keyDescription))
}

func TestReadRejectsMalformedManifest(t *testing.T) {
	_, err := Read(writeManifest(t, "[package\nname = 1\n"))
	require.Error(t, err)
	_, err = Read(t.TempDir())
	require.Error(t, err)
}

func TestRenderUnchangedIsByteIdentical(t *testing.T) {
	for name, src := range map[string]string{"package": packageManifest, "workspace": workspaceManifest, "member": memberManifest} {
		t.Run(name, func(t *testing.T) {
			doc, err := Read(writeManifest(t, src))
			require.NoError(t, err)
			assert.Equal(t, src, renderString(t, doc))
		})
	}
}

func TestRenderRewritesOnlyChangedLines(t *testing.T) {
	doc, err := Read(writeManifest(t, packageManifest))
	require.NoError(t, err)
	doc.Version = "2.0.0"
	doc.Keywords = []string{keywordCLI}
	want := `# SPDX-FileCopyrightText: 2026 Alice Example <alice@example.com>
#
# SPDX-License-Identifier: MIT

[package]
name = "demo" # crate name
version = "2.0.0"
edition = "2024"
description = "Brackets ] and # hashes stay inside strings"
keywords = ["cli"]
license = "MIT"
# trailing note about the package table

[dependencies]
serde = { version = "1", features = ["derive"] }

[package.metadata.docs.rs]
all-features = true
`
	assert.Equal(t, want, renderString(t, doc))
}

func TestRenderAddsKeyAfterLastAssignment(t *testing.T) {
	doc, err := Read(writeManifest(t, packageManifest))
	require.NoError(t, err)
	doc.Homepage = "https://example.com"
	assert.Contains(t, renderString(t, doc), "license = \"MIT\"\nhomepage = \"https://example.com\"\n# trailing note")
}

func TestRenderRemovesClearedKey(t *testing.T) {
	doc, err := Read(writeManifest(t, packageManifest))
	require.NoError(t, err)
	doc.Keywords = nil
	doc.Description = ""
	got := renderString(t, doc)
	assert.NotContains(t, got, "keywords")
	assert.NotContains(t, got, "description")
	assert.Contains(t, got, "edition = \"2024\"\nlicense = \"MIT\"\n")
}

func TestRenderLeavesInheritedKeysAlone(t *testing.T) {
	doc, err := Read(writeManifest(t, memberManifest))
	require.NoError(t, err)
	doc.Version = "9.9.9"
	doc.Description = "Member crate"
	got := renderString(t, doc)
	assert.Contains(t, got, "version.workspace = true\n")
	assert.NotContains(t, got, "9.9.9")
	assert.Contains(t, got, "edition = \"2024\"\ndescription = \"Member crate\"\n")
}

func TestRenderRoutesInheritedKeyToInFileWorkspace(t *testing.T) {
	doc, err := Read(writeManifest(t, rootWithWorkspace))
	require.NoError(t, err)
	assert.True(t, doc.Owns(keyVersion), "the workspace defining it lives in this file")
	doc.Version = "0.4.0"
	want := `[package]
name = "root"
version = { workspace = true }

[workspace]
members = ["sub"]

[workspace.package]
version = "0.4.0"
`
	assert.Equal(t, want, renderString(t, doc))
}

func TestRenderCreatesWorkspacePackageBeforeNextTable(t *testing.T) {
	doc, err := Read(writeManifest(t, "[workspace]\nmembers = [\"a\"]\n\n[workspace.dependencies]\nlog = \"1\"\n"))
	require.NoError(t, err)
	doc.Version = "1.0.0"
	assert.Equal(t, "[workspace]\nmembers = [\"a\"]\n\n[workspace.package]\nversion = \"1.0.0\"\n\n[workspace.dependencies]\nlog = \"1\"\n", renderString(t, doc))
}

func TestRenderFreshManifestGetsHeaderAndPackageTable(t *testing.T) {
	doc := &Document{Name: "fresh", Version: "0.1.0", Authors: []string{"Alice <a@example.com>"}, ReuseHeader: mitHeader}
	want := mitHeader + "[package]\nname = \"fresh\"\nversion = \"0.1.0\"\nauthors = [\"Alice <a@example.com>\"]\n"
	assert.Equal(t, want, renderString(t, doc))
}

func TestRenderKeepsHandWrittenHeader(t *testing.T) {
	doc, err := Read(writeManifest(t, packageManifest))
	require.NoError(t, err)
	doc.ReuseHeader = "# SPDX-FileCopyrightText: someone else\n"
	assert.NotContains(t, renderString(t, doc), "someone else")
}

func TestScannerSpansMultiLineValues(t *testing.T) {
	lines := []string{`a = """`, `has ] and "quotes" inside`, `"""`, `b = ['x]', "y\"]"]`, `c = [`, `  ["n"],`, `]`, `d = 1`}
	assert.Equal(t, 2, valueEnd(lines, 0), "triple-quoted strings span lines")
	assert.Equal(t, 3, valueEnd(lines, 3), "brackets inside strings do not nest")
	assert.Equal(t, 6, valueEnd(lines, 4), "nested arrays close at depth zero")
	assert.Equal(t, 7, valueEnd(lines, 7))
	assert.Equal(t, 0, valueEnd([]string{"[t]"}, 0), "a line without = is its own span")
	assert.Equal(t, 1, valueEnd([]string{"a = [", "1"}, 0), "an unclosed value runs to the end")
}

func TestFindZoneSkipsBracketLinesInsideValues(t *testing.T) {
	lines := []string{"[package]", "x = [", "[1],", "]", "y = 2", "[next]"}
	assert.Equal(t, zone{header: 0, end: 5}, findZone(lines, tablePackage))
	assert.Equal(t, zone{header: -1, end: -1}, findZone(lines, tableWorkspace))
}

func TestHeaderName(t *testing.T) {
	assert.Equal(t, "package", headerName("[package] # c"))
	assert.Equal(t, "workspace.package", headerName("  [ workspace.package ]"))
	assert.Empty(t, headerName("[[bin]]"))
	assert.Empty(t, headerName("name = 1"))
	assert.Equal(t, `x = "#" `, stripComment(`x = "#" # c`))
	assert.Equal(t, `x = '#' `, stripComment(`x = '#' # c`))
	assert.Equal(t, `x = "\"#" `, stripComment(`x = "\"#" # c`))
}

func TestQuoteEscapesForTOML(t *testing.T) {
	assert.Equal(t, `"a\"b\\c\n\t\u0001\u007Fé"`, quote("a\"b\\c\n\t\x01\x7fé"))
	assert.Equal(t, `["a", "b"]`, quoteList([]string{"a", "b"}))
}

func TestSyncWritesProjectfileValuesIntoWorkspace(t *testing.T) {
	dir := writeManifest(t, workspaceManifest)
	pf := &projectfile.Document{}
	pf.Identity.Name = "sl"
	pf.Identity.Version = "0.3.0"
	pf.Keywords = []string{"linter", "topic:security"}
	pf.Repositories = []projectfile.Repository{{URL: "ssh://git@example.com/o/r.git", Role: projectfile.RepositoryRoleOrigin}}
	pf.Links = []projectfile.Link{{Type: projectfile.LinkSourceCode, URL: "https://example.com/o/r"}}

	res, err := core.RunSync(Bridge{}, pf, core.Options{Dir: dir, Mode: core.ModeSync})
	require.NoError(t, err)
	assert.True(t, res.ExtChanged)
	got := readManifest(t, dir)
	assert.Contains(t, got, "[workspace.package]\nversion = \"0.3.0\"\n")
	assert.Contains(t, got, "keywords = [\"linter\"]\n")
	assert.Contains(t, got, "repository = \"https://example.com/o/r\"\n", "an SSH clone URL resolves to the forge page")
	assert.Contains(t, got, "license = \"AGPL-3.0-only\"", "a default sync keeps what the projectfile lacks")
	assert.NotContains(t, got, "name =", "a workspace never gets a crate name")

	res, err = core.RunSync(Bridge{}, pf, core.Options{Dir: dir, Mode: core.ModeSync})
	require.NoError(t, err)
	assert.False(t, res.ExtChanged, "a second sync converges")
}

func TestReadDirectionFillsProjectfile(t *testing.T) {
	extra := "license = \"MIT\"\nrust-version = \"1.85\"\nhomepage = \"https://example.com\"\nauthors = [\"Alice Example <alice@example.com>\"]\n"
	doc, err := Read(writeManifest(t, strings.Replace(packageManifest, "license = \"MIT\"\n", extra, 1)))
	require.NoError(t, err)
	pf := &projectfile.Document{}
	for _, m := range (Bridge{}).BuildMappers(doc, pf) {
		m.ToPF(true)
	}
	assert.Equal(t, crateDemo, pf.Identity.Name)
	assert.Equal(t, "1.2.3", pf.Identity.Version)
	assert.Equal(t, "MIT", pf.License.Spdx)
	assert.Equal(t, "1.85", pf.Requirements.Runtime[rustRequirementKey])
	assert.Equal(t, []string{keywordCLI, crateDemo}, pf.Keywords)
	assert.Len(t, pf.People, 1)
	for _, m := range (Bridge{}).BuildMappers(doc, pf) {
		assert.Empty(t, m.ToPF(false), "%s: a second pass changes nothing", m.ExtKey)
	}
}

func TestWriteDirectionClearsAndCloneIsIndependent(t *testing.T) {
	dir := writeManifest(t, packageManifest)
	pf := &projectfile.Document{}
	pf.Identity.Name = crateDemo
	_, err := core.RunSync(Bridge{}, pf, core.Options{Dir: dir, Mode: core.ModeWrite})
	require.NoError(t, err)
	got := readManifest(t, dir)
	assert.NotContains(t, got, "keywords")
	assert.NotContains(t, got, "license =")
	assert.Contains(t, got, "version = '1.2.3'", "the package identity is the file's even on to")

	doc := &Document{Keywords: []string{"a"}, Foreign: map[string]bool{keyVersion: true}}
	cp := (Bridge{}).Clone(doc).(*Document)
	cp.Keywords[0] = "b"
	cp.Foreign[keyName] = true
	assert.Equal(t, []string{"a"}, doc.Keywords)
	assert.False(t, doc.Foreign[keyName])
	var none *Document
	assert.Nil(t, none.Clone())
}
