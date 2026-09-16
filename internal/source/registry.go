// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package source

import (
	"fmt"
	"sync"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
)

type LocalizedString struct {
	Bare  string
	Langs map[string]string
}

type Partial struct {
	Namespace    *string
	Name         *string
	Title        *LocalizedString
	Summary      *LocalizedString
	Version      *string
	License      *string
	Kind         *string
	People       []PersonEntry
	Keywords     []string
	Repositories []projectfile.Repository
	Links        []projectfile.Link
	Stack        []string
	// ISO-8601 date string (YYYY-MM-DD), matching projectfile.Identity's
	// raw-string convention. Populated by the git scanner from the
	// root-commit date.
	Created *string
	// Modified is the latest-commit (HEAD) date in YYYY-MM-DD form, populated
	// by the git scanner. Gap-fill only: an explicit identity.modified in the
	// document is never overwritten (the apply sites guard on empty), so a
	// curated value survives even though HEAD's date shifts every commit.
	Modified *string
}

type PersonEntry struct {
	FamilyNames string
	GivenNames  string
	Name        string
	Email       string
	Orcid       string
	Roles       []string
	URL         string
	// Affiliation is the institution / employer string. Populated today by
	// the CFF extractor (cff.PersonEntity.Affiliation) and by the user-
	// identity auto-attach (source.ApplyUserIdentity). npm and composer have
	// no equivalent field — those extractors leave it blank.
	Affiliation string
}

type Source interface {
	Name() string
	Filename() string
	Detect(dir string) bool
	Extract(dir string) (*Partial, error)
}

// repoTypeGit is the VCS type every source emits for repository entries.
const repoTypeGit = "git"

var (
	registryMu sync.Once
	registry   []Source
)

func Register(s Source) {
	registry = append(registry, s)
}

func Registry() []Source {
	registryMu.Do(func() {
		if len(registry) == 0 {
			Register(CFFSource{})
			Register(NPMSource{})
			Register(ComposerSource{})
			Register(ShardSource{})
		}
	})
	return registry
}

func Discover(dir string) []Source {
	var found []Source
	for _, s := range Registry() {
		if s.Detect(dir) {
			found = append(found, s)
		}
	}
	return found
}

func ExtractAll(dir string) (*Partial, []string, error) {
	sources := Discover(dir)
	if len(sources) == 0 {
		return &Partial{}, nil, nil
	}

	merged := &Partial{}
	var sourceNames []string

	for _, s := range sources {
		p, err := s.Extract(dir)
		if err != nil {
			return nil, nil, fmt.Errorf("extract from %s: %w", s.Name(), err)
		}
		sourceNames = append(sourceNames, s.Name())
		merged = MergePartials(merged, p)
	}

	return merged, sourceNames, nil
}

func MergePartials(base, overlay *Partial) *Partial {
	result := *base

	if overlay.Namespace != nil && result.Namespace == nil {
		result.Namespace = overlay.Namespace
	}
	if overlay.Name != nil && result.Name == nil {
		result.Name = overlay.Name
	}
	if overlay.Title != nil && result.Title == nil {
		result.Title = overlay.Title
	}
	if overlay.Summary != nil && result.Summary == nil {
		result.Summary = overlay.Summary
	}
	if overlay.Version != nil && result.Version == nil {
		result.Version = overlay.Version
	}
	if overlay.License != nil && result.License == nil {
		result.License = overlay.License
	}
	if overlay.Kind != nil && result.Kind == nil {
		result.Kind = overlay.Kind
	}
	if len(overlay.People) > 0 && len(result.People) == 0 {
		result.People = overlay.People
	}
	if len(overlay.Keywords) > 0 && len(result.Keywords) == 0 {
		result.Keywords = overlay.Keywords
	}
	// Repositories: existing list wins as a whole when present; otherwise
	// adopt the overlay. The first source to declare a repository wins —
	// usually that's the git scanner producing the canonical clone URL.
	if len(result.Repositories) == 0 && len(overlay.Repositories) > 0 {
		result.Repositories = overlay.Repositories
	}
	// Links: gap-fill by (type, url). Existing entries keep their position;
	// only never-seen (type, url) pairs from the overlay are appended.
	for _, l := range overlay.Links {
		if !hasLink(result.Links, l) {
			result.Links = append(result.Links, l)
		}
	}
	if len(overlay.Stack) > 0 && len(result.Stack) == 0 {
		result.Stack = overlay.Stack
	}
	if overlay.Created != nil && result.Created == nil {
		result.Created = overlay.Created
	}
	if overlay.Modified != nil && result.Modified == nil {
		result.Modified = overlay.Modified
	}

	return &result
}

func StringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// hasLink returns true when links already contains an entry with the same
// (type, url) pair — used by MergePartials to keep links dedup'd across
// overlay applications without forcing each source to know what others
// produced.
func hasLink(links []projectfile.Link, want projectfile.Link) bool {
	for _, l := range links {
		if l.Type == want.Type && l.URL == want.URL {
			return true
		}
	}
	return false
}
