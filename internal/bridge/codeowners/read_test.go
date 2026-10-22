// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package codeowners

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testProbeGithub = ".github/" + Filename
	testProbeDocs   = "docs/" + Filename
)

// writeProbe plants one CODEOWNERS probe path with body under dir.
func writeProbe(t *testing.T, dir, rel, body string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(dir, rel)), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, rel), []byte(body), 0o644))
}

// readFile returns path's contents, failing the test when it cannot be read.
func readFile(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(body)
}

func TestLocatedPathFallsBackToTheRootWithoutAFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	assert.Equal(t, filepath.Join(dir, Filename), LocatedPath(dir))
}

func TestLocatedPathPrefersTheFirstProbeThatExists(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeProbe(t, dir, testProbeDocs, testPatternDeploy+" "+testOwnerOps+"\n")
	writeProbe(t, dir, testProbeGithub, testPatternSrc+" "+testOwnerAda+"\n")
	assert.Equal(t, filepath.Join(dir, testProbeGithub), LocatedPath(dir), "GitHub's own search order wins")
}

func TestLocatedPathReadsTheDocsProbeAlone(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeProbe(t, dir, testProbeDocs, testPatternDeploy+" "+testOwnerOps+"\n")
	assert.Equal(t, filepath.Join(dir, testProbeDocs), LocatedPath(dir))
}

func TestExistsInReportsAnyProbePath(t *testing.T) {
	t.Parallel()

	assert.False(t, ExistsIn(t.TempDir()))
	for _, rel := range []string{Filename, testProbeGithub, testProbeDocs} {
		dir := t.TempDir()
		writeProbe(t, dir, rel, testPatternSrc+" "+testOwnerAda+"\n")
		assert.True(t, ExistsIn(dir), rel)
	}
}

func TestReadReturnsAnEmptyDocumentWithoutAFile(t *testing.T) {
	t.Parallel()

	doc, err := Read(t.TempDir())
	require.NoError(t, err)
	assert.Empty(t, doc.Lines, "no file is an empty document, not an error")
}

func TestReadParsesEntriesCommentsAndBlanks(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeProbe(t, dir, Filename, "# owners team\n\n"+testPatternSrc+" "+testOwnerAda+" "+testOwnerOps+" # trailing\n"+testPatternDeploy+" "+testOwnerOps+"\n")
	doc, err := Read(dir)
	require.NoError(t, err)
	require.Len(t, doc.Lines, 4)
	assert.Equal(t, Line{Kind: KindComment, Text: testCommentOwners}, doc.Lines[0])
	assert.Equal(t, Line{Kind: KindBlank}, doc.Lines[1])
	assert.Equal(t, Line{Kind: KindEntry, Pattern: testPatternSrc, Owners: []string{testOwnerAda, testOwnerOps}}, doc.Lines[2],
		"an inline trailing comment is dropped on parse")
	assert.Equal(t, Line{Kind: KindEntry, Pattern: testPatternDeploy, Owners: []string{testOwnerOps}}, doc.Lines[3])
}

func TestReadParsesTheGithubProbe(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	writeProbe(t, dir, testProbeGithub, testPatternDeploy+" "+testOwnerOps+"\n")
	doc, err := Read(dir)
	require.NoError(t, err)
	require.Len(t, doc.Lines, 1)
	assert.Equal(t, testPatternDeploy, doc.Lines[0].Pattern)
}

func TestParseLineClassifiesEveryKind(t *testing.T) {
	t.Parallel()

	assert.Equal(t, Line{Kind: KindBlank}, parseLine("   "))
	assert.Equal(t, Line{Kind: KindComment, Text: " keep"}, parseLine("# keep"))
	assert.Equal(t, Line{Kind: KindComment, Text: "zizmor"}, parseLine("#zizmor"), "a marker-style comment keeps its text without the #")
	assert.Equal(t, Line{Kind: KindComment, Text: ""}, parseLine("#"))
	assert.Equal(t, Line{Kind: KindEntry, Pattern: testPatternSrc, Owners: []string{testOwnerAda}}, parseLine(testPatternSrc+" "+testOwnerAda))
}

// A file stat can see but open cannot read surfaces the wrapped error rather
// than an empty document, so the caller never mistakes it for "no file".
func TestReadReportsAnUnreadableFile(t *testing.T) {
	t.Parallel()

	if os.Geteuid() == 0 {
		t.Skip("root reads a mode-000 file, so the error path cannot trigger")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, Filename)
	require.NoError(t, os.WriteFile(path, []byte(testPatternSrc+" "+testOwnerAda+"\n"), 0o000))
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })

	_, err := Read(dir)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open "+path)
}

func TestAbsJoinCleansTheProbePath(t *testing.T) {
	t.Parallel()

	got, err := absJoin(t.TempDir(), "x/../"+Filename)
	require.NoError(t, err)
	assert.Equal(t, Filename, filepath.Base(got))
}
