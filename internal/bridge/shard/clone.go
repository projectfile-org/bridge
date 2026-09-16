// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package shard

// Clone returns a deep copy of the document including the raw round-trip view.
func (d *Document) Clone() *Document {
	if d == nil {
		return nil
	}
	cp := *d
	cp.Authors = cloneStrings(d.Authors)
	cp.Rest = d.Rest.Clone()
	return &cp
}

// cloneStrings copies a string slice preserving nil-ness for round-trip checks.
func cloneStrings(in []string) []string {
	if in == nil {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}
