// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core

import (
	"context"
	"reflect"
	"testing"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
)

const (
	testRepoCodeberg = "ssh://git@codeberg.org/me/proj.git"
	testLogin        = "owner"
	testStable       = "main"
	testTrunk        = "dev"
)

// fakeProtector adds the Protector surface to fakeClient.
type fakeProtector struct {
	fakeClient
	current Protection
	patch   *Protection
}

func (f *fakeProtector) Login(context.Context) (string, error) { return testLogin, nil }

func (f *fakeProtector) FetchProtection(context.Context, string, string) (Protection, error) {
	return f.current, nil
}

func (f *fakeProtector) ApplyProtection(_ context.Context, _, _ string, p Protection) error {
	f.patch = &p
	return nil
}

// releasePF returns a document declaring the fleet release shape and one push goal.
func releasePF(repos ...projectfile.Repository) *projectfile.Document {
	pf := newPF("X", "", nil, repos)
	projectfile.SetExtension(pf, "org.projectfile.release", map[string]any{
		"trunk":         testTrunk,
		"stable-branch": testStable,
		"branches":      []any{map[string]any{"pattern": testTrunk}, map[string]any{"pattern": "release/*"}},
	})
	projectfile.SetExtension(pf, "org.projectfile.ci", map[string]any{"nodes": map[string]any{
		"published": map[string]any{"goal": true},
		"audited":   map[string]any{"goal": true, "when": map[string]any{"events": []any{"schedule", "dispatch"}}},
		"lint":      map[string]any{},
	}})
	return pf
}

func TestProtectionPlan_Desired(t *testing.T) {
	plan, err := buildProtectionPlan(releasePF())
	if err != nil || plan == nil {
		t.Fatalf("plan: %v %v", plan, err)
	}
	origin := plan.desired(true, testLogin)
	want := Protection{
		FastForwardOnly: true,
		Branches: []BranchRule{
			{Pattern: testStable, Locked: true, SignedCommits: true, Pushers: []string{testLogin}, StatusChecks: []string{"published / published (*)"}},
			{Pattern: testTrunk, Locked: true},
			{Pattern: "release/*", Locked: true},
		},
		Tags: []TagRule{{Pattern: "*", Pushers: []string{testLogin}}},
	}
	if !reflect.DeepEqual(origin, want) {
		t.Fatalf("origin:\n got %+v\nwant %+v", origin, want)
	}
	mirror := plan.desired(false, "")
	if mirror.FastForwardOnly || len(mirror.Tags) != 0 || mirror.Branches[0].Pushers != nil || mirror.Branches[0].StatusChecks != nil {
		t.Fatalf("mirror must carry history guards only: %+v", mirror)
	}
}

func TestProtectionPlan_AbsentRelease(t *testing.T) {
	plan, err := buildProtectionPlan(newPF("X", "", nil, nil))
	if err != nil || plan != nil {
		t.Fatalf("want no plan, got %+v %v", plan, err)
	}
}

func TestTagGlob(t *testing.T) {
	for format, want := range map[string]string{"": "*", "${version}": "*", "v${version}": "v*"} {
		if got := tagGlob(format); got != want {
			t.Errorf("tagGlob(%q) = %q, want %q", format, got, want)
		}
	}
}

func TestDiffProtection_KeepsUnownedAndConforming(t *testing.T) {
	current := Protection{Branches: []BranchRule{
		{Pattern: "other", Locked: true},
		{Pattern: testTrunk, Locked: true},
		{ID: 7, Pattern: testStable, Locked: true},
	}}
	desired := Protection{Branches: []BranchRule{
		{Pattern: testTrunk, Locked: true},
		{Pattern: testStable, Locked: true, SignedCommits: true},
	}}
	patch := DiffProtection(current, desired)
	if len(patch.Branches) != 1 || patch.Branches[0].Pattern != testStable || patch.Branches[0].ID != 7 {
		t.Fatalf("want only the stable rule with its id, got %+v", patch.Branches)
	}
}

func TestNamesEqual_NilIsNotEmpty(t *testing.T) {
	if namesEqual(nil, []string{}) {
		t.Fatal("nil (anyone) must differ from empty (nobody)")
	}
	if !namesEqual([]string{"a", "b"}, []string{"b", "a"}) {
		t.Fatal("order must not matter")
	}
}

func TestPush_ProtectionAppliedOnOrigin(t *testing.T) {
	pf := releasePF(projectfile.Repository{URL: testRepoCodeberg, Role: projectfile.RepositoryRoleOrigin})
	fp := &fakeProtector{fakeClient: fakeClient{kind: testForgejo, snapshot: Snapshot{Description: "X", DefaultBranch: testTrunk}}}
	res, err := Push(context.Background(), pf, PushOptions{
		Resolve:     func(string) Client { return fp },
		TokenLookup: stubToken,
	})
	if err != nil {
		t.Fatalf("Push: %v", err)
	}
	if fp.applyCount != 0 {
		t.Fatalf("metadata already conforms, apply called %d times", fp.applyCount)
	}
	if fp.patch == nil || len(fp.patch.Branches) != 3 || len(fp.patch.Tags) != 1 || !fp.patch.FastForwardOnly {
		t.Fatalf("protection patch: %+v", fp.patch)
	}
	if res.Repos[0].UpToDate || res.Repos[0].Format() == "" {
		t.Fatalf("repo result: %+v", res.Repos[0])
	}
}

func TestPush_ProtectionDryRunAndFieldToggle(t *testing.T) {
	pf := releasePF(projectfile.Repository{URL: testRepoCodeberg, Role: projectfile.RepositoryRoleOrigin})
	fp := &fakeProtector{fakeClient: fakeClient{kind: testForgejo, snapshot: Snapshot{Description: "X"}}}
	opts := PushOptions{Resolve: func(string) Client { return fp }, TokenLookup: stubToken, DryRun: true}
	if _, err := Push(context.Background(), pf, opts); err != nil || fp.patch != nil {
		t.Fatalf("dry run applied protection: %+v %v", fp.patch, err)
	}
	opts.DryRun, opts.Fields = false, []string{FieldDescription}
	res, err := Push(context.Background(), pf, opts)
	if err != nil || fp.patch != nil || !res.Repos[0].UpToDate {
		t.Fatalf("--field description must leave protection alone: %+v %+v %v", fp.patch, res.Repos[0], err)
	}
}

func TestPush_ProtectionUnsupportedSkipsWithoutFailing(t *testing.T) {
	pf := releasePF(projectfile.Repository{URL: testRepoGitLab, Role: projectfile.RepositoryRoleMirror})
	fc := &fakeClient{kind: testKindGitLab, snapshot: Snapshot{Description: "X"}}
	res, err := Push(context.Background(), pf, PushOptions{Resolve: func(string) Client { return fc }, TokenLookup: stubToken})
	if err != nil || res.Repos[0].Failed || !res.Repos[0].UpToDate {
		t.Fatalf("unsupported protection must not fail the repo: %+v %v", res.Repos[0], err)
	}
}
