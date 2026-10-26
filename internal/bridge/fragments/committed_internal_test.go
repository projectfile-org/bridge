// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package fragments

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
)

// itestHeadingInherited is the canonical parent section heading the assembler
// writes for a parent that publishes no title.
const itestHeadingInherited = "## Inherited from b19/ubuntu"

// The offline reader re-parses the layout assembly emits, and the fence is
// where a parent's own code samples live: a `# comment` or an H2 inside a fence
// is text, never a section boundary.

// TestExtractInheritedKeepsFencedLinesVerbatim covers the fence paths of the
// offline reader: an H2, the project heading and a directive inside a fence
// belong to the parent's body, so a section that carries a shell example
// round-trips whole — and both fence dialects close the block.
func TestExtractInheritedKeepsFencedLinesVerbatim(t *testing.T) {
	dir := t.TempDir()
	body := "<!--\nSPDX-FileCopyrightText: 2026 Tester\nSPDX-License-Identifier: MIT\n-->\n\n" +
		"# Features\n\n" +
		"## Project Features\n\n### Own\n\nOwn body.\n\n" +
		itestHeadingInherited + "\n\n" +
		"### Entry\n\n```sh\n## not a heading\n## Project Features\n# install the thing\nmake install\n```\n\n" +
		"~~~\n## still fenced\n~~~\n\n### Second\n\nBody.\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, outFeaturesName), []byte(body), 0o644))

	sections, ok := extractInherited(dir, outFeaturesName, "## Project Features")
	require.True(t, ok)
	require.Len(t, sections, 1)
	got := sections[0].Body
	assert.Contains(t, got, "## not a heading", "an H2 inside a fence is code, not a boundary")
	assert.Contains(t, got, "## Project Features")
	assert.Contains(t, got, "# install the thing")
	assert.Contains(t, got, "### Second", "the section continues after the fence closes")
}

// TestExtractInheritedWarnsOnUnreadableDocument covers the one read failure
// that is not "nothing inherited yet": the document exists but cannot be read,
// which must warn and inherit nothing rather than fail the render.
func TestExtractInheritedWarnsOnUnreadableDocument(t *testing.T) {
	dir := t.TempDir()
	// A directory where the document should be: ReadFile fails, the file exists.
	require.NoError(t, os.MkdirAll(filepath.Join(dir, outFeaturesName), 0o755))

	var log strings.Builder
	genlog.SetOutput(&log)
	defer genlog.SetOutput(os.Stderr)

	sections, ok := extractInherited(dir, outFeaturesName, "## Project Features")
	assert.False(t, ok)
	assert.Empty(t, sections)
	assert.Contains(t, log.String(), "fragments: committed document unreadable")
}

// TestExtractInheritedMissingDocumentIsQuiet: an absent document is the state a
// project starts in, so it warns nothing.
func TestExtractInheritedMissingDocumentIsQuiet(t *testing.T) {
	dir := t.TempDir()
	var log strings.Builder
	genlog.SetOutput(&log)
	defer genlog.SetOutput(os.Stderr)

	sections, ok := extractInherited(dir, outFeaturesName, "## Project Features")
	assert.False(t, ok)
	assert.Empty(t, sections)
	assert.Empty(t, log.String())
}

// TestInheritedParentReadsEveryLanguage verifies a heading written in any
// shipped language resolves to its parent name, and a heading with no known
// prefix resolves to itself.
func TestInheritedParentReadsEveryLanguage(t *testing.T) {
	for heading, want := range map[string]string{
		"## Inherited from b19/ubuntu":   testParent,
		"## Heredado de b19/ubuntu":      testParent,
		"## Успадковано від b19/ubuntu":  testParent,
		"## Some other heading":          "Some other heading",
		"Inherited from no-hash-heading": "no-hash-heading",
	} {
		assert.Equal(t, want, InheritedParent(heading), heading)
	}
}

// TestShortCommitAbbreviatesForDisplay pins the display width used by the logs.
func TestShortCommitAbbreviatesForDisplay(t *testing.T) {
	assert.Equal(t, "db0ef9b", shortCommit("db0ef9b031b82d5"))
	assert.Equal(t, "abc1234", shortCommit("abc1234"))
	assert.Equal(t, "abc", shortCommit("abc"))
}

// TestLoadFragmentsFailsOnMalformedPattern verifies a fragment dir whose glob
// is malformed fails the load — the one error a call site can hand the reader.
func TestLoadFragmentsFailsOnMalformedPattern(t *testing.T) {
	_, err := loadFragments(t.TempDir(), "docs/[features.d")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "syntax error in pattern")
}
