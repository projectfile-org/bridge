// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package git

import (
	"os/exec"
	"sort"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/source"
)

// collectAuthors enumerates every commit author via `git log --format=%aN%x09%aE`
// (tab-separated to survive names with spaces or commas), deduplicates by
// canonical identity key, and returns a stable-ordered []PersonEntry. Mailmap
// support is free — git log applies .mailmap before %aN/%aE substitution.
//
// Returns a nil slice on any git failure (empty repos, missing git binary).
// Best-effort matches the rest of the git scanner — see Scan's docstring.
func collectAuthors(root string) []source.PersonEntry {
	out, err := run(exec.Command("git", "log", "--format=%aN%x09%aE"), root)
	if err != nil || out == "" {
		return nil
	}

	type bucket struct {
		name     string         // canonical display name (most frequent variant)
		email    string         // canonical email (lowercased)
		nameHits map[string]int // raw → count for canonical-name election
		count    int            // commit count for sort key
	}

	// Bucket commits by lowercased email; empty-email rows bucket by name.
	// Two-pass merge: pass 1 builds the email-keyed map; pass 2 folds in
	// commits that had no email (rare but legal in git history) and merges
	// when an email-keyed bucket shares its canonical name with one of them.
	byEmail := map[string]*bucket{}
	var byName []*bucket

	for _, line := range strings.Split(out, "\n") {
		// `%aN\t%aE` — split on the first tab, not strings.Fields, because
		// display names can contain whitespace.
		name, email, _ := strings.Cut(line, "\t")
		name = strings.TrimSpace(name)
		email = strings.ToLower(strings.TrimSpace(email))
		if name == "" && email == "" {
			continue
		}

		if email != "" {
			b, ok := byEmail[email]
			if !ok {
				b = &bucket{email: email, nameHits: map[string]int{}}
				byEmail[email] = b
			}
			b.count++
			if name != "" {
				b.nameHits[name]++
			}
			continue
		}

		// No email — find or create a name-only bucket.
		var b *bucket
		for _, candidate := range byName {
			if candidate.name == name {
				b = candidate
				break
			}
		}
		if b == nil {
			b = &bucket{name: name, nameHits: map[string]int{name: 0}}
			byName = append(byName, b)
		}
		b.count++
		b.nameHits[name]++
	}

	// Elect canonical name per email bucket (most frequent raw form, ties
	// broken alphabetically for determinism).
	for _, b := range byEmail {
		b.name = electName(b.nameHits)
	}

	// Fold name-only buckets into email-keyed buckets whose canonical name
	// matches — that's the same person committing with and without an email.
	emailByName := map[string]*bucket{}
	for _, b := range byEmail {
		if b.name != "" {
			emailByName[b.name] = b
		}
	}
	var orphans []*bucket
	for _, b := range byName {
		if dest, ok := emailByName[b.name]; ok {
			dest.count += b.count
			continue
		}
		orphans = append(orphans, b)
	}

	// Materialise the final slice. Sort by descending count, then ascending
	// canonical name — a stable order that surfaces the most active author
	// first while staying deterministic across runs.
	all := make([]*bucket, 0, len(byEmail)+len(orphans))
	for _, b := range byEmail {
		all = append(all, b)
	}
	all = append(all, orphans...)
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].count != all[j].count {
			return all[i].count > all[j].count
		}
		return all[i].name < all[j].name
	})

	people := make([]source.PersonEntry, 0, len(all))
	for _, b := range all {
		// Split the flat git author name into projectfile's family/given
		// pair per spec §5.5.5. A bare git author is always a person — git
		// has no concept of an organization committer — so the split-name
		// shape is correct here. Ambiguous (single-token) names produce
		// family-names only and we log so the user knows to review.
		family, given := projectfile.SplitGitName(b.name)
		if projectfile.AmbiguousGitName(b.name) {
			genlog.Info("git author has single-token name; review for organization vs mononym", "name", b.name, "email", b.email)
		}
		people = append(people, source.PersonEntry{
			FamilyNames: family,
			GivenNames:  given,
			Email:       b.email,
			Roles:       []string{"author"},
		})
	}
	return people
}

// electName picks the most frequent raw-name variant in a bucket; ties are
// broken alphabetically so two runs of the scanner always produce the same
// canonical name for the same git history.
func electName(hits map[string]int) string {
	if len(hits) == 0 {
		return ""
	}
	type entry struct {
		name  string
		count int
	}
	flat := make([]entry, 0, len(hits))
	for n, c := range hits {
		flat = append(flat, entry{n, c})
	}
	sort.Slice(flat, func(i, j int) bool {
		if flat[i].count != flat[j].count {
			return flat[i].count > flat[j].count
		}
		return flat[i].name < flat[j].name
	})
	return flat[0].name
}
