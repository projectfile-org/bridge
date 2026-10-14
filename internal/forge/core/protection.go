// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// Protector is the optional Client surface for branch and tag protection; a driver without it reports protection unsupported.
type Protector interface {
	// Login returns the account the request token authenticates as.
	Login(ctx context.Context) (string, error)
	// FetchProtection returns every branch and tag rule on the repository, owned or not.
	FetchProtection(ctx context.Context, owner, repo string) (Protection, error)
	// ApplyProtection creates each rule with a zero ID and replaces each rule with a non-zero one.
	ApplyProtection(ctx context.Context, owner, repo string, patch Protection) error
}

// BranchRule is one protected branch pattern, identified by Pattern.
type BranchRule struct {
	ID            int64    // forge rule id, 0 when the forge keys rules by pattern or the rule is new
	Pattern       string   // branch name or glob the rule covers
	Locked        bool     // force push and deletion are refused
	Pushers       []string // nil lets anyone with write access push, empty lets nobody
	StatusChecks  []string // status contexts a pull request merge waits for
	SignedCommits bool     // unsigned commits are refused
}

// TagRule is one protected tag pattern, identified by Pattern.
type TagRule struct {
	ID      int64    // forge rule id, 0 for a new rule
	Pattern string   // tag glob the rule covers
	Pushers []string // accounts allowed to create, move or delete a matching tag
}

// Protection is the protection state of one repository.
type Protection struct {
	Branches        []BranchRule
	Tags            []TagRule
	FastForwardOnly bool // pull requests merge fast-forward only; a desired false leaves the setting alone
}

// IsEmpty reports whether the protection carries nothing to apply.
func (p Protection) IsEmpty() bool {
	return len(p.Branches) == 0 && len(p.Tags) == 0 && !p.FastForwardOnly
}

// Summary renders the rule patterns for the per-field log row.
func (p Protection) Summary() string {
	parts := []string{}
	for _, b := range p.Branches {
		parts = append(parts, "branch "+b.Pattern)
	}
	for _, t := range p.Tags {
		parts = append(parts, "tag "+t.Pattern)
	}
	if p.FastForwardOnly {
		parts = append(parts, "fast-forward-only")
	}
	return strings.Join(parts, ", ")
}

// protectionPlan is the forge-neutral protection a document declares through org.projectfile.release.
type protectionPlan struct {
	stable   string   // release.stable-branch
	branches []string // release.trunk then release.branches, deduplicated, stable excluded
	checks   []string // status contexts of the CI goals a push runs
	tag      string   // tag glob derived from release.tag-format
}

// buildProtectionPlan reads the release intent; a nil plan means the document declares no branch to protect.
func buildProtectionPlan(pf *projectfile.Document) (*protectionPlan, error) {
	rel, err := pfmodel.GetReleaseExtension(pf)
	if err != nil {
		return nil, fmt.Errorf("read release extension: %w", err)
	}
	if rel == nil || (rel.StableBranch == "" && rel.Trunk == "" && len(rel.Branches) == 0) {
		genlog.Debug("forge push: no release branches declared", "extension", pfmodel.ReleaseExtensionNS)
		return nil, nil
	}
	plan := &protectionPlan{stable: rel.StableBranch, tag: tagGlob(rel.TagFormat)}
	seen := map[string]bool{"": true, rel.StableBranch: true}
	candidates := []string{rel.Trunk}
	for _, b := range rel.Branches {
		candidates = append(candidates, b.Pattern)
	}
	for _, name := range candidates {
		if !seen[name] {
			seen[name] = true
			plan.branches = append(plan.branches, name)
		}
	}
	for _, goal := range pfmodel.PushGoals(pf) {
		plan.checks = append(plan.checks, goal+" / "+goal+" (*)")
	}
	genlog.Debug("forge push: protection plan", "stable", plan.stable, "branches", plan.branches, "checks", plan.checks, "tag", plan.tag)
	return plan, nil
}

// tagGlob turns a tag-format such as v${version} into the glob v*.
func tagGlob(format string) string {
	if format == "" {
		return "*"
	}
	return strings.ReplaceAll(format, "${version}", "*")
}

