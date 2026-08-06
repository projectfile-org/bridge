// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package forgejo

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

func TestFetchAndApply(t *testing.T) {
	var (
		gotAuth   string
		gotPatch  map[string]any
		gotTopics map[string]any
	)
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/me/proj", func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"description": "old",
				"website":     "https://old",
			})
		case http.MethodPatch:
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &gotPatch)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{}"))
		}
	})
	mux.HandleFunc("/repos/me/proj/topics", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{"topics": []string{"foo"}})
		case http.MethodPut:
			body, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(body, &gotTopics)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{}"))
		}
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	d := newTestDriver(srv)
	ctx := core.WithToken(context.Background(), "tok_test")

	snap, err := d.Fetch(ctx, "me", "proj")
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if snap.Description != "old" || snap.Homepage != "https://old" {
		t.Fatalf("snapshot mismatch: %+v", snap)
	}
	if !strings.HasPrefix(gotAuth, "token ") {
		t.Fatalf("forgejo should send 'token <secret>' header, got %q", gotAuth)
	}

	desc := "new"
	hp := "https://new"
	topics := []string{"a"}
	if err := d.Apply(ctx, "me", "proj", core.Patch{Description: &desc, Homepage: &hp, Topics: &topics}); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if gotPatch["description"] != "new" {
		t.Fatalf("desc: %v", gotPatch)
	}
	if gotPatch["website"] != "https://new" {
		t.Fatalf("website (homepage) mapping wrong: %v", gotPatch)
	}
	if _, has := gotTopics["topics"]; !has {
		t.Fatalf("topics body missing: %v", gotTopics)
	}
}
