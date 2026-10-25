// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package scancmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/v4/internal/scanners/core"
	"projectfile.org/projectfile/bridge/v4/internal/source"
)

// Fixture values shared across the helper cases, hoisted so goconst sees one declaration.
const (
	personEmail    = "dev@example.com"
	orgName        = "Acme"
	givenNames     = "Jane"
	familyNames    = "Doe"
	stackTagGo     = "go"
	stackTagPy     = "python"
	createdDate    = "2024-01-01"
	modifiedDate   = "2025-06-15"
	curatedDate    = "2020-01-01"
	incomingRole   = "contributor"
	existingBranch = "main"
	incomingBranch = "dev"
	incomingType   = "git"
	incomingPath   = "packages/lib"
	otherRepoURL   = "https://github.com/acme/other"
)

func strPtr(s string) *string { return &s }

// resetScanVars restores the five option vars, since pflag only rewrites a global when the flag is registered.
func resetScanVars(t *testing.T) {
	t.Helper()
	scanDryRun, scanCheck, scanStrict, scanPrune, scanForce = false, false, false, false, false
	t.Cleanup(func() {
		scanDryRun, scanCheck, scanStrict, scanPrune, scanForce = false, false, false, false, false
	})
}

func TestParseScanArgs(t *testing.T) {
	dir := t.TempDir()
	other := t.TempDir()
	cases := []struct {
		name      string
		args      []string
		wantNames []string
		wantDir   string
	}{
		{"no args scans from the current directory", nil, nil, "."},
		{"one scanner keeps the default directory", []string{scannerStacks}, []string{scannerStacks}, "."},
		{"a directory arg is the directory, not a scanner", []string{dir}, nil, dir},
		{"scanners before the directory", []string{scannerForge, scannerSinks, scannerStacks, dir}, []string{scannerForge, scannerSinks, scannerStacks}, dir},
		{"the git composite is a name", []string{scannerGit}, []string{scannerGit}, "."},
		{"the last non-scanner arg wins the directory", []string{scannerStacks, dir, other}, []string{scannerStacks}, other},
		{"an unrecognised word is read as the directory", []string{"not-a-scanner"}, nil, "not-a-scanner"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			names, gotDir := parseScanArgs(tc.args)
			assert.Equal(t, tc.wantNames, names)
			assert.Equal(t, tc.wantDir, gotDir)
		})
	}
}

func TestExpandScannerNames(t *testing.T) {
	every := []string{"forge", "git-authors", "git-dates", "git-remotes", "sinks", "stacks"}
	gitComposite := []string{"git-authors", "git-remotes", "git-dates"}
	cases := []struct {
		name string
		in   []string
		want []string
	}{
		{"all expands to the whole sorted registry", []string{cmdAll}, every},
		{"git expands to the three git scanners", []string{scannerGit}, gitComposite},
		{"registry names pass through in order", []string{scannerGitRemotes, scannerForge}, []string{scannerGitRemotes, scannerForge}},
		{"duplicates collapse keeping first position", []string{scannerGit, scannerGitAuthors}, gitComposite},
		{"all dedupes an explicit scanner", []string{cmdAll, scannerForge}, every},
		{"no names expand to no names", nil, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, expandScannerNames(tc.in))
		})
	}
}

// The registry List() feed pins every sorted name: a new scanner driver lands here in this list or the test fails.
func TestExpandAllMatchesRegistryList(t *testing.T) {
	var want []string
	for _, s := range core.List() {
		want = append(want, s.Name())
	}
	assert.Equal(t, want, expandScannerNames([]string{cmdAll}))
	require.Len(t, want, 6, "six scanner drivers register through the aggregator")
}

func TestLogHits(_ *testing.T) {
	logHits([]core.Hit{
		{Source: scannerStacks, Field: "stack:go", Path: "/repo/go.mod"},
		{Source: scannerGitRemotes, Field: "repositories[role=origin]"},
	})
	logHits(nil)
}

