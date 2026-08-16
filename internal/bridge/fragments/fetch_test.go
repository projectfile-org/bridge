// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package fragments

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// spdxTag is split from its value so the REUSE scanner does not read the
// fixtures assembled below as this file's own licence expression.
const (
	spdxTag = "SPDX-License-Identifier"
	spdxMIT = "MIT"
)

// The parent every fixture inherits from: its repository path, the name it
// gives itself, and the release the copy was read at.
const (
	testParent      = "b19/ubuntu"
	testParentTitle = "B19/Ubuntu"
	testParentURL   = "https://kiota.ch/b19/ubuntu"
	testRef         = "1.0.0"
)

func upstreamDoc(bodyLines ...string) string {
	return "<!--\nSPDX-FileCopyrightText: 2026 Upstream\n" + spdxTag + ": " + spdxMIT + "\n-->\n\n" +
		"# Features\n\n## Project Features\n\n" + strings.Join(bodyLines, "\n") + "\n"
}

// TestSplitSPDXKeepsUpstreamHeader verifies the parent's REUSE header comes
// back whole: it licenses the text being vendored, so the cache file keeps it.
func TestSplitSPDXKeepsUpstreamHeader(t *testing.T) {
	header, body := splitSPDX(upstreamDoc("### One", "", "Body."))
	assert.Contains(t, header, spdxTag+": "+spdxMIT)
	assert.True(t, strings.HasPrefix(header, "<!--"))
	assert.True(t, strings.HasPrefix(body, "# Features"))
}

// TestSplitSPDXReportsMissingHeader verifies a document with no REUSE header
// yields an empty header, which is what makes the fetcher refuse to vendor it.
func TestSplitSPDXReportsMissingHeader(t *testing.T) {
	header, body := splitSPDX("# Features\n\n### One\n")
	assert.Empty(t, header)
	assert.Contains(t, body, "### One")
}

// TestNormalizeInheritedDropsH1AndH2 verifies the parent's document headings go
// and its H3 entries survive — the level the readme bridge scrapes.
func TestNormalizeInheritedDropsH1AndH2(t *testing.T) {
	_, body := splitSPDX(upstreamDoc("### Entry One", "", "Body one.", "",
		"## Inherited from b19/base 1.0.0", "", "### Entry Two", "", "Body two."))
	got := normalizeInherited(body)

	assert.NotContains(t, got, "# Features")
	assert.NotContains(t, got, "## Project Features")
	assert.NotContains(t, got, "## Inherited from b19/base")
	assert.Contains(t, got, "### Entry One")
	// The grandparent's entries arrive with the parent's, so one hop carries the
	// whole chain and nothing walks the ancestry.
	assert.Contains(t, got, "### Entry Two")
	assert.NotContains(t, got, "\n\n\n", "collapsed to at most one blank line (MD012)")
}

// TestNormalizeInheritedStripsTextlintWrap verifies the pair a parent's
// published localized document carries does not survive into the copy: the
// cache file re-wraps itself and assembly wraps the whole variant, so a pair
// that nested would double-wrap and mis-scope the disable.
func TestNormalizeInheritedStripsTextlintWrap(t *testing.T) {
	got := normalizeInherited("<!-- textlint-disable terminology,common-misspellings -->\n\n### Entrada\n\nCuerpo.\n\n<!-- textlint-enable -->\n")
	assert.NotContains(t, got, "textlint-")
	assert.Contains(t, got, "### Entrada")
}

// TestNormalizeInheritedKeepsFencedHashes is the regression that matters for
// documents with shell examples: a comment inside a fence starts with "# " too,
// and dropping those lines would silently rewrite the parent's code samples.
func TestNormalizeInheritedKeepsFencedHashes(t *testing.T) {
	_, body := splitSPDX(upstreamDoc("### Entry", "", "```sh", "# install the thing",
		"## not a heading either", "make install", "```"))
	got := normalizeInherited(body)

	assert.Contains(t, got, "# install the thing")
	assert.Contains(t, got, "## not a heading either")
	assert.Contains(t, got, "make install")
}