// desired returns the protection for a repository: the origin gets the full table, a mirror only history guards.
func (p *protectionPlan) desired(origin bool, login string) Protection {
	out := Protection{FastForwardOnly: origin && p.stable != ""}
	if p.stable != "" {
		rule := BranchRule{Pattern: p.stable, Locked: true, SignedCommits: true}
		if origin {
			rule.Pushers = []string{login}
			rule.StatusChecks = p.checks
		}
		out.Branches = append(out.Branches, rule)
	}
	for _, name := range p.branches {
		out.Branches = append(out.Branches, BranchRule{Pattern: name, Locked: true})
	}
	if origin {
		out.Tags = []TagRule{{Pattern: p.tag, Pushers: []string{login}}}
	}
	return out
}

// DiffProtection returns the desired rules that differ from current, each carrying the current rule's ID; rules not desired stay untouched.
func DiffProtection(current, desired Protection) Protection {
	patch := Protection{FastForwardOnly: desired.FastForwardOnly && !current.FastForwardOnly}
	for _, want := range desired.Branches {
		have, found := BranchRule{}, false
		for _, b := range current.Branches {
			if b.Pattern == want.Pattern {
				have, found = b, true
				break
			}
		}
		if found && have.Locked == want.Locked && have.SignedCommits == want.SignedCommits &&
			namesEqual(have.Pushers, want.Pushers) && namesEqual(have.StatusChecks, want.StatusChecks) {
			continue
		}
		want.ID = have.ID
		genlog.Debug("forge push: branch rule differs", "pattern", want.Pattern, "exists", found)
		patch.Branches = append(patch.Branches, want)
	}
	for _, want := range desired.Tags {
		have, found := TagRule{}, false
		for _, t := range current.Tags {
			if t.Pattern == want.Pattern {
				have, found = t, true
				break
			}
		}
		if found && namesEqual(have.Pushers, want.Pushers) {
			continue
		}
		want.ID = have.ID
		genlog.Debug("forge push: tag rule differs", "pattern", want.Pattern, "exists", found)
		patch.Tags = append(patch.Tags, want)
	}
	return patch
}

// namesEqual compares two name sets ignoring order, keeping nil (anyone) apart from empty (nobody).
func namesEqual(a, b []string) bool {
	if (a == nil) != (b == nil) || len(a) != len(b) {
		return false
	}
	ac := append([]string(nil), a...)
	bc := append([]string(nil), b...)
	sort.Strings(ac)
	sort.Strings(bc)
	for i := range ac {
		if ac[i] != bc[i] {
			return false
		}
	}
	return true
}

// protectionPatch fetches a repository's protection and returns the rules to apply, nil when it already conforms.
func protectionPatch(
	ctx context.Context,
	driver Client,
	owner, name string,
	plan *protectionPlan,
	role string,
	allow func(string) bool,
) (*Protection, *FieldChange, error) {
	if plan == nil || !allow(FieldProtection) {
		genlog.Debug("forge push: protection not managed", "repo", owner+"/"+name, "declared", plan != nil)
		return nil, nil, nil
	}
	guard, ok := driver.(Protector)
	if !ok {
		genlog.Warn("forge push: protection unsupported", "kind", driver.Kind(), "repo", owner+"/"+name)
		return nil, &FieldChange{Field: FieldProtection, Skipped: true, Reason: "forge:" + driver.Kind() + "-no-protection"}, nil
	}
	origin := role == projectfile.RepositoryRoleOrigin
	login := ""
	if origin {
		var err error
		if login, err = guard.Login(ctx); err != nil {
			return nil, nil, fmt.Errorf("read token account: %w", err)
		}
	}
	current, err := guard.FetchProtection(ctx, owner, name)
	if err != nil {
		return nil, nil, err
	}
	patch := DiffProtection(current, plan.desired(origin, login))
	genlog.Debug("forge push: protection diff", "repo", owner+"/"+name, "origin", origin, "login", login, "changes", patch.Summary())
	if patch.IsEmpty() {
		return nil, nil, nil
	}
	return &patch, &FieldChange{
		Field:  FieldProtection,
		Value:  Trunc(patch.Summary()),
		Source: "release.stable-branch, release.trunk, release.branches",
	}, nil
}
