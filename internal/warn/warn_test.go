// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package warn_test

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"projectfile.org/projectfile/bridge/internal/warn"
)

const driftMsg = "drift: file no longer matches the projectfile"

// A run with nothing to say prints nothing: the summary must not turn a clean
// command into two extra lines of output.
func TestSummarySilentWhenEmpty(t *testing.T) {
	warn.Reset()
	var buf bytes.Buffer

	warn.Summary(&buf)

	assert.Empty(t, buf.String())
}

// The block names every warning and the count, so the reader can act on it
// without scrolling back through the run that produced them.
func TestSummaryListsEveryWarning(t *testing.T) {
	warn.Reset()
	warn.Record(driftMsg, "file", "README.md")
	warn.Record("drift: file is missing", "file", "SUPPORT.md")
	warn.Hint("regenerate to fix")
	var buf bytes.Buffer

	warn.Summary(&buf)
	out := buf.String()

	assert.Equal(t, 2, warn.Count())
	assert.Contains(t, out, "2 warning(s)")
	assert.Contains(t, out, "file=README.md")
	assert.Contains(t, out, "file=SUPPORT.md")
	assert.Contains(t, out, "regenerate to fix")
}

// The same remedy recorded per file must appear once, not once per file.
func TestHintDeduplicates(t *testing.T) {
	warn.Reset()
	warn.Record(driftMsg, "file", "a")
	warn.Hint("regenerate to fix")
	warn.Record(driftMsg, "file", "b")
	warn.Hint("regenerate to fix")
	var buf bytes.Buffer

	warn.Summary(&buf)

	assert.Equal(t, 1, strings.Count(buf.String(), "regenerate to fix"))
}

// A multi-line message is ONE warning. The handoff file is line-oriented, so
// left unflattened a nine-line configuration hint would arrive at the parent as
// nine warnings and report a count nine times the truth.
func TestMultilineMessageStaysOneEntry(t *testing.T) {
	warn.Reset()
	warn.Record("bridge failed", "error", "no providers configured.\n  Add [org.projectfile.funding]\n  github = [\"handle\"]")
	var buf bytes.Buffer

	warn.Summary(&buf)
	body := strings.TrimSpace(buf.String())

	assert.Equal(t, 1, warn.Count())
	assert.Equal(t, 2, len(strings.Split(body, "\n")), "title line plus one entry")
}

// Past the cap the block still reports the true total — truncating the list must
// never truncate the count.
func TestSummaryCapsListNotCount(t *testing.T) {
	warn.Reset()
	for i := range 25 {
		warn.Record(driftMsg, "file", string(rune('a'+i)))
	}
	var buf bytes.Buffer

	warn.Summary(&buf)
	out := buf.String()

	assert.Contains(t, out, "25 warning(s)")
	assert.Contains(t, out, "more (of 25)")
}

// Under a collecting parent the child writes its ledger out and prints no block
// of its own — otherwise a 20-bridge fan-out ends in 20 summaries.
func TestHandoffReplacesTheChildSummary(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(warn.EnvDir, dir)
	warn.Reset()
	warn.Record(driftMsg, "file", "README.md")
	var buf bytes.Buffer

	warn.Summary(&buf)

	assert.Empty(t, buf.String(), "the parent prints, not the child")
	files, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, files, 1)
}

// The parent side of the same exchange: a child's entries become the parent's,
// and the directory does not outlive the run.
func TestStartHandoffCollectsChildEntries(t *testing.T) {
	warn.Reset()
	dir, collect := warn.StartHandoff()
	require.NotEmpty(t, dir)

	writeChildLedger(t, dir)
	collect()
	var buf bytes.Buffer
	warn.Summary(&buf)
	out := buf.String()

	assert.Contains(t, out, "file=CHILD.md")
	assert.Contains(t, out, "child remedy")
	assert.NoDirExists(t, dir)
}

// writeChildLedger runs a real child process against the handoff directory, so
// the test exercises the contract as the fan-out uses it (a separate process
// reading EnvDir) rather than a hand-written file the format could drift from.
func writeChildLedger(t *testing.T, dir string) {
	t.Helper()
	// `go run` needs the module, so drive the ledger through this package's own
	// test binary re-executed in child mode instead.
	cmd := exec.Command(os.Args[0], "-test.run=TestHelperChildLedger") // #nosec G204 -- re-exec of this test binary
	cmd.Env = append(os.Environ(), "PF_BRIDGE_TEST_CHILD=1", warn.EnvDir+"="+dir)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
}

// TestHelperChildLedger is the child half of TestStartHandoffCollectsChildEntries:
// it records a warning and flushes, which under EnvDir means writing the handoff
// file. Inert unless the parent asked for it.
func TestHelperChildLedger(t *testing.T) {
	if os.Getenv("PF_BRIDGE_TEST_CHILD") != "1" {
		t.Skip("child-mode helper")
	}
	warn.Record(driftMsg, "file", "CHILD.md")
	warn.Hint("child remedy")
	warn.Summary(os.Stderr)
}
