// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package aipolicy

import (
	"sort"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// llmView is the policy template data for one render. Every enum token
// (Attitude, Autonomy, AppliesTo, the row stances, Obligations, Enforcement,
// ContentSignals entries) is data; its prose meaning lives in the template's
// {{define}} blocks, one per language — Go decides WHICH tokens exist, the
// template says what they mean.
type llmView struct {
	ProjectName      string
	Statement        string // "" omits the statement block
	PolicyURL        string // "" omits the "Full policy" line
	Attitude         string
	Autonomy         string
	AppliesTo        string
	DiscloseRequired bool
	DiscloseTrailer  string   // "" keeps the disclosure prose generic
	DiscloseDetails  []string // what the disclosure must carry
	Obligations      []string
	IssueRequired    bool
	ExcludedLabels   []string
	Enforcement      []string // declared order IS the escalation order
	Rows             []activityRow
	ProjectUseRows   []projectUseRow
	ContentSignals   []string
	ContactEmail     string // "" falls back to the SupportFile pointer
	SupportFile      string
}

// activityRow is one row of the "only overrides reach the table" list.
type activityRow struct {
	Activity string // stable key, e.g. "pull-requests"; unknown keys render raw
	Stance   string // token from the same enum as Attitude
}

// projectUseRow is one row of the INTERNAL direction: what the project itself
// does with AI. Every declared entry renders — unlike an activity, there is no
// document-level default it could merely repeat.
type projectUseRow struct {
	Activity string
	Autonomy string // token from the same enum as Autonomy
}

// llmActivityOrder is the canonical declared order for known activity keys,
// taken from the illustrative shape (spec/shapes/org.projectfile.llm.yaml) —
// so two renders of the same document never reorder around Go map iteration.
// Unknown keys sort alphabetically after these (Determinism, ai-policy-generator.md).
// The three families of the shape are kept in order: channel, content, task.
var llmActivityOrder = []string{
	"pull-requests", "commit-messages", "bug-reports", "discussions",
	"code-review", "security-reports",
	"code", "code-comments", "documentation", "translations", "prose",
	"images", "audio", "video",
	"refactoring", "bug-fixing", "tests", "grammar-tools",
	"merges", "releases", "triage",
}

// llmKnownStances is the one closed enum `attitude` and every activity share
// (spec change #1 — no attitude/activity derivation table, ever).
var llmKnownStances = map[string]bool{
	"encouraged": true, "allowed": true, "neutral": true,
	"discouraged": true, "prohibited": true,
}

// llmKnownAutonomy is the closed enum for the autonomy axis (spec change #2),
// shared by the document-level `autonomy` and every project-use entry.
var llmKnownAutonomy = map[string]bool{"any": true, "assisted": true, "none": true}

// llmKnownAppliesTo is the closed enum for who the policy binds.
var llmKnownAppliesTo = map[string]bool{"everyone": true, "contributors": true}

// orderedActivityKeys returns the declared keys of an activity-keyed map in
// render order: known keys in llmActivityOrder first, then unknown keys
// alphabetically. Map iteration order is random, and a random order fails
// --check at random.
func orderedActivityKeys(m map[string]string) []string {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	seen := make(map[string]bool, len(m))
	for _, key := range llmActivityOrder {
		if _, ok := m[key]; ok {
			keys = append(keys, key)
			seen[key] = true
		}
	}
	unknown := make([]string, 0, len(m)-len(seen))
	for key := range m {
		if !seen[key] {
			unknown = append(unknown, key)
		}
	}
	sort.Strings(unknown)
	return append(keys, unknown...)
}

// buildActivityRows resolves the table rows: only an activity whose declared
// stance DIFFERS from Attitude reaches the table. A project that spelled out
// every activity identically to its attitude gets no table row at all — a row
// is noise unless it says something the reader could not already assume.
func buildActivityRows(ext *pfmodel.LLMExtension) []activityRow {
	if ext == nil {
		return nil
	}
	var rows []activityRow
	for _, key := range orderedActivityKeys(ext.Activities) {
		if v := ext.Activities[key]; v != ext.Attitude {
			rows = append(rows, activityRow{Activity: key, Stance: v})
		}
	}
	return rows
}

// buildProjectUseRows resolves the INTERNAL table. No filter: a project that
// says "we merge by hand" is saying it, not repeating a default.
func buildProjectUseRows(ext *pfmodel.LLMExtension) []projectUseRow {
	if ext == nil {
		return nil
	}
	var rows []projectUseRow
	for _, key := range orderedActivityKeys(ext.ProjectUse) {
		rows = append(rows, projectUseRow{Activity: key, Autonomy: ext.ProjectUse[key]})
	}
	return rows
}

// dedupSignals drops repeated sequence values while keeping first-seen
// (declared) order — the one invariant a sequence field carries. Shared by
// content-signals, obligations, disclose-details and enforcement, whose
// declared order is the escalation order a reader walks down.
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

// warnUnknownStances logs — but never fails on — any value outside its closed
// vocabulary. Refusing to render is the wrong failure here: a reader would
// lose the whole policy over one typo, so the template's own {{else}} branch
// still renders the raw token; this only leaves the maintainer a trace to act
// on. Open-vocabulary fields (obligations, enforcement, content-signals,
// activity KEYS) are deliberately not checked — an unknown value there is the
// spec working as designed, not a mistake.
func warnUnknownStances(ext *pfmodel.LLMExtension) {
	if ext.Attitude != "" && !llmKnownStances[ext.Attitude] {
		genlog.Warn("unrecognised org.projectfile.llm attitude value", "value", ext.Attitude)
	}
	if !llmKnownAutonomy[ext.Autonomy] {
		genlog.Warn("unrecognised org.projectfile.llm autonomy value", "value", ext.Autonomy)
	}
	if !llmKnownAppliesTo[ext.AppliesTo] {
		genlog.Warn("unrecognised org.projectfile.llm applies-to value", "value", ext.AppliesTo)
	}
	for key, v := range ext.Activities {
		if !llmKnownStances[v] {
			genlog.Warn("unrecognised org.projectfile.llm activity stance", "activity", key, "value", v)
		}
	}
	for key, v := range ext.ProjectUse {
		if !llmKnownAutonomy[v] {
			genlog.Warn("unrecognised org.projectfile.llm project-use autonomy", "activity", key, "value", v)
		}
	}
}
