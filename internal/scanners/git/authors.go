// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package git

import (
	"projectfile.org/projectfile/bridge/internal/scanners/core"
	"projectfile.org/projectfile/bridge/internal/source"
)

// authorsScanner extracts people from git commit history. Each unique commit
// author (deduplicated by email, with .mailmap applied automatically) becomes
// a PersonEntry with role ["author"].
type authorsScanner struct{}

func (authorsScanner) Name() string { return "git-authors" }

func (authorsScanner) Detect(root string) bool { return detectGit(root) }

func (authorsScanner) Scan(root string) (*source.Partial, []core.Hit, error) {
	p := &source.Partial{}
	people := collectAuthors(root)
	if len(people) > 0 {
		p.People = people
		hits := make([]core.Hit, len(people))
		for i, person := range people {
			hits[i] = core.Hit{Source: "git-authors", Field: "people:" + displayName(person)}
		}
		return p, hits, nil
	}
	return p, nil, nil
}
