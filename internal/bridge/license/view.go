// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package license

// compoundView feeds LICENSE.tmpl when the SPDX expression is compound;
// single-term expressions bypass the template entirely (the body IS the
// SPDX text after substitution).
type compoundView struct {
	Expression  string
	Conjunction string // "OR" (disjunctive) or "AND" (conjunctive)
	Terms       []string
	Holders     []string
	Year        int
}
