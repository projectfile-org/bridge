// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package forgejo

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"projectfile.org/projectfile/bridge/v4/internal/forge/core"
)

// testOwner is the account the fake forge authenticates.
const testOwner = "owner"

func TestProtectionRoundTrip(t *testing.T) {
	calls := map[string]map[string]any{}
	mux := http.NewServeMux()
	mux.HandleFunc("/user", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"login":"owner"}`))
	})
	mux.HandleFunc("/repos/me/proj/branch_protections", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			calls["POST "+body["rule_name"].(string)] = body
		}
		_, _ = w.Write([]byte(`[{"rule_name":"main","enable_push":true,"enable_push_whitelist":true,"push_whitelist_usernames":["owner"],"enable_status_check":false,"require_signed_commits":false}]`))
	})
	mux.HandleFunc("/repos/me/proj/branch_protections/", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		calls[r.Method+" "+r.URL.EscapedPath()] = body
		_, _ = w.Write([]byte(`{}`))
	})
	mux.HandleFunc("/repos/me/proj/tag_protections", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			calls["POST tag"] = body
		}
		_, _ = w.Write([]byte(`[]`))
	})
	mux.HandleFunc("/repos/me/proj", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		calls[r.Method+" repo"] = body
		_, _ = w.Write([]byte(`{"allow_merge_commits":true,"default_merge_style":"merge"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	d := newTestDriver(srv)
	ctx := context.Background()

	if login, err := d.Login(ctx); err != nil || login != testOwner {
		t.Fatalf("Login: %q %v", login, err)
	}
	got, err := d.FetchProtection(ctx, "me", "proj")
	if err != nil {
		t.Fatalf("FetchProtection: %v", err)
	}
	want := core.Protection{Branches: []core.BranchRule{{Pattern: "main", Locked: true, Pushers: []string{testOwner}}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fetch:\n got %+v\nwant %+v", got, want)
	}

	patch := core.Protection{
		FastForwardOnly: true,
		Branches: []core.BranchRule{
			{Pattern: "main", Locked: true, Pushers: []string{testOwner}, StatusChecks: []string{"published / published (*)"}, SignedCommits: true},
			{Pattern: "release/*", Locked: true},
		},
		Tags: []core.TagRule{{Pattern: "*", Pushers: []string{testOwner}}},
	}
	if err := d.ApplyProtection(ctx, "me", "proj", patch); err != nil {
		t.Fatalf("ApplyProtection: %v", err)
	}
	if b := calls["PATCH /repos/me/proj/branch_protections/main"]; b["require_signed_commits"] != true || b["enable_status_check"] != true {
		t.Fatalf("stable rule not patched: %v", calls)
	}
	if b := calls["POST release/*"]; b["enable_push"] != true || b["enable_push_whitelist"] != false {
		t.Fatalf("release rule must be created open to writers: %v", calls)
	}
	if b := calls["POST tag"]; b["name_pattern"] != "*" {
		t.Fatalf("tag rule not created: %v", calls)
	}
	if b := calls["PATCH repo"]; b["default_merge_style"] != fastForwardOnly || b["allow_merge_commits"] != false {
		t.Fatalf("merge style not set: %v", calls)
	}
}
