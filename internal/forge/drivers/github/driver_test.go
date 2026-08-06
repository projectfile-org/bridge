// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package github

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"projectfile.org/projectfile/bridge/internal/forge/core"
)

func newTestDriver(server *httptest.Server) *driver {
	return &driver{http: server.Client(), userAgent: "pf-cli-test", baseURL: server.URL}
}

func TestOwner(t *testing.T) {
	d := &driver{}
	cases := []struct {
		url, wantO, wantR string
	}{
		{"https://github.com/me/proj", "me", "proj"}, //nolint:goconst // test-fixture repetition is the point of a table-driven test
		{"https://github.com/me/proj/", "me", "proj"},
		{"https://github.com/me/proj.git", "me", "proj"},
	}
	for _, c := range cases {
		o, r, err := d.Owner(c.url)
		if err != nil {
			t.Fatalf("Owner(%q) err: %v", c.url, err)
		}
		if o != c.wantO || r != c.wantR {
			t.Fatalf("Owner(%q) = (%q,%q), want (%q,%q)", c.url, o, r, c.wantO, c.wantR)
		}
	}
}

func TestFetchAndApply(t *testing.T) {
	var (
		gotAuth    string
		gotPatch   map[string]any
		gotTopics  map[string]any
		patchCount int
		topicCount int
	)
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/me/proj", func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"description": "old desc",
				"homepage":    "https://old",
			})
		case http.MethodPatch:
			patchCount++
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &gotPatch)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{}"))
		}
	})
	mux.HandleFunc("/repos/me/proj/topics", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{"names": []string{"a", "b"}})
		case http.MethodPut:
			topicCount++
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &gotTopics)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{}"))
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	d := newTestDriver(srv)
	ctx := core.WithToken(context.Background(), "ghp_test")

	snap, err := d.Fetch(ctx, "me", "proj")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if snap.Description != "old desc" || snap.Homepage != "https://old" {
		t.Fatalf("snapshot mismatch: %+v", snap)
	}
	if !reflect.DeepEqual(snap.Topics, []string{"a", "b"}) {
		t.Fatalf("topics mismatch: %v", snap.Topics)
	}
	if !strings.HasPrefix(gotAuth, "Bearer ") {
		t.Fatalf("missing bearer auth, got %q", gotAuth)
	}

	desc := "new desc"
	hp := "https://new"
	topics := []string{"c", "d"}
	if err := d.Apply(ctx, "me", "proj", core.Patch{Description: &desc, Homepage: &hp, Topics: &topics}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if patchCount != 1 || topicCount != 1 {
		t.Fatalf("patch=%d topic=%d, want 1/1", patchCount, topicCount)
	}
	if gotPatch["description"] != "new desc" || gotPatch["homepage"] != "https://new" {
		t.Fatalf("patch body: %v", gotPatch)
	}
	names, _ := gotTopics["names"].([]any)
	if len(names) != 2 || names[0] != "c" || names[1] != "d" {
		t.Fatalf("topics body: %v", gotTopics)
	}
}

func TestApply_NoTopics(t *testing.T) {
	var topicCount int
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/me/proj", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPatch {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{}"))
		}
	})
	mux.HandleFunc("/repos/me/proj/topics", func(w http.ResponseWriter, _ *http.Request) {
		topicCount++
		w.WriteHeader(http.StatusOK)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	d := newTestDriver(srv)
	desc := "x"
	if err := d.Apply(context.Background(), "me", "proj", core.Patch{Description: &desc}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if topicCount != 0 {
		t.Fatalf("topics endpoint called even though patch had no topics, count=%d", topicCount)
	}
}
