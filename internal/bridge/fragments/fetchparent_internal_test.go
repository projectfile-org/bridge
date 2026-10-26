// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package fragments

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"projectfile.org/projectfile/bridge/v4/internal/bridge/core"
	"projectfile.org/projectfile/bridge/v4/internal/pfmodel"
)

// The whole parent fetch runs against a real local repository here: ls-remote
// resolves the version, `git archive --remote` hands over the documents, and
// the copy the bridge nests is reduced from bytes git actually served.

// TestFetchParentReadsThePublishedDocument is the happy path: the newest
// release tag resolves, its canonical document and the declared language's copy
// both reduce to the nesting shape, and the parent's own title names the
// section. A second read of the same parent reuses the memoized version.
func TestFetchParentReadsThePublishedDocument(t *testing.T) {
	url := gitFixture(t, map[string]string{
		outFeaturesName: upstreamDoc("### Live Entry", "", "Live body."),
		core.LocalizedFilename(outFeaturesName, "es"): upstreamDoc("### Entrada", "", "Cuerpo."),
		pfYAML: "identity:\n  title: " + testParentTitle + "\n",
	}, "v1.0.0", "v2.0.0")
	parent := pfmodel.FragmentParent{URL: url}

	cop, translated, err := fetchParent(context.Background(), parent, outFeaturesName, []string{"es"})
	require.NoError(t, err)
	assert.Equal(t, testParent, cop.Name)
	assert.Equal(t, testParentTitle, cop.Title)
	assert.Equal(t, "v2.0.0", cop.Ref, "the newest release tag wins")
	assert.Len(t, cop.Commit, 40)
	assert.Contains(t, cop.SPDX, spdxTag+": "+spdxMIT)
	assert.Contains(t, cop.Body, "### Live Entry")
	assert.NotContains(t, cop.Body, "# Features")
	assert.Equal(t, "## Inherited from B19/Ubuntu", cop.Heading())
	require.Contains(t, translated, "es")
	assert.Equal(t, "es", translated["es"].Lang)
	assert.Equal(t, core.LocalizedFilename(outFeaturesName, "es"), translated["es"].Document)
	assert.Contains(t, translated["es"].Body, "### Entrada")
	assert.Equal(t, testParentTitle, translated["es"].Title, "provenance rides along")

	again, _, err := fetchParent(context.Background(), parent, outFeaturesName, []string{"es"})
	require.NoError(t, err)
	assert.Equal(t, cop.Ref, again.Ref, "the resolved version is memoized per repository")
	assert.Equal(t, cop.Commit, again.Commit)
	assert.Equal(t, cop.Title, again.Title)
}

// TestFetchParentFallsBackToTheRootBasename: a parent that keeps the document
// at its repository root publishes it under the bare basename, so the nested
// path read misses and the root read answers.
func TestFetchParentFallsBackToTheRootBasename(t *testing.T) {
	url := gitFixture(t, map[string]string{
		outFeaturesName: upstreamDoc("### Root Entry", "", "Root body."),
	}, "v1.0.0")
	nested := "docs/" + outFeaturesName

	cop, translated, err := fetchParent(context.Background(), pfmodel.FragmentParent{URL: url}, nested, nil)
	require.NoError(t, err)
	assert.Equal(t, nested, cop.Document, "the declared path stays the copy's provenance")
	assert.Contains(t, cop.Body, "### Root Entry")
	assert.Empty(t, translated)
}

// TestFetchParentRefusesTextWithNoLicence pins the one refusal: vendored text
// carries no licence statement, which would put an unlicensed file in this
// repository over a fault upstream owns.
func TestFetchParentRefusesTextWithNoLicence(t *testing.T) {
	url := gitFixture(t, map[string]string{outFeaturesName: "# Features\n\n### Entry\n\nBody.\n"}, "v1.0.0")

	_, _, err := fetchParent(context.Background(), pfmodel.FragmentParent{URL: url}, outFeaturesName, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "carries no SPDX header")
}

