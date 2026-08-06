// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package fragments

// fragmentView is the template data for one assembled document. The
// template renders the SPDX header, the H1 Title, the Project section, then
// one section per parent. Each own fragment carries its demoted H3 title;
// each inherited section carries its parent's H3 entries in one block, under
// a heading that names the parent and the version it was read at.
type fragmentView struct {
	REUSEHeader      string
	Title            string
	HasProject       bool
	ProjectHeading   string
	ProjectFragments []Fragment
	Inherited        []inheritedCopy
}
