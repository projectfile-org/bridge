// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// Test fixtures. Centralised so the goconst linter doesn't flag the
// expected repetition across table-driven cases.
const (
	testRepoGitHub = "https://github.com/me/proj"
	testKindGitHub = "github"
	testKindGitLab = "gitlab"
	testToken      = "tok"
	testForgejo    = "forgejo"
)

// stubToken is the TokenLookup stub the push tests share; returns a fixed
// token + source for every call so the algorithm reaches the driver path.
func stubToken(string, string) (string, string) { return testToken, "X" }

func TestDiff_EmptyWhenEqual(t *testing.T) {
	cur := Snapshot{Description: "x", Homepage: "h", Topics: []string{"a", "b"}}
	des := Snapshot{Description: "x", Homepage: "h", Topics: []string{"b", "a"}}
	patch := Diff(cur, des)
	if !patch.IsEmpty() {
		t.Fatalf("Diff should be empty: %+v", patch)
	}
}

func TestDiff_DescriptionChange(t *testing.T) {
	cur := Snapshot{Description: "old"}
	des := Snapshot{Description: "new"}
	patch := Diff(cur, des)
	if patch.Description == nil || *patch.Description != "new" {
		t.Fatalf("got %+v", patch)
	}
}

func TestDiff_TopicsClear(t *testing.T) {
	cur := Snapshot{Topics: []string{"a"}}
	des := Snapshot{Topics: nil}
	patch := Diff(cur, des)
	if patch.Topics == nil {
		t.Fatalf("expected topics in patch, got empty patch")
	}
	if len(*patch.Topics) != 0 {
		t.Fatalf("expected empty topics, got %v", *patch.Topics)
	}
}

