// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package codeowners

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Bytes serialises doc back to CODEOWNERS canonical form. Entries emit as
// `<pattern> <owner1> <owner2> ...\n`; comments as `# <text>\n`; blanks as
// a single newline. Trailing newline is always present.
func Bytes(doc *Document) []byte {
	if doc == nil {
		return nil
	}
	var buf bytes.Buffer
	lines := doc.Lines
	if doc.ReuseHeader != "" {
		// Read keeps the header it found as ordinary comment lines, so writing
		// the current one on top of them stacks a second copy on every write.
		// Drop the old one first: the header is a Write-time output, never an
		// inherited document value.
		lines = stripLeadingREUSE(lines)
		buf.WriteString(doc.ReuseHeader)
	}
	for _, l := range lines {
		switch l.Kind {
		case KindEntry:
			buf.WriteString(l.Pattern)
			for _, o := range l.Owners {
				buf.WriteByte(' ')
				buf.WriteString(o)
			}
			buf.WriteByte('\n')
		case KindComment:
			buf.WriteByte('#')
			text := l.Text
			// Re-add a leading space for human-friendly comment form unless
			// the original line was a marker-style comment (`#!`, `#:`) or
			// blank-after-`#`.
			if text != "" && !strings.HasPrefix(text, " ") && !strings.HasPrefix(text, "!") && !strings.HasPrefix(text, ":") {
				buf.WriteByte(' ')
			}
			buf.WriteString(text)
			buf.WriteByte('\n')
		case KindBlank:
			buf.WriteByte('\n')
		}
	}
	return buf.Bytes()
}

// spdxTagPrefix opens every REUSE tag line. Matching the prefix (rather than
// each tag) keeps the check true whatever tags a header carries.
const spdxTagPrefix = "SPDX-"

// stripLeadingREUSE removes a previously written REUSE header from the head of
// lines. The region runs to the LAST tag comment in the opening comment/blank
// run, plus the blank lines after it — so a comment a human wrote below the
// header survives, while the header itself never accumulates.
func stripLeadingREUSE(lines []Line) []Line {
	last := -1
	for i, l := range lines {
		if l.Kind == KindBlank {
			continue
		}
		if l.Kind != KindComment {
			break
		}
		if strings.HasPrefix(strings.TrimSpace(l.Text), spdxTagPrefix) {
			last = i
		}
	}
	if last < 0 {
		return lines
	}
	cut := last + 1
	for cut < len(lines) && lines[cut].Kind == KindBlank {
		cut++
	}
	return lines[cut:]
}

// Write serialises doc to the first existing CODEOWNERS probe path under dir.
// When no file exists yet, writes to dir/CODEOWNERS (the canonical root path).
func Write(dir string, doc *Document) error {
	path := LocatedPath(dir)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { // #nosec G301 -- VCS-tracked metadata directory
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, Bytes(doc), 0o644); err != nil { // #nosec G306 -- VCS-tracked metadata, world-readable
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// Clone returns a deep copy of doc — used by the dry-run path in sync/core
// so the planning phase doesn't mutate the caller's in-memory document.
func Clone(doc *Document) *Document {
	if doc == nil {
		return nil
	}
	cp := &Document{Lines: make([]Line, len(doc.Lines)), ReuseHeader: doc.ReuseHeader}
	for i, l := range doc.Lines {
		cl := l
		if l.Owners != nil {
			cl.Owners = append([]string(nil), l.Owners...)
		}
		cp.Lines[i] = cl
	}
	return cp
}