func TestPartialPeopleToProjectfile(t *testing.T) {
	t.Run("a family-less name is an organization", func(t *testing.T) {
		people, orgs := partialPeopleToProjectfile([]source.PersonEntry{
			{Name: orgName, Email: "team@example.com", Orcid: "https://orcid.org/0000-0002", URL: "https://acme.example.com", Roles: []string{"funder"}},
		})
		assert.Empty(t, people)
		require.Len(t, orgs, 1)
		assert.Equal(t, projectfile.Organization{
			Name: orgName, Email: "team@example.com", Orcid: "https://orcid.org/0000-0002",
			URL: "https://acme.example.com", Roles: []string{"funder"},
		}, orgs[0])
	})

	t.Run("a family name is a person carrying every field", func(t *testing.T) {
		people, orgs := partialPeopleToProjectfile([]source.PersonEntry{
			{FamilyNames: familyNames, GivenNames: givenNames, Email: personEmail, Orcid: "https://orcid.org/0000-0001", URL: "https://jane.example.com", Roles: []string{"author"}, Affiliation: orgName},
		})
		assert.Empty(t, orgs)
		require.Len(t, people, 1)
		assert.Equal(t, projectfile.Person{
			FamilyNames: familyNames, GivenNames: givenNames, Email: personEmail,
			Orcid: "https://orcid.org/0000-0001", URL: "https://jane.example.com",
			Roles: []string{"author"}, Affiliation: orgName,
		}, people[0])
	})

	t.Run("no entries produce no people and no orgs", func(t *testing.T) {
		people, orgs := partialPeopleToProjectfile(nil)
		assert.Nil(t, people)
		assert.Nil(t, orgs)
	})
}

func TestApplyRepository(t *testing.T) {
	t.Run("appends a URL the document does not list", func(t *testing.T) {
		doc := &projectfile.Document{Repositories: []projectfile.Repository{{URL: testApplyLinkURL}}}
		applyRepository(doc, projectfile.Repository{URL: otherRepoURL, Role: projectfile.RepositoryRoleOrigin})
		require.Len(t, doc.Repositories, 2)
		assert.Equal(t, projectfile.RepositoryRoleOrigin, doc.Repositories[1].Role)
	})

	t.Run("gap-fills every empty field of a known URL", func(t *testing.T) {
		doc := &projectfile.Document{Repositories: []projectfile.Repository{{URL: testApplyLinkURL}}}
		applyRepository(doc, projectfile.Repository{
			URL: testApplyLinkURL, Role: incomingRole, Branch: incomingBranch,
			Type: incomingType, Path: incomingPath,
		})
		require.Len(t, doc.Repositories, 1)
		assert.Equal(t, projectfile.Repository{
			URL: testApplyLinkURL, Role: incomingRole, Branch: incomingBranch,
			Type: incomingType, Path: incomingPath,
		}, doc.Repositories[0])
	})

	t.Run("promotes a non-origin role to origin", func(t *testing.T) {
		doc := &projectfile.Document{Repositories: []projectfile.Repository{{URL: testApplyLinkURL, Role: "mirror"}}}
		applyRepository(doc, projectfile.Repository{URL: testApplyLinkURL, Role: projectfile.RepositoryRoleOrigin})
		assert.Equal(t, projectfile.RepositoryRoleOrigin, doc.Repositories[0].Role)
	})

	t.Run("keeps an origin role and a declared branch", func(t *testing.T) {
		doc := &projectfile.Document{Repositories: []projectfile.Repository{
			{URL: otherRepoURL, Role: "mirror"},
			{URL: testApplyLinkURL, Role: projectfile.RepositoryRoleOrigin, Branch: existingBranch, Type: "ssh", Path: "src"},
		}}
		applyRepository(doc, projectfile.Repository{
			URL: testApplyLinkURL, Role: projectfile.RepositoryRoleOrigin, Branch: incomingBranch, Type: incomingType, Path: incomingPath,
		})
		assert.Equal(t, projectfile.Repository{
			URL: testApplyLinkURL, Role: projectfile.RepositoryRoleOrigin, Branch: existingBranch, Type: "ssh", Path: "src",
		}, doc.Repositories[1], "an origin entry never loses a declared branch, type or path")
	})

	t.Run("fills an empty role from a non-origin proposal", func(t *testing.T) {
		doc := &projectfile.Document{Repositories: []projectfile.Repository{{URL: testApplyLinkURL}}}
		applyRepository(doc, projectfile.Repository{URL: testApplyLinkURL, Role: incomingRole})
		assert.Equal(t, incomingRole, doc.Repositories[0].Role)
	})
}

