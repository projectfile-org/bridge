// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package warn is the ledger behind every warning pf-bridge emits that the user
// must still see once the run has scrolled past.
//
// What we are trying to do: make a non-fatal finding impossible to miss. A
// warning printed in the middle of a 40-file `bridge all` run is a warning
// nobody reads, and under a buffering runner (m6e-run keeps the whole tool log
// in reports/<target>.log and prints ONE pointer line when the command exits 0)
// it is not printed at all. So every call here does two things: it prints
// immediately, next to the work that produced it, AND it is kept, so Summary can
// re-state the whole set as the last thing the process writes.
//
// The ledger is process-wide because the alternative — threading a collector
// through Options into every bridge — buys nothing: one process is one command
// run, and the CLI is the only caller that flushes.
package warn

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
)

// EnvDir names a directory a PARENT pf-bridge process creates so each child
// hands its warnings back instead of printing a summary of its own — otherwise
// a fan-out over 20 bridges ends in 20 separate summaries and the reader is no
// better off than before. Each child writes ONE file named after its pid, so the
// fan-out shares no mutable file and needs no lock.
const EnvDir = "PF_BRIDGE_WARNINGS_DIR"

// Shape of the summary block. The cap keeps one thoroughly drifted repository
// from replacing the run's output with its own ledger; the count line still
// reports the true total.
const (
	summaryMaxEntries = 20
	handoffExt        = ".warnings"
)

// Styles for the summary block. Yellow and bold by intent: this is the one part
// of the output that must survive a skim. lipgloss drops the escapes itself when
// the destination is not a terminal, so a captured log stays plain text.
var (
	styleSummaryRule  = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	styleSummaryTitle = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)
	styleSummaryHint  = lipgloss.NewStyle().Foreground(lipgloss.Color("8")).Italic(true)
)

var (
	mu      sync.Mutex
	entries []string
	hints   []string
)

// Record prints one warning through genlog — so it lands in context, beside the
// file that produced it — and keeps a copy for Summary.
func Record(msg string, kv ...any) {
	genlog.Warn(msg, kv...)
	mu.Lock()
	defer mu.Unlock()
	entries = append(entries, format(msg, kv...))
}

