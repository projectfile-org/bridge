// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core

// FieldMapper declares a bidirectional field conversion between an external
// document and projectfile. Syncers return a list of these from BuildMappers,
// closing over the two mutable documents.
//
// `force` controls whether a closure overwrites an existing target value or
// only fills it when the target is empty (gap-fill semantics). The same
// closure body supports both because the only difference is one guard —
// which is what lets the sync algorithm run a "push from authoritative"
// pass and then a "gap-fill from the other side" pass without two mapper
// definitions.
type FieldMapper struct {
	ExtKey string // label rendered when destination is the external file
	PFKey  string // label rendered when destination is projectfile

	// ToPF reads from extDoc and writes to pf. Returns a short display
	// value to record in FieldChange, or "" when no change happened.
	ToPF func(force bool) string

	// FromPF reads from pf and writes to extDoc. Same semantics as ToPF.
	FromPF func(force bool) string
}

// MapperList is the slice returned by Syncer.BuildMappers; defining it as a
// named type keeps driver signatures self-documenting.
type MapperList []FieldMapper