// TestFetchParentRefusesAnExecutableURL is the security guard at the entry of
// the crossing: a parent URL that would make git run a command is refused
// before any subprocess sees it.
func TestFetchParentRefusesAnExecutableURL(t *testing.T) {
	_, _, err := fetchParent(context.Background(), pfmodel.FragmentParent{URL: "ext::sh -c id"}, outFeaturesName, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "remote helper")
}

// TestFetchParentReportsAnUnreachableParent: a repository nothing can reach
// fails the whole document, so the caller keeps its committed sections rather
// than mix fresh and stale content.
func TestFetchParentReportsAnUnreachableParent(t *testing.T) {
	_, _, err := fetchParent(context.Background(), pfmodel.FragmentParent{URL: filepath.Join(t.TempDir(), "no-such-remote.git")}, outFeaturesName, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not appear to be a git repository")
}

// TestFetchParentLayersPerFileTranslations: one absent language fails the whole
// batched archive read, so every language falls back to its own file — and the
// per-file half still degrades honestly, a localized copy with no licence
// dropped with a warning while an absent one is a quiet absence.
func TestFetchParentLayersPerFileTranslations(t *testing.T) {
	url := gitFixture(t, map[string]string{
		outFeaturesName: upstreamDoc("### Live Entry", "", "Live body."),
		core.LocalizedFilename(outFeaturesName, "es"): "# Características\n\n### Entrada\n\nCuerpo.\n",
	}, "v1.0.0")

	var log strings.Builder
	genlog.SetOutput(&log)
	defer genlog.SetOutput(os.Stderr)

	cop, translated, err := fetchParent(context.Background(), pfmodel.FragmentParent{URL: url}, outFeaturesName, []string{"es", "uk"})
	require.NoError(t, err)
	assert.Contains(t, cop.Body, "### Live Entry")
	assert.NotContains(t, translated, "es", "an unlicensed localized copy is dropped, not nested")
	assert.NotContains(t, translated, "uk", "a parent publishing no Ukrainian copy is an absence")
	assert.Contains(t, log.String(), "localized parent fetch failed")
}

// TestFetchDocumentRejectsAPathOutsideTheRepository: a path git refuses to
// serve — not a miss, a pathspec outside the tree — surfaces as a real error
// rather than as a parent that publishes nothing, and the parent fetch hands it
// on instead of trying the root.
func TestFetchDocumentRejectsAPathOutsideTheRepository(t *testing.T) {
	url := gitFixture(t, map[string]string{outFeaturesName: upstreamDoc("### Entry", "", "Body.")}, "v1.0.0")

	_, err := fetchDocument(context.Background(), url, "v1.0.0", "../"+outFeaturesName)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside repository")

	_, _, err = fetchParent(context.Background(), pfmodel.FragmentParent{URL: url}, "../"+outFeaturesName, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside repository")
}

// TestParentTitleReadsEachProjectfileEncoding: the title comes from whichever
// encoding a parent publishes — one read per missing one — and a parent that
// publishes none is still inheritable, named by its repository.
func TestParentTitleReadsEachProjectfileEncoding(t *testing.T) {
	ctx := context.Background()

	toml := gitFixture(t, map[string]string{
		pfTOML: "[identity]\ntitle = \"" + testParentTitle + "\"\n",
	}, "v1.0.0")
	assert.Equal(t, testParentTitle, parentTitle(ctx, toml, "v1.0.0"))
	assert.Equal(t, testParentTitle, parentTitle(ctx, toml, "v1.0.0"), "memoized per repository and ref")

	none := gitFixture(t, map[string]string{outFeaturesName: upstreamDoc("### Entry", "", "Body.")}, "v1.0.0")
	assert.Empty(t, parentTitle(ctx, none, "v1.0.0"))
	assert.Equal(t, testParent, ownerRepo(none), "a parent with no title is named by its repository")
}
