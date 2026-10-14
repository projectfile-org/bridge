// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"projectfile.org/projectfile/bridge/internal/forge/core"
)

// errUnsupported names the protection a repository ruleset cannot carry without a bypass actor.
var errUnsupported = errors.New("github: push allowlists, required checks, tag rules and merge style need bypass actors, which this tool never grants")

// ruleset is the subset of a GitHub repository ruleset the tool reads and writes.
type ruleset struct {
	ID          int64      `json:"id,omitempty"`
	Name        string     `json:"name"`
	Target      string     `json:"target"`
	SourceType  string     `json:"source_type,omitempty"`
	Enforcement string     `json:"enforcement"`
	Conditions  conditions `json:"conditions"`
	Rules       []rule     `json:"rules"`
}

type conditions struct {
	RefName refName `json:"ref_name"`
}

type refName struct {
	Include []string `json:"include"`
	Exclude []string `json:"exclude"`
}

type rule struct {
	Type string `json:"type"`
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
	base := fmt.Sprintf("%s/repos/%s/%s/rulesets", d.apiBase(ctx, owner), owner, repo)
	var list []ruleset
	if err := d.do(ctx, http.MethodGet, base+"?includes_parents=false&per_page=100", nil, &list); err != nil {
		return core.Protection{}, err
	}
	out := core.Protection{}
	for _, summary := range list {
		if summary.Target != "branch" || summary.SourceType != "Repository" {
			genlog.Debug("github ruleset not owned", "repo", owner+"/"+repo, "name", summary.Name, "target", summary.Target, "source", summary.SourceType)
			continue
		}
		var rs ruleset
		if err := d.do(ctx, http.MethodGet, fmt.Sprintf("%s/%d", base, summary.ID), nil, &rs); err != nil {
			return core.Protection{}, err
		}
		types := make([]string, 0, len(rs.Rules))
		for _, r := range rs.Rules {
			types = append(types, r.Type)
		}
		// A ruleset reads as locked only in the exact shape this tool writes for its own name.
		locked := rs.Enforcement == "active" && slices.Equal(rs.Conditions.RefName.Include, []string{"refs/heads/" + rs.Name}) &&
			slices.Contains(types, "deletion") && slices.Contains(types, "non_fast_forward")
		out.Branches = append(out.Branches, core.BranchRule{
			ID:            rs.ID,
			Pattern:       rs.Name,
			Locked:        locked,
			SignedCommits: slices.Contains(types, "required_signatures"),
		})
	}
	genlog.Debug("github protection", "repo", owner+"/"+repo, "rulesets", len(out.Branches))
	return out, nil
}

func (d *driver) ApplyProtection(ctx context.Context, owner, repo string, patch core.Protection) error {
	if len(patch.Tags) > 0 || patch.FastForwardOnly {
		return errUnsupported
	}
	base := fmt.Sprintf("%s/repos/%s/%s/rulesets", d.apiBase(ctx, owner), owner, repo)
	for _, br := range patch.Branches {
		if br.Pushers != nil || len(br.StatusChecks) > 0 {
			return errUnsupported
		}
		body := ruleset{
			Name:        br.Pattern,
			Target:      "branch",
			Enforcement: "active",
			Conditions:  conditions{RefName: refName{Include: []string{"refs/heads/" + br.Pattern}, Exclude: []string{}}},
			Rules:       []rule{{Type: "deletion"}, {Type: "non_fast_forward"}},
		}
		if br.SignedCommits {
			body.Rules = append(body.Rules, rule{Type: "required_signatures"})
		}
		method, target := http.MethodPost, base
		if br.ID != 0 {
			method, target = http.MethodPut, fmt.Sprintf("%s/%d", base, br.ID)
		}
		genlog.Debug("github ruleset", "repo", owner+"/"+repo, "pattern", br.Pattern, "method", method)
		if err := d.do(ctx, method, target, body, nil); err != nil {
			return err
		}
	}
	return nil
}

// The driver applies branch and tag protection.
var _ core.Protector = (*driver)(nil)
