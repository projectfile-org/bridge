// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core

import "reflect"

// FieldMapper declares a bidirectional field conversion between an external
// document and projectfile. Syncers return a list of these from BuildMappers,
// closing over the two mutable documents.
//
// FromPF force=false converges without destroying (pf values fill and overwrite, missing pf values preserve); force=true is fully authoritative (missing pf values clear via ClearExt).
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

// Removed is the FieldChange display value of a field ClearExt dropped.
const Removed = "removed"

// ClearExt zeroes an external field the projectfile does not declare; empty slices and maps count as absent.
func ClearExt[T any](force bool, field *T) string {
	v := reflect.ValueOf(field).Elem()
	if !force || v.IsZero() || ((v.Kind() == reflect.Slice || v.Kind() == reflect.Map) && v.Len() == 0) {
		return ""
	}
	v.SetZero()
	return Removed
}