// TestRoundTripsProvenance verifies a rendered cache file parses back into the
// same copy — the provenance is what carries the version into the heading, so a
// write the reader cannot read would silently drop the version.
func TestRoundTripsProvenance(t *testing.T) {
	dir := t.TempDir()
	want := inheritedCopy{
		Name:     testParent,
		URL:      testParentURL,
		Ref:      testRef,
		Commit:   "db0ef9b031b82d578410a6f3ec14e51e971049f2",
		Document: "FEATURES.md",
		SPDX:     "<!--\nSPDX-FileCopyrightText: 2026 Upstream\n" + spdxTag + ": " + spdxMIT + "\n-->",
		Body:     "### Entry\n\nBody.",
	}
	path := filepath.Join(dir, "b19-ubuntu.md")
	require.NoError(t, os.WriteFile(path, renderInherited(want), 0o644))

	got, err := parseInherited(path)
	require.NoError(t, err)
	assert.Equal(t, want.Name, got.Name)
	assert.Equal(t, want.Ref, got.Ref)
	assert.Equal(t, want.Commit, got.Commit)
	assert.Equal(t, want.Document, got.Document)
	assert.Equal(t, want.Body, got.Body)
	assert.Contains(t, got.SPDX, spdxTag+": "+spdxMIT)
	assert.Equal(t, "## Inherited from b19/ubuntu 1.0.0", got.Heading())
}

// TestRoundTripsLocalizedProvenance verifies a translated cache file round-trips:
// the lang attr reaches the parsed copy, the on-disk textlint wrap protects the
// file's prose but never leaks into the assembled Body.
func TestRoundTripsLocalizedProvenance(t *testing.T) {
	dir := t.TempDir()
	want := inheritedCopy{
		Name:     testParent,
		URL:      testParentURL,
		Ref:      testRef,
		Commit:   "db0ef9b031b82d578410a6f3ec14e51e971049f2",
		Document: "docs/es/FEATURES.md",
		SPDX:     "<!--\nSPDX-FileCopyrightText: 2026 Upstream\n" + spdxTag + ": " + spdxMIT + "\n-->",
		Body:     "### Entrada\n\nCuerpo.",
		Lang:     "es",
	}
	path := filepath.Join(dir, "b19-ubuntu.md")
	require.NoError(t, os.WriteFile(path, renderInherited(want), 0o644))

	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(raw), "textlint-disable", "the on-disk copy is linted, so it carries the wrap")

	got, err := parseInherited(path)
	require.NoError(t, err)
	assert.Equal(t, want.Lang, got.Lang)
	assert.Equal(t, want.Document, got.Document)
	assert.Equal(t, want.Body, got.Body)
	assert.NotContains(t, got.Body, "textlint-", "the wrap is storage pragma, stripped on read")
}

// TestHeadingFallsBackToCommit verifies a parent that publishes no tags is
// identified by commit rather than by a branch name that would read like a
// version it never released.
func TestHeadingFallsBackToCommit(t *testing.T) {
	c := inheritedCopy{Name: testParent, Commit: "db0ef9b031b82d5"}
	assert.Equal(t, "## Inherited from b19/ubuntu db0ef9b", c.Heading())
}

// TestHeadingPrefersTheParentTitle verifies a heading names the parent the way
// the parent names itself. owner/repo is a path, and reads to a prose linter as
// a misspelling of the product it points at.
func TestHeadingPrefersTheParentTitle(t *testing.T) {
	c := inheritedCopy{Name: testParent, Title: testParentTitle, Ref: testRef}
	assert.Equal(t, "## Inherited from B19/Ubuntu 1.0.0", c.Heading())
}

// TestTitleSurvivesTheCachedCopy verifies the title reaches assembly through
// the provenance comment. Assembly reads local files only, so a title that did
// not round-trip would be lost on every run but the refresh that fetched it.
func TestTitleSurvivesTheCachedCopy(t *testing.T) {
	dir := t.TempDir()
	want := inheritedCopy{
		Name:     testParent,
		Title:    testParentTitle,
		URL:      testParentURL,
		Ref:      testRef,
		Document: "FEATURES.md",
		SPDX:     "<!--\nSPDX-FileCopyrightText: 2026 Upstream\n" + spdxTag + ": " + spdxMIT + "\n-->",
		Body:     "### Entry\n\nBody.",
	}
	path := filepath.Join(dir, "b19-ubuntu.md")
	require.NoError(t, os.WriteFile(path, renderInherited(want), 0o644))

	got, err := parseInherited(path)
	require.NoError(t, err)
	assert.Equal(t, want.Title, got.Title)
	assert.Equal(t, "## Inherited from B19/Ubuntu 1.0.0", got.Heading())
}

