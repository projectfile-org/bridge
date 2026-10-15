// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package github

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"projectfile.org/projectfile/bridge/v4/internal/forge/core"
)

func TestProtectionRoundTrip(t *testing.T) {
	writes := map[string]ruleset{}
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/me/proj/rulesets", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var body ruleset
			_ = json.NewDecoder(r.Body).Decode(&body)
			writes["POST "+body.Name] = body
			_, _ = w.Write([]byte(`{}`))
			return
		}
		_, _ = w.Write([]byte(`[{"id":1,"name":"main","target":"branch","source_type":"Repository"},{"id":2,"name":"org","target":"branch","source_type":"Organization"}]`))
	})
	mux.HandleFunc("/repos/me/proj/rulesets/1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			var body ruleset
			_ = json.NewDecoder(r.Body).Decode(&body)
			writes["PUT main"] = body
		}
		_, _ = w.Write([]byte(`{"id":1,"name":"main","target":"branch","enforcement":"active","conditions":{"ref_name":{"include":["refs/heads/main"]}},"rules":[{"type":"deletion"},{"type":"non_fast_forward"}]}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	d := newTestDriver(srv)
	ctx := context.Background()

	got, err := d.FetchProtection(ctx, "me", "proj")
	if err != nil {
		t.Fatalf("FetchProtection: %v", err)
	}
	if len(got.Branches) != 1 || got.Branches[0].ID != 1 || !got.Branches[0].Locked || got.Branches[0].SignedCommits {
		t.Fatalf("fetch: %+v", got)
	}
	patch := core.Protection{Branches: []core.BranchRule{
		{ID: 1, Pattern: "main", Locked: true, SignedCommits: true},
		{Pattern: "dev", Locked: true},
	}}
	if err := d.ApplyProtection(ctx, "me", "proj", patch); err != nil {
		t.Fatalf("ApplyProtection: %v", err)
	}
	if rs := writes["PUT main"]; len(rs.Rules) != 3 || rs.Rules[2].Type != "required_signatures" {
		t.Fatalf("stable ruleset not replaced: %+v", writes)
	}
	if rs := writes["POST dev"]; len(rs.Rules) != 2 || rs.Conditions.RefName.Include[0] != "refs/heads/dev" {
		t.Fatalf("trunk ruleset not created: %+v", writes)
	}
	err = d.ApplyProtection(ctx, "me", "proj", core.Protection{Tags: []core.TagRule{{Pattern: "*"}}})
	if !errors.Is(err, errUnsupported) {
		t.Fatalf("tag rule must be refused, got %v", err)
	}
}
