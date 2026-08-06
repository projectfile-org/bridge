// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package codeowners models the GitHub-flavoured CODEOWNERS file format.
//
// The model is a flat ordered list of Lines — every line is either an Entry
// (pattern + owners), a comment (#-prefixed), or blank. This preserves source
// order and comments through a parse → write round-trip, which is the only
// way to keep "last match wins per path" semantics stable.
//
// Round-trip with projectfile: pf has no slot for comments, so the first
// to-pf sync normalises any hand-written file (comments stripped); subsequent
// runs are byte-stable. See AGENTS.md for the v1 limitation note.
package codeowners

// Filename is the canonical on-disk name. The driver also probes
// `.github/CODEOWNERS` and `docs/CODEOWNERS` per GitHub's autodiscovery.
const Filename = "CODEOWNERS"

// Kind tags the Line variant. Using a string-typed kind plus a thin struct
// keeps the package zero-dependency and trivial to reflect over in tests
// — a real sum type would force a visitor and extra surface for one tiny use.
type Kind string

const (
	KindEntry   Kind = "entry"
	KindComment Kind = "comment"
	KindBlank   Kind = "blank"
)

// Line is one parsed line. Only one of (Pattern+Owners) / Text is populated
// depending on Kind.
type Line struct {
	Kind    Kind
	Pattern string   // KindEntry only
	Owners  []string // KindEntry only; verbatim — `@handle`, `@org/team`, `email@host`
	Text    string   // KindComment: the comment body (after `#`); KindBlank: empty
}

// Document is the order-preserving view of a CODEOWNERS file.
type Document struct {
	Lines []Line

	// ReuseHeader is the REUSE/SPDX block prepended ahead of the entry body
	// on Write. The sync driver populates it in BuildMappers (which has pf
	// available); a zero value disables the prepend so non-sync callers stay
	// header-free.
	ReuseHeader string
}

// Entries returns just the Pattern+Owners rows, in source order. Used by the
// sync mapper when comparing extension state against file state.
func (d *Document) Entries() []Line {
	out := make([]Line, 0, len(d.Lines))
	for _, l := range d.Lines {
		if l.Kind == KindEntry {
			out = append(out, l)
		}
	}
	return out
}
