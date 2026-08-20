// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package fragments

// fragmentView is the template data for one assembled document. The
// template renders the SPDX header, the H1 Title, the Project section, then
// one section per parent. Each own fragment carries its demoted H3 title;
// each inherited section carries its parent's H3 entries in one block, under
// a heading that names the parent. Title, ProjectHeading and each inherited
// Heading arrive pre-localized for the render language — the template stays
// structural for every language.
type fragmentView struct {
	REUSEHeader      string
	Title            string
	HasProject       bool
	ProjectHeading   string
	ProjectFragments []Fragment
	Inherited        []inheritedEntry
}

// inheritedEntry is one parent section in the template's shape: the heading
// (pre-localized) and the entry blocks, nested verbatim.
type inheritedEntry struct {
	Heading string
	Body    string
}

// localizedInherited adapts the cached copies for one render language,
// resolving each section heading through the strings table.
func localizedInherited(copies []inheritedCopy, lang string) []inheritedEntry {
	out := make([]inheritedEntry, 0, len(copies))
	for _, c := range copies {
		out = append(out, inheritedEntry{
			Heading: localizedInheritedHeading(c, lang),
			Body:    c.Body,
		})
	}
	return out
}
