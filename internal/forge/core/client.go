// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package core contains the forge-agnostic push algorithm and the Client
// interface every forge driver implements. Mirrors the sync/core split:
// per-format/per-forge specifics live under drivers/, the algorithm itself
// stays in one place.
package core

import (
	"context"
	"reflect"
	"sort"
	"strings"
)

// Client is the surface every forge driver implements. Owner() does the
// URL-parsing per-forge because path conventions diverge (GitLab nests
// groups, GitHub doesn't); Fetch reads current state; Apply pushes only the
// fields the diff selected.
type Client interface {
	// Kind returns the forge family identifier ("github", "gitlab",
	// "forgejo"). Matches hostmatch.Kind values; used in log lines and to
	// key the per-kind token env var.
	Kind() string

	// Owner parses repoURL into the (owner, repo) tuple the forge API
	// expects. For GitLab this is the URL-encoded "group/subgroup/project"
	// string concatenation; for GitHub/Forgejo it's the bare "owner/repo".
	Owner(repoURL string) (owner, repo string, err error)

	// Fetch returns the repository's current metadata snapshot. Implementations
	// MUST attempt the request once and surface transport / 5xx errors so the
	// algorithm can skip-with-warning instead of mass-aborting.
	Fetch(ctx context.Context, owner, repo string) (Snapshot, error)

	// Apply pushes the patch to the forge. Implementations decide whether
	// description/homepage and topics need separate HTTP calls (GitHub does;
	// GitLab fits in one PUT).
	Apply(ctx context.Context, owner, repo string, patch Patch) error
}

// Snapshot is the field set pf-cli reads & writes. Three fields cover what
// the user can edit in every forge's web UI; adding a fourth field means
// touching every driver, so this set is deliberately small.
type Snapshot struct {
	Description string
	Homepage    string
	Topics      []string
}

// Patch carries the desired-state delta. The pointer-or-nil convention
// distinguishes "field absent from this patch" (nil → leave unchanged) from
// "clear the field" (non-nil empty value). Same idiom as the GitHub Go SDK.
type Patch struct {
	Description *string
	Homepage    *string
	Topics      *[]string
}

// IsEmpty reports whether the patch would mutate nothing. Used by the
// algorithm to decide whether to log "(up to date)" and skip the HTTP call.
func (p Patch) IsEmpty() bool {
	return p.Description == nil && p.Homepage == nil && p.Topics == nil
}

// Diff produces the minimal patch that transforms current into desired.
// "Minimal" means: a field whose current and desired values already match
// is not included in the patch — that's where idempotency comes from. Topic
// comparison is order-insensitive (GitHub returns them in storage order,
// not the order they were last set; treating order as significant would
// produce churn on every run).
func Diff(current, desired Snapshot) Patch {
	var patch Patch
	if current.Description != desired.Description {
		v := desired.Description
		patch.Description = &v
	}
	if current.Homepage != desired.Homepage {
		v := desired.Homepage
		patch.Homepage = &v
	}
	if !topicsEqual(current.Topics, desired.Topics) {
		v := append([]string(nil), desired.Topics...)
		patch.Topics = &v
	}
	return patch
}

// topicsEqual compares two topic lists ignoring order. Both forges store
// topics as a set; their list serialisation is incidental.
func topicsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	ac := append([]string(nil), a...)
	bc := append([]string(nil), b...)
	sort.Strings(ac)
	sort.Strings(bc)
	return reflect.DeepEqual(ac, bc)
}

// LowerTopics normalises a topic slice the way every forge does on ingest:
// trim, lowercase, drop empties, dedupe-preserving-order. Callers run their
// pf-side keywords through this before Diff so a casing-only delta doesn't
// show up as a change every run.
func LowerTopics(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, t := range in {
		t = strings.ToLower(strings.TrimSpace(t))
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		out = append(out, t)
	}
	return out
}