func TestEnsurePrimaryRepository(t *testing.T) {
	t.Run("promotes the first entry when none is origin", func(t *testing.T) {
		repos := []projectfile.Repository{{URL: testApplyLinkURL}, {URL: otherRepoURL}}
		ensurePrimaryRepository(repos)
		assert.Equal(t, projectfile.RepositoryRoleOrigin, repos[0].Role)
		assert.Equal(t, "", repos[1].Role)
	})

	t.Run("leaves an existing origin alone", func(t *testing.T) {
		repos := []projectfile.Repository{
			{URL: otherRepoURL},
			{URL: testApplyLinkURL, Role: projectfile.RepositoryRoleOrigin},
		}
		ensurePrimaryRepository(repos)
		assert.Equal(t, "", repos[0].Role, "a mirror reads empty rather than promoted")
		assert.Equal(t, projectfile.RepositoryRoleOrigin, repos[1].Role)
	})
}

func TestApplyPartialToDocMergesScannerOutput(t *testing.T) {
	resetScanVars(t) // applyLink reads scanForce, and an earlier case may have armed it
	t.Run("a nil partial changes nothing", func(t *testing.T) {
		doc := &projectfile.Document{Identity: projectfile.Identity{Name: "scancase"}}
		applyPartialToDoc(doc, &projectfile.Document{}, nil)
		assert.Equal(t, "scancase", doc.Identity.Name)
		assert.Empty(t, doc.Stack)
		assert.Empty(t, doc.Repositories)
	})

	t.Run("stacks union with an existing declaration", func(t *testing.T) {
		doc := &projectfile.Document{Stack: []string{stackTagPy}}
		applyPartialToDoc(doc, &projectfile.Document{}, &source.Partial{Stack: []string{stackTagGo}})
		assert.Equal(t, []string{stackTagGo, stackTagPy}, doc.Stack, "Union dedupes and sorts")
	})

	t.Run("an effective person is not re-materialised into the base", func(t *testing.T) {
		effective := &projectfile.Document{People: []projectfile.Person{{Email: personEmail}}}
		doc := &projectfile.Document{}
		applyPartialToDoc(doc, effective, &source.Partial{People: []source.PersonEntry{
			{FamilyNames: familyNames, GivenNames: givenNames, Email: personEmail},
			{FamilyNames: "Lovelace", GivenNames: "Ada", Email: "ada@example.com"},
		}})
		require.Len(t, doc.People, 1)
		assert.Equal(t, "ada@example.com", doc.People[0].Email, "the include already carries the maintainer")
	})

	t.Run("an effective organization is not re-materialised into the base", func(t *testing.T) {
		effective := &projectfile.Document{Organizations: []projectfile.Organization{{Name: orgName}}}
		doc := &projectfile.Document{}
		applyPartialToDoc(doc, effective, &source.Partial{People: []source.PersonEntry{
			{Name: orgName},
			{Name: "Sponsor"},
		}})
		require.Len(t, doc.Organizations, 1)
		assert.Equal(t, "Sponsor", doc.Organizations[0].Name)
	})

	t.Run("a scanned repository is applied and promoted to origin", func(t *testing.T) {
		doc := &projectfile.Document{}
		applyPartialToDoc(doc, &projectfile.Document{}, &source.Partial{
			Repositories: []projectfile.Repository{{URL: testApplyLinkURL}},
		})
		require.Len(t, doc.Repositories, 1)
		assert.Equal(t, projectfile.RepositoryRoleOrigin, doc.Repositories[0].Role)
	})

	t.Run("created and modified fill only an empty identity date", func(t *testing.T) {
		scanned := &source.Partial{Created: strPtr(createdDate), Modified: strPtr(modifiedDate)}

		filled := &projectfile.Document{}
		applyPartialToDoc(filled, &projectfile.Document{}, scanned)
		assert.Equal(t, createdDate, filled.Identity.Created)
		assert.Equal(t, modifiedDate, filled.Identity.Modified)

		curated := &projectfile.Document{Identity: projectfile.Identity{Created: curatedDate, Modified: curatedDate}}
		applyPartialToDoc(curated, &projectfile.Document{}, scanned)
		assert.Equal(t, curatedDate, curated.Identity.Created, "HEAD's date churns every commit")
		assert.Equal(t, curatedDate, curated.Identity.Modified)

		blank := &projectfile.Document{}
		applyPartialToDoc(blank, &projectfile.Document{}, &source.Partial{Created: strPtr(""), Modified: strPtr("")})
		assert.Equal(t, "", blank.Identity.Created, "a blank scanned date fills nothing")
	})
}
