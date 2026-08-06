// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
)

// checkWithDiff runs the drift gate with the diff enabled and returns both the
// verdict and everything the run wrote to its stderr.
func checkWithDiff(t *testing.T, dir string, r core.Renderer) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	err := core.RunRender(r, &projectfile.Document{}, core.Options{
		Dir: dir, Check: true, DryRun: true, Diff: true, Stderr: &buf,
	})
	return buf.String(), err
}

// seedFile writes the on-disk (stale) side of a drift case.
func seedFile(t *testing.T, dir, name, body string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644))
}

// The diff must name both sides and show the moved line twice — once as it is
// on disk, once as the projectfile would render it.
func TestCheckDiffShowsBothSides(t *testing.T) {
	dir := t.TempDir()
	r := fixedRenderer{name: generatedFile, body: "# generated\ncontact: fresh@example.org\n"}
	seedFile(t, dir, r.name, "# generated\ncontact: stale@example.org\n")

	out, err := checkWithDiff(t, dir, r)

	require.Error(t, err)
	assert.Contains(t, out, generatedFile+" (on disk)")
	assert.Contains(t, out, generatedFile+" (from the projectfile)")
	assert.Contains(t, out, "-contact: stale@example.org")
	assert.Contains(t, out, "+contact: fresh@example.org")
}

// --no-diff (Diff false) keeps the verdict and drops the body. The gate must
// still fail, so a repository that mutes the diff does not mute the failure.
func TestCheckDiffDisabled(t *testing.T) {
	dir := t.TempDir()
	r := fixedRenderer{name: generatedFile, body: generatedBody}
	seedFile(t, dir, r.name, "# hand-edited\n")

	var buf bytes.Buffer
	err := core.RunRender(r, &projectfile.Document{}, core.Options{
		Dir: dir, Check: true, DryRun: true, Stderr: &buf,
	})

	require.Error(t, err)
	assert.NotContains(t, buf.String(), "-# hand-edited")
}

// A library caller that never handed us a stream gets no output and no panic.
func TestCheckDiffWithoutStderr(t *testing.T) {
	dir := t.TempDir()
	r := fixedRenderer{name: generatedFile, body: generatedBody}
	seedFile(t, dir, r.name, "# hand-edited\n")

	err := core.RunRender(r, &projectfile.Document{}, core.Options{
		Dir: dir, Check: true, DryRun: true, Diff: true,
	})

	require.Error(t, err)
}

// An in-sync file produces no diff at all.
func TestCheckDiffSilentWhenInSync(t *testing.T) {
	dir := t.TempDir()
	r := fixedRenderer{name: generatedFile, body: generatedBody}
	seedFile(t, dir, r.name, generatedBody)

	out, err := checkWithDiff(t, dir, r)

	require.NoError(t, err)
	assert.NotContains(t, out, "(on disk)")
}

// A missing final newline would render as two identical-looking lines. Naming
// the cause beats printing a diff the reader cannot see.
func TestCheckDiffReportsInvisibleWhitespace(t *testing.T) {
	dir := t.TempDir()
	r := fixedRenderer{name: generatedFile, body: generatedBody}
	seedFile(t, dir, r.name, strings.TrimRight(generatedBody, "\n"))

	out, err := checkWithDiff(t, dir, r)

	require.Error(t, err)
	assert.Contains(t, out, "trailing whitespace differs")
	assert.NotContains(t, out, "@@")
}

// CRLF against LF is the other invisible case, and it names itself separately
// because the fix is a .gitattributes entry, not an editor setting.
func TestCheckDiffReportsLineEndings(t *testing.T) {
	dir := t.TempDir()
	r := fixedRenderer{name: generatedFile, body: "alpha\nbeta\n"}
	seedFile(t, dir, r.name, "alpha\r\nbeta\r\n")

	out, err := checkWithDiff(t, dir, r)

	require.Error(t, err)
	assert.Contains(t, out, "line endings differ")
}

// A wholly rewritten file must not bury the rest of a `bridge all` run.
func TestCheckDiffTruncatesLongDiff(t *testing.T) {
	dir := t.TempDir()
	var fresh, stale strings.Builder
	for i := range 200 {
		fmt.Fprintf(&fresh, "fresh line %d\n", i)
		fmt.Fprintf(&stale, "stale line %d\n", i)
	}
	r := fixedRenderer{name: generatedFile, body: fresh.String()}
	seedFile(t, dir, r.name, stale.String())

	out, err := checkWithDiff(t, dir, r)

	require.Error(t, err)
	assert.Contains(t, out, "more diff line(s) omitted")
	assert.Less(t, strings.Count(out, "\n"), 140, "the cap must hold")
}

// Binary content has no line structure to diff; say so with the sizes instead
// of spraying control bytes into the terminal.
func TestCheckDiffSuppressedForBinary(t *testing.T) {
	dir := t.TempDir()
	r := fixedRenderer{name: generatedFile, body: "\x00\x01binary\n"}
	seedFile(t, dir, r.name, generatedBody)

	out, err := checkWithDiff(t, dir, r)

	require.Error(t, err)
	assert.Contains(t, out, "binary content, diff suppressed")
}
