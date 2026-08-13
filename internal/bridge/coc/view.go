// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package coc

// cocView is the CODE_OF_CONDUCT.md template data. Covenant names which
// covenant applies (only Contributor Covenant 2.1 ships a template today);
// Scope narrows where it applies (project-and-spaces vs project). ReadmeFile
// is the same-language README rebased relative to this document, so the
// project name can link to it.
type cocView struct {
	ProjectName  string
	ReadmeFile   string
	ContactEmail string
	Covenant     string
	Scope        string
}
