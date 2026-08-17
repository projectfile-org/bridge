// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package llm

import (
	"sort"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// llmView is the LLM.md template data for one render. Every enum token
// (Attitude, Autonomy, Rows[].Stance, ContentSignals entries) is data; its
// prose meaning lives in the template's {{define}} blocks, one per language —
// Go decides WHICH tokens exist, the template says what they mean.
type llmView struct {
	ProjectName      string
	Statement        string // "" omits the statement block
	PolicyURL        string // "" omits the "Full policy" line
	Attitude         string
	Autonomy         string
	DiscloseRequired bool
	DiscloseTrailer  string // "" keeps the disclosure prose generic
	Rows             []activityRow
	ContentSignals   []string
	ContactEmail     string // "" falls back to the SupportFile pointer
	SupportFile      string
}

// activityRow is one row of the "only overrides reach the table" list.
type activityRow struct {
	Activity string // stable key, e.g. "pull-requests"; unknown keys render raw
	Stance   string // token from the same enum as Attitude
}

// llmActivityOrder is the canonical declared order for known activity keys,
// taken from the illustrative shape (spec/shapes/org.projectfile.llm.yaml) —
// so two renders of the same document never reorder around Go map iteration.
// Unknown keys sort alphabetically after these (Determinism, llm-generator.md).
var llmActivityOrder = []string{
	"translations", "security-reports", "pull-requests", "commit-messages",
	"discussions", "bug-reports", "documentation", "code-comments", "grammar-tools",
}

// llmKnownStances is the one closed enum `attitude` and every activity share
// (spec change #1 — no attitude/activity derivation table, ever).
var llmKnownStances = map[string]bool{
	"encouraged": true, "allowed": true, "neutral": true,
	"discouraged": true, "prohibited": true,
}

// llmKnownAutonomy is the closed enum for the autonomy axis (spec change #2).
var llmKnownAutonomy = map[string]bool{"any": true, "assisted": true, "none": true}

// buildActivityRows resolves the table rows: only an activity whose declared
// stance DIFFERS from Attitude reaches the table, known keys in
// llmActivityOrder first, then any unknown key alphabetically. A project that
// spelled out every activity identically to its attitude gets no table row at
// all — a row is noise unless it says something the reader could not already
// assume.
func buildActivityRows(ext *pfmodel.LLMExtension) []activityRow {
	if ext == nil || len(ext.Activities) == 0 {
		return nil
	}
	var rows []activityRow
	seen := make(map[string]bool, len(ext.Activities))
	for _, key := range llmActivityOrder {
		v, ok := ext.Activities[key]
		if !ok {
			continue
		}
		seen[key] = true
		if v != ext.Attitude {
			rows = append(rows, activityRow{Activity: key, Stance: v})
		}
	}
	unknown := make([]string, 0, len(ext.Activities)-len(seen))
	for key := range ext.Activities {
		if !seen[key] {
			unknown = append(unknown, key)
		}
	}
	sort.Strings(unknown)
	for _, key := range unknown {
		if v := ext.Activities[key]; v != ext.Attitude {
			rows = append(rows, activityRow{Activity: key, Stance: v})
		}
	}
	return rows
}

// dedupSignals drops repeated content-signals values while keeping first-seen
// (declared) order — a sequence field's one invariant worth preserving.
func dedupSignals(signals []string) []string {
	if len(signals) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(signals))
	out := make([]string, 0, len(signals))
	for _, s := range signals {
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// warnUnknownStances logs — but never fails on — any attitude/autonomy/
// activity value outside its closed vocabulary. Refusing to render is the
// wrong failure here: a reader would lose the whole policy over one typo, so
// the template's own {{else}} branch still renders the raw token; this only
// leaves the maintainer a trace to act on.
func warnUnknownStances(ext *pfmodel.LLMExtension) {
	if ext.Attitude != "" && !llmKnownStances[ext.Attitude] {
		genlog.Warn("unrecognised org.projectfile.llm attitude value", "value", ext.Attitude)
	}
	if !llmKnownAutonomy[ext.Autonomy] {
		genlog.Warn("unrecognised org.projectfile.llm autonomy value", "value", ext.Autonomy)
	}
	for key, v := range ext.Activities {
		if !llmKnownStances[v] {
			genlog.Warn("unrecognised org.projectfile.llm activity stance", "activity", key, "value", v)
		}
	}
}
