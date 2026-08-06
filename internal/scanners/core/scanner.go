// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package core defines the Scanner interface and its registry. Scanners are
// the init-time counterpart to source.Source: where Source extracts metadata
// from declarative files (package.json, CITATION.cff, ...), Scanner derives
// it from filesystem-wide signals (git history, language markers, ...). Both
// produce a *source.Partial so init's existing merge pipeline can fold their
// output together with the same "existing wins, fill gaps" semantics.
package core

import (
	"projectfile.org/projectfile/bridge/internal/source"
)

// Scanner is the interface every init-time auto-discovery driver implements.
// Implementations self-register from init() via Register, mirroring the
// pattern in internal/bridge/.
type Scanner interface {
	// Name identifies the scanner ("stack", "git"); used as a registry key
	// and surfaced in Hit.Source so users can audit *why* a field appeared.
	Name() string
	// Detect is a cheap probe — return true iff the scanner is applicable to
	// root. RunAll skips Scan calls for which Detect returned false.
	Detect(root string) bool
	// Scan walks/queries root and returns a Partial with the fields this
	// scanner can supply (others left nil/zero) plus per-finding Hits. An
	// error aborts the run for this scanner only; the rest still execute.
	Scan(root string) (*source.Partial, []Hit, error)
}

// Hit records one piece of evidence the scanner emitted, for logging.
// Source is the scanner's Name(); Field is a free-form locator
// ("stack:go", "repositories[role=origin]", "people:Damián Búho"); Path is optional
// filesystem evidence when relevant.
type Hit struct {
	Source string
	Field  string
	Path   string
}
