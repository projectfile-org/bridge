// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package gitlab

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"projectfile.org/projectfile/bridge/internal/forge/core"
)

func newTestDriver(server *httptest.Server) *driver {
	return &driver{http: server.Client(), userAgent: "pf-cli-test", baseURL: server.URL}
}

func TestOwner_NestedGroup(t *testing.T) {
	d := &driver{}
	o, r, err := d.Owner("https://gitlab.com/grp/sub/proj")
	if err != nil {
		t.Fatalf("Owner: %v", err)
	}
	if o != "grp/sub" || r != "proj" {
		t.Fatalf("got (%q,%q), want (grp/sub, proj)", o, r)
	}
}

func TestOwner_ScpStyle(t *testing.T) {
	d := &driver{}
	o, r, err := d.Owner("git@gitlab.com:me/proj.git")
	if err != nil {
		t.Fatalf("Owner: %v", err)
	}
	if o != "me" || r != "proj" {
		t.Fatalf("got (%q,%q), want (me, proj)", o, r)
	}
}

func TestFetchAndApply(t *testing.T) {
	var (
		gotPath  string
		gotAuth  string
		gotBody  map[string]any
		putCount int
	)
	mux := http.NewServeMux()
	// projects/{id} where {id} is URL-encoded "owner/repo".
	mux.HandleFunc("/projects/", func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"description": "old",
				"topics":      []string{"foo"},
			})
		case http.MethodPut:
			putCount++
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &gotBody)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{}"))
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	d := newTestDriver(srv)
	ctx := core.WithToken(context.Background(), "glpat_test")

	snap, err := d.Fetch(ctx, "owner", "repo")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if snap.Description != "old" {
		t.Fatalf("description mismatch: %q", snap.Description)
	}
	if snap.Homepage != "" {
		t.Fatalf("gitlab should never return homepage: %q", snap.Homepage)
	}
	if !strings.Contains(gotPath, "owner") {
		t.Fatalf("path %q should contain owner segment", gotPath)
	}
	if !strings.HasPrefix(gotAuth, "Bearer ") {
		t.Fatalf("missing bearer auth: %q", gotAuth)
	}

	desc := "new"
	topics := []string{"a", "b"}
	hp := "https://homepage" // explicitly set; should be dropped server-side
	if err := d.Apply(ctx, "owner", "repo", core.Patch{Description: &desc, Topics: &topics, Homepage: &hp}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if putCount != 1 {
		t.Fatalf("expected 1 PUT, got %d", putCount)
	}
	if gotBody["description"] != "new" {
		t.Fatalf("desc not in body: %v", gotBody)
	}
	if _, has := gotBody["homepage"]; has {
		t.Fatalf("homepage should not have been sent: %v", gotBody)
	}
	if _, has := gotBody["website"]; has {
		t.Fatalf("website should not have been sent (that's forgejo's name): %v", gotBody)
	}
}
