// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/pmezard/go-difflib/difflib"
)

// Shape of the drift diff. Three lines of context place a hunk inside the
// generated file without reprinting it; the line cap keeps one fully
// regenerated README from burying the rest of a `bridge all` run in a CI log.
const (
	diffContextLines = 3
	diffMaxLines     = 120
)

// Styles for the diff body. Same subdued palette as the decision trace —
// lipgloss drops the escapes by itself when the destination is not a terminal,
// so a CI log stays plain text.
var (
	styleDiffAdd    = lipgloss.NewStyle().Foreground(lipgloss.Color("2")) // green
	styleDiffDel    = lipgloss.NewStyle().Foreground(lipgloss.Color("1")) // red
	styleDiffHeader = lipgloss.NewStyle().Foreground(lipgloss.Color("8")) // dim
)

// WriteDiff prints the unified diff behind ONE drift report: what the file
// holds now ("-") against what re-rendering the projectfile would put there
// ("+"). Callers pass it the two byte slices --check already compared, so the
// diff costs nothing beyond formatting.
//
// Drift with no visible cause is the expensive failure. The value that moved
// can come from the projectfile, an include, a template, or the per-user
// config, and a bare "drifted" verdict sends the reader hunting through all
// four — usually by regenerating, which destroys the evidence.
func WriteDiff(w io.Writer, rel string, onDisk, rendered []byte) {
	if w == nil {
		return
	}
	if !isDiffable(onDisk) || !isDiffable(rendered) {
		fmt.Fprintf(w, "  (binary content, diff suppressed: %d bytes on disk, %d rendered)\n",
			len(onDisk), len(rendered))
		return
	}
	if note := invisibleDiffNote(onDisk, rendered); note != "" {
		fmt.Fprintf(w, "  (%s: %d bytes on disk, %d rendered)\n", note, len(onDisk), len(rendered))
		return
	}

	text, err := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
		A:        difflib.SplitLines(string(onDisk)),
		B:        difflib.SplitLines(string(rendered)),
		FromFile: rel + " (on disk)",
		ToFile:   rel + " (from the projectfile)",
		Context:  diffContextLines,
	})
	if err != nil {
		fmt.Fprintf(w, "  (diff unavailable for %s: %s)\n", rel, err)
		return
	}

	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	shown, omitted := lines, 0
	if len(lines) > diffMaxLines {
		shown, omitted = lines[:diffMaxLines], len(lines)-diffMaxLines
	}
	for _, line := range shown {
		fmt.Fprintln(w, "  "+styleDiffLine(line))
	}
	if omitted > 0 {
		fmt.Fprintf(w, "  … %d more diff line(s) omitted (of %d)\n", omitted, len(lines))
	}
}

// styleDiffLine colours one unified-diff line by its marker. The file headers
// carry the same marker characters as the body, so they are matched first.
func styleDiffLine(line string) string {
	switch {
	case strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"), strings.HasPrefix(line, "@@"):
		return styleDiffHeader.Render(line)
	case strings.HasPrefix(line, "+"):
		return styleDiffAdd.Render(line)
	case strings.HasPrefix(line, "-"):
		return styleDiffDel.Render(line)
	default:
		return line
	}
}

// invisibleDiffNote names the difference when a line-by-line diff would show
// two identical-looking sides: a stray CRLF, a trailing space, or a missing
// final newline. Printing "-foo / +foo" for those is worse than printing
// nothing — the reader concludes the tool is broken.
func invisibleDiffNote(onDisk, rendered []byte) string {
	if !bytes.Equal(normalizeInvisible(onDisk), normalizeInvisible(rendered)) {
		return ""
	}
	if bytes.Contains(onDisk, []byte("\r\n")) != bytes.Contains(rendered, []byte("\r\n")) {
		return "line endings differ (CRLF against LF); content is identical"
	}
	return "trailing whitespace differs; content is identical"
}

// normalizeInvisible strips the differences no reader can see in a diff:
// CR characters, per-line trailing spaces, and the final newline.
func normalizeInvisible(b []byte) []byte {
	lines := strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	return []byte(strings.TrimRight(strings.Join(lines, "\n"), "\n"))
}

// diffOut resolves where a drift diff goes. A nil Stderr means the caller is a
// library consumer that never opted in to CLI output, so the diff is dropped
// rather than pushed onto a stream it does not own — the rule RunSync uses.
func diffOut(opts Options) io.Writer {
	if opts.Stderr == nil {
		return io.Discard
	}
	return opts.Stderr
}

// isDiffable reports whether b is text a line-oriented diff can describe.
func isDiffable(b []byte) bool {
	return utf8.Valid(b) && !bytes.ContainsRune(b, 0)
}