// Hint registers a one-line remedy printed under the summary block. Deduplicated
// so a per-file caller can state the fix on every drift without repeating it 40
// times at the end.
func Hint(s string) {
	if s == "" {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	for _, h := range hints {
		if h == s {
			return
		}
	}
	hints = append(hints, s)
}

// Count reports how many warnings the ledger holds. The CLI reads it to decide
// an exit code when the caller asked for warnings to be fatal.
func Count() int {
	mu.Lock()
	defer mu.Unlock()
	return len(entries)
}

// Reset empties the ledger. Tests only — a command run is one process.
func Reset() {
	mu.Lock()
	defer mu.Unlock()
	entries, hints = nil, nil
}

// Summary writes the end-of-run block, or hands the ledger to the parent process
// when one is collecting (EnvDir). No warnings means no output at all — a clean
// run must stay one line.
func Summary(w io.Writer) {
	mu.Lock()
	snapshot := append([]string(nil), entries...)
	hintList := append([]string(nil), hints...)
	mu.Unlock()

	if len(snapshot) == 0 {
		return
	}
	if dir := os.Getenv(EnvDir); dir != "" {
		handOff(dir, snapshot, hintList)
		return
	}
	WriteSummary(w, snapshot, hintList)
}

// WriteSummary renders the block. Exported so the fan-out dispatcher can print
// the ONE aggregate it assembled from its children through the same renderer.
func WriteSummary(w io.Writer, list, hintList []string) {
	if w == nil || len(list) == 0 {
		return
	}
	title := fmt.Sprintf("%d warning(s) — the command did not fail", len(list))
	if len(list) == 1 {
		title = "1 warning — the command did not fail"
	}

	fmt.Fprintln(w)
	fmt.Fprintln(w, styleSummaryRule.Render(strings.Repeat("─", 3))+" "+styleSummaryTitle.Render(title))
	shown := list
	if len(shown) > summaryMaxEntries {
		shown = shown[:summaryMaxEntries]
	}
	for _, e := range shown {
		fmt.Fprintln(w, "  - "+e)
	}
	if omitted := len(list) - len(shown); omitted > 0 {
		fmt.Fprintf(w, "  … %d more (of %d)\n", omitted, len(list))
	}
	for _, h := range hintList {
		fmt.Fprintln(w, styleSummaryHint.Render("  → "+h))
	}
}

// StartHandoff prepares a collection directory for a fan-out and returns it plus
// the collector that folds every child's ledger into this process's own. The
// caller exports the directory through EnvDir into the children's environment.
//
// A failure to create the directory is NOT an error the caller must handle: the
// fan-out still runs, each child simply prints its own summary as it would
// alone. Degrading to noisier output beats refusing to run.
func StartHandoff() (dir string, collect func()) {
	dir, err := os.MkdirTemp("", "pf-bridge-warnings-")
	if err != nil {
		genlog.Info("warning handoff unavailable", "error", err.Error())
		return "", func() {}
	}
	return dir, func() {
		list, hintList := readHandoff(dir)
		if err := os.RemoveAll(dir); err != nil {
			genlog.Info("warning handoff cleanup failed", "dir", dir, "error", err.Error())
		}
		mu.Lock()
		entries = append(entries, list...)
		mu.Unlock()
		for _, h := range hintList {
			Hint(h)
		}
	}
}

// hintPrefix marks a handoff line as a remedy rather than a warning. A prefix
// keeps the handoff one flat, append-only text file per child — the format a
// crashed child can still leave behind half-written without corrupting a peer.
const hintPrefix = "→ "

// handOff writes this process's ledger where the parent will find it. Best
// effort by design: a warning that cannot be handed over has already been
// printed inline by Record, so nothing is lost that was not already on screen.
//
// The destination directory comes from the environment, but the FILE NAME is
// ours (this pid) — nothing a caller supplies reaches the basename, so the write
// cannot escape the directory the parent chose. Setting that directory requires
// control of our environment, which is the same authority as setting our PATH:
// a principal holding it can already run anything as us.
func handOff(dir string, list, hintList []string) {
	name := filepath.Join(filepath.Clean(dir), fmt.Sprintf("%d%s", os.Getpid(), handoffExt))
	var b strings.Builder
	for _, e := range list {
		b.WriteString(e + "\n")
	}
	for _, h := range hintList {
		b.WriteString(hintPrefix + h + "\n")
	}
	if err := os.WriteFile(name, []byte(b.String()), 0o600); err != nil { // #nosec G703 -- basename is this pid; the directory is the parent's, see above
		genlog.Info("warning handoff write failed", "file", name, "error", err.Error())
	}
}

// readHandoff collects every child's file in a stable order (pid-named files
// sorted by name) so two runs over the same tree print the same block.
func readHandoff(dir string) (list, hintList []string) {
	items, err := os.ReadDir(dir)
	if err != nil {
		genlog.Info("warning handoff read failed", "dir", dir, "error", err.Error())
		return nil, nil
	}
	names := make([]string, 0, len(items))
	for _, it := range items {
		if strings.HasSuffix(it.Name(), handoffExt) {
			names = append(names, it.Name())
		}
	}
	sort.Strings(names)

	for _, n := range names {
		body, err := os.ReadFile(filepath.Join(dir, n)) // #nosec G304 -- our own temp dir, created by StartHandoff
		if err != nil {
			genlog.Info("warning handoff entry unreadable", "file", n, "error", err.Error())
			continue
		}
		for _, line := range strings.Split(strings.TrimRight(string(body), "\n"), "\n") {
			switch {
			case line == "":
			case strings.HasPrefix(line, hintPrefix):
				hintList = append(hintList, strings.TrimPrefix(line, hintPrefix))
			default:
				list = append(list, line)
			}
		}
	}
	return list, hintList
}

// Shape of one ledger entry. Both caps exist because a summary is an INDEX, not
// a second copy of the output: the full text already printed inline, above.
const (
	entryMaxLen = 160
	ellipsis    = "…"
)

// format flattens a genlog message plus its key/value pairs into the one line
// the summary shows. A dangling key (odd kv count) is printed bare rather than
// dropped — a half-written call still names what it was about.
//
// The result is forced onto ONE line: a multi-line message (a bridge's
// "here is how to configure me" error is nine lines of guidance) is handed
// between processes through a line-oriented file, so left as-is it would arrive
// as nine separate warnings and report a count nine times the truth.
func format(msg string, kv ...any) string {
	var b strings.Builder
	b.WriteString(msg)
	for i := 0; i < len(kv); i += 2 {
		if i+1 >= len(kv) {
			fmt.Fprintf(&b, " %v", kv[i])
			break
		}
		fmt.Fprintf(&b, " %v=%v", kv[i], kv[i+1])
	}
	return truncate(strings.Join(strings.Fields(b.String()), " "))
}

// truncate caps one entry at a scannable width, on a rune boundary so a
// multi-byte character is never cut in half.
func truncate(s string) string {
	if len(s) <= entryMaxLen {
		return s
	}
	runes := []rune(s)
	for len(string(runes)) > entryMaxLen-len(ellipsis) {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + ellipsis
}