func TestLowerTopics(t *testing.T) {
	got := LowerTopics([]string{" Go ", "go", "SAT", "  ", "sat", "cli"}) //nolint:goconst // "cli" reads more clearly as a literal test fixture than as a named constant
	want := []string{"go", "sat", "cli"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// fakeClient is the per-test driver stub used by the push tests. Records
// the call history on the embedded fields so each assertion can inspect
// exactly which methods fired.
type fakeClient struct {
	kind       string
	snapshot   Snapshot
	fetchErr   error
	applyErr   error
	applied    Patch
	applyCount int
}

func (f *fakeClient) Kind() string { return f.kind }
func (f *fakeClient) Owner(_ string) (string, string, error) {
	return "owner", "repo", nil
}

func (f *fakeClient) Fetch(_ context.Context, _, _ string) (Snapshot, error) {
	return f.snapshot, f.fetchErr
}

func (f *fakeClient) Apply(_ context.Context, _, _ string, p Patch) error {
	f.applyCount++
	f.applied = p
	return f.applyErr
}

func newPF(summary, homepage string, keywords []string, repos []projectfile.Repository) *projectfile.Document {
	pf := &projectfile.Document{
		Identity: projectfile.Identity{
			Summary: &projectfile.LocalizedString{Bare: summary},
		},
		Repositories: repos,
		Keywords:     keywords,
	}
	if homepage != "" {
		pf.Links = []projectfile.Link{{Type: projectfile.LinkHomepage, URL: homepage}}
	}
	return pf
}

func TestPush_DescriptionFromSummary(t *testing.T) {
	pf := newPF("My nice project", "https://example.org", []string{"go", "cli"},
		[]projectfile.Repository{{URL: testRepoGitHub}})
	fc := &fakeClient{kind: testKindGitHub}
	opts := PushOptions{
		Resolve:     func(string) Client { return fc },
		TokenLookup: stubToken,
	}
	res, err := Push(context.Background(), pf, opts)
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if fc.applyCount != 1 {
		t.Fatalf("apply called %d times, want 1", fc.applyCount)
	}
	if fc.applied.Description == nil || *fc.applied.Description != "My nice project" {
		t.Fatalf("description not pushed: %+v", fc.applied.Description)
	}
	if fc.applied.Homepage == nil || *fc.applied.Homepage != "https://example.org" {
		t.Fatalf("homepage not pushed: %+v", fc.applied.Homepage)
	}
	if fc.applied.Topics == nil || !reflect.DeepEqual(*fc.applied.Topics, []string{"go", "cli"}) {
		t.Fatalf("topics not pushed: %+v", fc.applied.Topics)
	}
	if len(res.Repos) != 1 || res.Repos[0].UpToDate {
		t.Fatalf("unexpected repo result: %+v", res.Repos)
	}
}

func TestPush_DescriptionFallsBackToDescription(t *testing.T) {
	pf := &projectfile.Document{
		Identity: projectfile.Identity{
			Description: &projectfile.LocalizedString{Bare: "first line\nsecond line"},
		},
		Repositories: []projectfile.Repository{{URL: testRepoGitHub}},
	}
	fc := &fakeClient{kind: testKindGitHub}
	opts := PushOptions{
		Resolve:     func(string) Client { return fc },
		TokenLookup: stubToken,
	}
	if _, err := Push(context.Background(), pf, opts); err != nil {
		t.Fatalf("Push: %v", err)
	}
	if fc.applied.Description == nil || *fc.applied.Description != "first line" {
		t.Fatalf("got %v, want 'first line'", fc.applied.Description)
	}
}

func TestPush_IdempotentNoApply(t *testing.T) {
	pf := newPF("X", "https://example.org", []string{"a"},
		[]projectfile.Repository{{URL: testRepoGitHub}})
	fc := &fakeClient{
		kind:     testKindGitHub,
		snapshot: Snapshot{Description: "X", Homepage: "https://example.org", Topics: []string{"a"}},
	}
	opts := PushOptions{
		Resolve:     func(string) Client { return fc },
		TokenLookup: stubToken,
	}
	res, err := Push(context.Background(), pf, opts)
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if fc.applyCount != 0 {
		t.Fatalf("apply called %d times on no-op run, want 0", fc.applyCount)
	}
	if !res.Repos[0].UpToDate {
		t.Fatalf("expected UpToDate, got %+v", res.Repos[0])
	}
}

func TestPush_DryRunSkipsApply(t *testing.T) {
	pf := newPF("X", "", nil, []projectfile.Repository{{URL: testRepoGitHub}})
	fc := &fakeClient{kind: testKindGitHub}
	opts := PushOptions{
		DryRun:      true,
		Resolve:     func(string) Client { return fc },
		TokenLookup: stubToken,
	}
	if _, err := Push(context.Background(), pf, opts); err != nil {
		t.Fatalf("Push: %v", err)
	}
	if fc.applyCount != 0 {
		t.Fatalf("apply called in dry-run, count=%d", fc.applyCount)
	}
}

func TestPush_ArchiveRoleSkipped(t *testing.T) {
	pf := newPF("X", "", nil, []projectfile.Repository{
		{URL: testRepoGitHub, Role: projectfile.RepositoryRoleArchive},
	})
	fc := &fakeClient{kind: testKindGitHub}
	opts := PushOptions{
		Resolve:     func(string) Client { return fc },
		TokenLookup: stubToken,
	}
	res, err := Push(context.Background(), pf, opts)
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if fc.applyCount != 0 {
		t.Fatalf("apply called on archive role")
	}
	if !res.Repos[0].Skipped || res.Repos[0].Reason != "archive role" {
		t.Fatalf("expected archive skip, got %+v", res.Repos[0])
	}
}

func TestPush_MissingTokenSkipped(t *testing.T) {
	pf := newPF("X", "", nil, []projectfile.Repository{{URL: testRepoGitHub}})
	fc := &fakeClient{kind: testKindGitHub}
	opts := PushOptions{
		Resolve:     func(string) Client { return fc },
		TokenLookup: func(string, string) (string, string) { return "", "" },
	}
	res, err := Push(context.Background(), pf, opts)
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if fc.applyCount != 0 {
		t.Fatalf("apply called without token")
	}
	if !res.Repos[0].Skipped {
		t.Fatalf("expected skip, got %+v", res.Repos[0])
	}
}

func TestPush_HomepageSkippedOnGitLab(t *testing.T) {
	pf := newPF("X", "https://example.org", nil,
		[]projectfile.Repository{{URL: "https://gitlab.com/me/proj"}})
	fc := &fakeClient{kind: testKindGitLab}
	opts := PushOptions{
		Resolve:     func(string) Client { return fc },
		TokenLookup: stubToken,
	}
	if _, err := Push(context.Background(), pf, opts); err != nil {
		t.Fatalf("Push: %v", err)
	}
	if fc.applied.Homepage != nil {
		t.Fatalf("homepage should not be in gitlab patch, got %v", *fc.applied.Homepage)
	}
	if fc.applied.Description == nil {
		t.Fatalf("description should still be pushed")
	}
}

func TestPush_ExtensionDisablesAll(t *testing.T) {
	pf := newPF("X", "", nil, []projectfile.Repository{{URL: testRepoGitHub}})
	projectfile.SetExtension(pf, pfmodel.ForgeExtensionNS, map[string]any{"push": false})
	fc := &fakeClient{kind: testKindGitHub}
	opts := PushOptions{
		Resolve:     func(string) Client { return fc },
		TokenLookup: stubToken,
	}
	res, err := Push(context.Background(), pf, opts)
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if fc.applyCount != 0 {
		t.Fatalf("apply called when extension disabled push")
	}
	if len(res.Repos) != 0 {
		t.Fatalf("expected zero repo results when disabled, got %d", len(res.Repos))
	}
}

func TestPush_HostOptOut(t *testing.T) {
	pf := newPF("X", "", nil, []projectfile.Repository{{URL: testRepoGitHub}})
	if pf.Extensions == nil {
		pf.Extensions = map[string]any{}
	}
	projectfile.SetExtension(pf, pfmodel.ForgeExtensionNS, map[string]any{
		"hosts": map[string]any{CanonicalGitHub: false},
	})
	fc := &fakeClient{kind: testKindGitHub}
	opts := PushOptions{
		Resolve:     func(string) Client { return fc },
		TokenLookup: stubToken,
	}
	res, err := Push(context.Background(), pf, opts)
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if fc.applyCount != 0 {
		t.Fatalf("apply called when host opt-out set")
	}
	if !res.Repos[0].Skipped {
		t.Fatalf("expected skip, got %+v", res.Repos[0])
	}
}

func TestPush_KindsOverride_SelfHostedForgejo(t *testing.T) {
	// code.example.com is a hostname hostmatch can't classify; the extension
	// declares it as forgejo and push should route through the forgejo driver.
	pf := newPF("X", "", nil, []projectfile.Repository{{URL: "https://code.example.com/me/proj"}})
	projectfile.SetExtension(pf, pfmodel.ForgeExtensionNS, map[string]any{
		"kinds": map[string]any{"code.example.com": testForgejo},
	})
	fc := &fakeClient{kind: testForgejo}
	var resolvedKind string
	opts := PushOptions{
		Resolve: func(k string) Client {
			resolvedKind = k
			return fc
		},
		TokenLookup: stubToken,
	}
	res, err := Push(context.Background(), pf, opts)
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if resolvedKind != testForgejo {
		t.Fatalf("expected resolver to be called with 'forgejo', got %q", resolvedKind)
	}
	if res.Repos[0].Skipped {
		t.Fatalf("repo should not have been skipped: %+v", res.Repos[0])
	}
	if res.Repos[0].Kind != testForgejo {
		t.Fatalf("expected kind=forgejo, got %q", res.Repos[0].Kind)
	}
	if fc.applyCount != 1 {
		t.Fatalf("expected one Apply call, got %d", fc.applyCount)
	}
}

func TestPush_KindsOverride_BeatsHostmatch(t *testing.T) {
	// github.com would auto-resolve to "github"; the extension overrides to
	// "forgejo" (pathological but legal — user knows best).
	pf := newPF("X", "", nil, []projectfile.Repository{{URL: testRepoGitHub}})
	projectfile.SetExtension(pf, pfmodel.ForgeExtensionNS, map[string]any{
		"kinds": map[string]any{CanonicalGitHub: testForgejo},
	})
	fc := &fakeClient{kind: testForgejo}
	var resolvedKind string
	opts := PushOptions{
		Resolve: func(k string) Client {
			resolvedKind = k
			return fc
		},
		TokenLookup: stubToken,
	}
	if _, err := Push(context.Background(), pf, opts); err != nil {
		t.Fatalf("Push: %v", err)
	}
	if resolvedKind != testForgejo {
		t.Fatalf("extension kind override didn't win: resolver got %q", resolvedKind)
	}
}

func TestPush_UnsupportedHost_NoOverride(t *testing.T) {
	pf := newPF("X", "", nil, []projectfile.Repository{{URL: "https://code.example.com/me/proj"}})
	fc := &fakeClient{kind: testForgejo}
	opts := PushOptions{
		Resolve:     func(string) Client { return fc },
		TokenLookup: stubToken,
	}
	res, err := Push(context.Background(), pf, opts)
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if !res.Repos[0].Skipped {
		t.Fatalf("expected skip on unsupported host, got %+v", res.Repos[0])
	}
	if !strings.Contains(res.Repos[0].Reason, "code.example.com") {
		t.Fatalf("skip reason should name the host and the extension key: %q", res.Repos[0].Reason)
	}
}

func TestPush_FieldFilter(t *testing.T) {
	pf := newPF("X", "https://example.org", []string{"a"},
		[]projectfile.Repository{{URL: testRepoGitHub}})
	fc := &fakeClient{kind: testKindGitHub}
	opts := PushOptions{
		Fields:      []string{FieldDescription}, // only push description
		Resolve:     func(string) Client { return fc },
		TokenLookup: stubToken,
	}
	if _, err := Push(context.Background(), pf, opts); err != nil {
		t.Fatalf("Push: %v", err)
	}
	if fc.applied.Description == nil {
		t.Fatalf("description should be in patch")
	}
	if fc.applied.Homepage != nil {
		t.Fatalf("homepage should be filtered out, got %v", *fc.applied.Homepage)
	}
	if fc.applied.Topics != nil {
		t.Fatalf("topics should be filtered out, got %v", *fc.applied.Topics)
	}
}
