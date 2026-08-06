// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core

import (
	"fmt"
	"strings"
)

// FieldChange describes one field that was (or would be) copied between the
// two documents. From/To carry the human-friendly file names so the renderer
// reads naturally regardless of which side is the source.
type FieldChange struct {
	Key   string
	Value string
	From  string
	To    string
}

// Result is the unified outcome of a RunSync call. It is intentionally
// bridge-agnostic: the bridge supplies ExtName via Labels() so Format()
// reads naturally for any future bridge.
type Result struct {
	DryRun  bool
	Created bool

	ExtName string // "CITATION.cff" / "package.json" / ...
	PFName  string // "projectfile"

	ExtChanged bool
	PFChanged  bool
	ExtFields  []FieldChange
	PFFields   []FieldChange
}

// Format renders the Result as the text the CLI prints. It is dry-run aware:
// "Would update" / "Would create" replaces "Updated" / "Created" when the
// caller passed --dry-run, so users do not misread the output.
func (r *Result) Format() string {
	if !r.PFChanged && !r.ExtChanged {
		return "Already in sync — no changes needed"
	}

	updated := "Updated"
	created := "Created"
	if r.DryRun {
		updated = "Would update"
		created = "Would create"
	}

	var summary string
	switch {
	case r.Created && r.ExtChanged && r.PFChanged:
		summary = fmt.Sprintf("%s %s (%d fields) and %s %s (%d fields)",
			created, r.ExtName, len(r.ExtFields), updated, r.PFName, len(r.PFFields))
	case r.Created && r.ExtChanged:
		summary = fmt.Sprintf("%s %s (%d fields)", created, r.ExtName, len(r.ExtFields))
	case r.ExtChanged && r.PFChanged:
		summary = fmt.Sprintf("Bridged: %s %s (%d fields) and %s (%d fields)",
			strings.ToLower(updated), r.ExtName, len(r.ExtFields), r.PFName, len(r.PFFields))
	case r.ExtChanged:
		summary = fmt.Sprintf("Bridged: %s %s (%d fields)", strings.ToLower(updated), r.ExtName, len(r.ExtFields))
	case r.PFChanged:
		summary = fmt.Sprintf("Bridged: %s %s (%d fields)", strings.ToLower(updated), r.PFName, len(r.PFFields))
	}

	var lines []string
	for _, f := range r.ExtFields {
		lines = append(lines, formatField(f))
	}
	for _, f := range r.PFFields {
		lines = append(lines, formatField(f))
	}
	if len(lines) == 0 {
		return summary
	}
	return summary + "\n" + strings.Join(lines, "\n")
}

func formatField(f FieldChange) string {
	return fmt.Sprintf("  %s (%s) copied from %s to %s", f.Key, f.Value, f.From, f.To)
}

// Trunc caps display strings used in FieldChange values so a 5 KB README
// abstract does not dominate the CLI output.
func Trunc(s string) string {
	const maxLen = 60
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