// TestIdentityTitleReadsEveryEncoding verifies the title is read from each
// encoding the spec allows, and that a localized title resolves to one string.
func TestIdentityTitleReadsEveryEncoding(t *testing.T) {
	for name, tc := range map[string]struct{ file, body, want string }{
		"yaml":       {pfYAML, "identity:\n  title: B19/Ubuntu\n", testParentTitle},
		"toml":       {pfTOML, "[identity]\ntitle = \"B19/Ubuntu\"\n", testParentTitle},
		"json":       {pfJSON, `{"identity":{"title":"B19/Ubuntu"}}`, testParentTitle},
		"localized":  {pfYAML, "identity:\n  title:\n    uk: Убунту\n    en: B19/Ubuntu\n", testParentTitle},
		"no title":   {pfYAML, "identity:\n  name: ubuntu\n", ""},
		"unparsable": {pfYAML, "identity: [", ""},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, identityTitle([]byte(tc.body), tc.file))
		})
	}
}

// TestLocalizedTitleIsDeterministic verifies a title published in several
// languages, none of them English, always resolves to the same one. The drift
// gate compares the assembled document byte for byte, so a title chosen by map
// iteration order would report drift at random.
func TestLocalizedTitleIsDeterministic(t *testing.T) {
	title := map[string]any{"uk": "Убунту", "es": "Ubuntu ES", "de": "Ubuntu DE"}
	for range 20 {
		assert.Equal(t, "Ubuntu DE", localizedTitle(title))
	}
}

// TestTarEntryPicksTheDocument verifies one file is read out of the archive
// stream by name, so a server that prefixes or pads the stream cannot hand back
// a different file than the one asked for.
func TestTarEntryPicksTheDocument(t *testing.T) {
	var buf bytes.Buffer
	writer := tar.NewWriter(&buf)
	for name, body := range map[string]string{"README.md": "wrong", "b19-ubuntu/FEATURES.md": "right"} {
		require.NoError(t, writer.WriteHeader(&tar.Header{
			Name: name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg,
		}))
		_, err := writer.Write([]byte(body))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())

	got, err := tarEntry(buf.Bytes(), "FEATURES.md")
	require.NoError(t, err)
	assert.Equal(t, "right", got)

	_, err = tarEntry(buf.Bytes(), "ROADMAP.md")
	assert.Error(t, err, "a document the parent does not publish must not resolve")
}

// TestValidateRepoURLRejectsExecutableForms is a security guard: the URL reaches
// a git subprocess, and remote helpers such as ext:: run a command of the URL
// author's choice — and a parent URL comes from a file another project writes.
func TestValidateRepoURLRejectsExecutableForms(t *testing.T) {
	for _, raw := range []string{"ext::sh -c id", "--upload-pack=id", "", "  "} {
		assert.Error(t, validateRepoURL(raw), raw)
	}
	// Every ordinary git url form stays accepted — these repositories are ssh.
	for _, raw := range []string{
		"ssh://git@kiota.ch/b19/ubuntu.git",
		"git@github.com:owner/repo.git",
		"https://codeberg.org/b19/ubuntu",
	} {
		assert.NoError(t, validateRepoURL(raw), raw)
	}
}

// TestOwnerRepoAndSlug verifies the parent name and its cache filename, for both
// url forms git accepts.
func TestOwnerRepoAndSlug(t *testing.T) {
	assert.Equal(t, "b19/ubuntu", ownerRepo("ssh://git@kiota.ch/b19/ubuntu.git"))
	assert.Equal(t, "b19/ubuntu", ownerRepo("https://codeberg.org/b19/ubuntu"))
	assert.Equal(t, "owner/repo", ownerRepo("git@github.com:owner/repo.git"))
	// A nested group keeps the project, not the group it sits in.
	assert.Equal(t, "sub/repo", ownerRepo("ssh://git@example.test/group/sub/repo.git"))
	assert.Equal(t, "b19-ubuntu", slug("b19/ubuntu"))
	assert.Equal(t, "damian-buho-b19-ubuntu", slug("damian-buho/B19-Ubuntu"))
}

// TestFirstRefTakesLeadingLine verifies ls-remote parsing picks the first ref,
// which under --sort=-v:refname is the newest release tag.
func TestFirstRefTakesLeadingLine(t *testing.T) {
	sha, name, ok := firstRef("\ndb0ef9b\trefs/tags/2.0.0\nabc1234\trefs/tags/1.0.0\n")
	require.True(t, ok)
	assert.Equal(t, "db0ef9b", sha)
	assert.Equal(t, "refs/tags/2.0.0", name)

	_, _, ok = firstRef("\n\n")
	assert.False(t, ok)
}
