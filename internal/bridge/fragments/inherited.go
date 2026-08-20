// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package fragments

import (
	"sort"
)

// inheritedCopy is one parent's fetched document: where it came from, which
// version, and the entry blocks to nest under this project's document.
type inheritedCopy struct {
	Name     string // parent project name (owner/repo from its URL)
	Title    string // the parent's identity.title, empty when it publishes none
	URL      string // parent forge repository
	Ref      string // release tag the copy was read at, empty for a branch read
	Commit   string // commit the copy was read at
	Document string // parent file the copy came from (FEATURES.md, …)
	SPDX     string // the parent's REUSE header, kept with the text it licenses
	Body     string // H3 entry blocks, ready to nest
	Lang     string // language the Body is translated into; empty when canonical
}

// Heading is the H2 the copy renders under. It names the parent only: a
// version here rewrites every child document on every parent release while
// the inherited list itself rarely changes. Which version was read stays in
// Ref/Commit, for the logs.
func (c inheritedCopy) Heading() string {
	return "## Inherited from " + c.displayName()
}

// displayName is what a heading calls the parent. The parent's own title is
// preferred because a heading is prose a person reads, and a title is written
// as prose ("B19/Ubuntu") where owner/repo is a path ("b19/ubuntu") — which
// reads as a typo to both people and prose linters.
//
// owner/repo remains the fallback: it is derived from the URL, so a parent that
// publishes no title still names itself.
func (c inheritedCopy) displayName() string {
	if c.Title != "" {
		return c.Title
	}
	return c.Name
}

// orderedCopies flattens the keyed copies into the order they render, sorted by
// parent key so the assembled document is byte-identical run to run — the
// property the drift gate compares against.
func orderedCopies(copies map[string]inheritedCopy) []inheritedCopy {
	keys := make([]string, 0, len(copies))
	for k := range copies {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := make([]inheritedCopy, 0, len(keys))
	for _, k := range keys {
		out = append(out, copies[k])
	}
	return out
}

// shortCommit abbreviates a commit for display, matching git's default width.
func shortCommit(commit string) string {
	if len(commit) > 7 {
		return commit[:7]
	}
	return commit
}
