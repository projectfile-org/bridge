// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package forgejo

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"projectfile.org/projectfile/bridge/internal/forge/core"
)

// fastForwardOnly is the Forgejo merge style that never creates a commit.
const fastForwardOnly = "fast-forward-only"

// branchProtection is the subset of a Forgejo branch protection the tool reads and writes.
type branchProtection struct {
	RuleName            string   `json:"rule_name,omitempty"`
	EnablePush          bool     `json:"enable_push"`
	EnablePushWhitelist bool     `json:"enable_push_whitelist"`
	PushWhitelist       []string `json:"push_whitelist_usernames"`
	EnableStatusCheck   bool     `json:"enable_status_check"`
	StatusCheckContexts []string `json:"status_check_contexts"`
	RequireSignedCommit bool     `json:"require_signed_commits"`
}

// tagProtection is the subset of a Forgejo tag protection the tool reads and writes.
type tagProtection struct {
	ID          int64    `json:"id,omitempty"`
	NamePattern string   `json:"name_pattern"`
	Whitelist   []string `json:"whitelist_usernames"`
}

// mergeStyles is the repository merge settings that make a pull request merge fast-forward only.
type mergeStyles struct {
	AllowFastForwardOnly bool   `json:"allow_fast_forward_only_merge"`
	AllowMerge           bool   `json:"allow_merge_commits"`
	AllowRebase          bool   `json:"allow_rebase"`
	AllowRebaseExplicit  bool   `json:"allow_rebase_explicit"`
	AllowSquash          bool   `json:"allow_squash_merge"`
	DefaultMergeStyle    string `json:"default_merge_style"`
}

func (d *driver) Login(ctx context.Context) (string, error) {
	var user struct {
		Login string `json:"login"`
	}
	if err := d.do(ctx, http.MethodGet, d.apiBase(ctx, "")+"/user", nil, &user); err != nil {
		return "", err
	}
	return user.Login, nil
}

func (d *driver) FetchProtection(ctx context.Context, owner, repo string) (core.Protection, error) {
	base := fmt.Sprintf("%s/repos/%s/%s", d.apiBase(ctx, owner), owner, repo)
	var branches []branchProtection
	if err := d.do(ctx, http.MethodGet, base+"/branch_protections", nil, &branches); err != nil {
		return core.Protection{}, err
	}
	var tags []tagProtection
	if err := d.do(ctx, http.MethodGet, base+"/tag_protections", nil, &tags); err != nil {
		return core.Protection{}, err
	}
	var merge mergeStyles
	if err := d.do(ctx, http.MethodGet, base, nil, &merge); err != nil {
		return core.Protection{}, err
	}
	out := core.Protection{FastForwardOnly: merge == mergeStyles{AllowFastForwardOnly: true, DefaultMergeStyle: fastForwardOnly}}
	for _, b := range branches {
		// Forgejo refuses force push and deletion on every protected branch, admins included.
		rule := core.BranchRule{Pattern: b.RuleName, Locked: true, SignedCommits: b.RequireSignedCommit}
		switch {
		case !b.EnablePush:
			rule.Pushers = []string{}
		case b.EnablePushWhitelist:
			rule.Pushers = append([]string{}, b.PushWhitelist...)
		}
		if b.EnableStatusCheck {
			rule.StatusChecks = append([]string{}, b.StatusCheckContexts...)
		}
		out.Branches = append(out.Branches, rule)
	}
	for _, t := range tags {
		out.Tags = append(out.Tags, core.TagRule{ID: t.ID, Pattern: t.NamePattern, Pushers: append([]string{}, t.Whitelist...)})
	}
	genlog.Debug("forgejo protection", "repo", owner+"/"+repo, "branches", len(out.Branches), "tags", len(out.Tags), "fast-forward-only", out.FastForwardOnly)
	return out, nil
}

func (d *driver) ApplyProtection(ctx context.Context, owner, repo string, patch core.Protection) error {
	base := fmt.Sprintf("%s/repos/%s/%s", d.apiBase(ctx, owner), owner, repo)
	existing := map[string]bool{}
	var branches []branchProtection
	if err := d.do(ctx, http.MethodGet, base+"/branch_protections", nil, &branches); err != nil {
		return err
	}
	for _, b := range branches {
		existing[b.RuleName] = true
	}
	for _, rule := range patch.Branches {
		body := branchProtection{
			EnablePush:          rule.Pushers == nil || len(rule.Pushers) > 0,
			EnablePushWhitelist: len(rule.Pushers) > 0,
			PushWhitelist:       append([]string{}, rule.Pushers...),
			EnableStatusCheck:   len(rule.StatusChecks) > 0,
			StatusCheckContexts: append([]string{}, rule.StatusChecks...),
			RequireSignedCommit: rule.SignedCommits,
		}
		method, target := http.MethodPatch, base+"/branch_protections/"+url.PathEscape(rule.Pattern)
		if !existing[rule.Pattern] {
			body.RuleName = rule.Pattern
			method, target = http.MethodPost, base+"/branch_protections"
		}
		genlog.Debug("forgejo branch protection", "repo", owner+"/"+repo, "pattern", rule.Pattern, "method", method)
		if err := d.do(ctx, method, target, body, nil); err != nil {
			return err
		}
	}
	for _, rule := range patch.Tags {
		body := tagProtection{NamePattern: rule.Pattern, Whitelist: append([]string{}, rule.Pushers...)}
		method, target := http.MethodPost, base+"/tag_protections"
		if rule.ID != 0 {
			method, target = http.MethodPatch, fmt.Sprintf("%s/tag_protections/%d", base, rule.ID)
		}
		genlog.Debug("forgejo tag protection", "repo", owner+"/"+repo, "pattern", rule.Pattern, "method", method)
		if err := d.do(ctx, method, target, body, nil); err != nil {
			return err
		}
	}
	if patch.FastForwardOnly {
		genlog.Debug("forgejo merge style", "repo", owner+"/"+repo, "style", fastForwardOnly)
		return d.do(ctx, http.MethodPatch, base, mergeStyles{AllowFastForwardOnly: true, DefaultMergeStyle: fastForwardOnly}, nil)
	}
	return nil
}

// The driver applies branch and tag protection.
var _ core.Protector = (*driver)(nil)
