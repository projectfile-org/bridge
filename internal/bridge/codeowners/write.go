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
	if doc.ReuseHeader != "" {
		buf.WriteString(doc.ReuseHeader)
	}
	for _, l := range doc.Lines {
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
