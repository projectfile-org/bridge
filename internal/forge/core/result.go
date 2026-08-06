// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core

import "fmt"

// FieldChange describes a single field the push command touched (or would
// touch in dry-run). Mirrors sync/core/FieldChange so the two log surfaces
// read alike — same shape, same Trunc convention, different verb.
type FieldChange struct {
	Field   string // "description" | "homepage" | "topics"
	Value   string // truncated value as it would land on the forge
	Source  string // pf-side origin: "identity.summary", "keywords", ...
	Skipped bool   // true when the field was deliberately not pushed
	Reason  string // when Skipped is true, the human-readable cause
}

// RepoResult collects every per-field decision for one repository entry.
// One of Skipped, Failed, UpToDate, or Changes-is-non-empty is true after
// the algorithm finishes processing the entry.
type RepoResult struct {
	URL      string // exact repositories[].url string
	Host     string // lowercased host extracted from URL
	Kind     string // hostmatch.Kind ("" when unknown — Skipped is true)
	UpToDate bool   // diff was empty; no API call beyond Fetch
	Skipped  bool   // entire repo skipped (archive role, opt-out, no token, ...)
	Reason   string // when Skipped is true, the human-readable cause
	Failed   bool   // Fetch or Apply returned an error
	Error    string // when Failed is true
	Changes  []FieldChange
}

// PushResult is the unified outcome of a `forge push` invocation across
// every (non-archive) repository entry. Per-repo detail lives in Repos;
// summary stats are computed on demand by Format.
type PushResult struct {
	DryRun bool
	Repos  []RepoResult
}

// Touched returns the number of repos whose state would change.
func (r *PushResult) Touched() int {
	n := 0
	for _, rp := range r.Repos {
		if !rp.Skipped && !rp.Failed && !rp.UpToDate && len(rp.Changes) > 0 {
			n++
		}
	}
	return n
}

// Format renders a one-line summary suitable as the command's last log line.
func (r *PushResult) Format() string {
	verb := "pushed"
	if r.DryRun {
		verb = "would push"
	}
	return fmt.Sprintf("%s changes to %d/%d repo(s)", verb, r.Touched(), len(r.Repos))
}

// Trunc caps display strings used in FieldChange values. Same 60-char limit
// the sync package uses — keeps multi-repo runs scannable in a 100-col terminal.
func Trunc(s string) string {
	const maxLen = 60
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
