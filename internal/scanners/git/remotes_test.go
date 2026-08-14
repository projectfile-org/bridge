// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// remotesRepo builds an empty git repo with two remotes (origin + a mirror).
// No commits are needed for remotes scanning, so this sidesteps any host-side
// commit-signing config. origin must be a host hostmatch recognises so its
// landing page is emitted (forgePageURL non-empty).
func remotesRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	require.NoError(t, exec.Command("git", "-C", dir, "init", "--quiet").Run())
	addRemote := func(name, url string) {
		require.NoError(t, exec.Command("git", "-C", dir, "remote", "add", name, url).Run(),
			"git remote add %s", name)
	}
	addRemote("origin", "https://github.com/acme/proj.git")
	addRemote("codeberg", "https://codeberg.org/acme/proj.git")
	return dir
}

// TestRemotesScannerOriginLinkPreferred guards that the git-remotes scanner
// marks ONLY the origin remote's source-code link Preferred: true, so the
// §5.11 selector (LinkByType) returns the origin page over mirror pages —
// the citable repo is the origin, not whichever remote appended first.
func TestRemotesScannerOriginLinkPreferred(t *testing.T) {
	dir := remotesRepo(t)
	p, hits, err := remotesScanner{}.Scan(dir)
	require.NoError(t, err)

	// Exactly two source-code links: origin (preferred) + codeberg mirror.
	var origin, mirror *projectfile.Link
	var originHit, mirrorHit bool
	for i := range p.Links {
		l := &p.Links[i]
		if l.Type != projectfile.LinkSourceCode {
			continue
		}
		switch l.URL {
		case "https://github.com/acme/proj":
			origin = l
		case "https://codeberg.org/acme/proj":
			mirror = l
		}
	}
	for _, h := range hits {
		if h.Field == "links[type=source-code]:origin" {
			originHit = true
		}
		if h.Field == "links[type=source-code]:codeberg" {
			mirrorHit = true
		}
	}

	require.NotNil(t, origin, "origin source-code link must be emitted")
	require.NotNil(t, mirror, "mirror source-code link must be emitted")
	assert.True(t, origin.Preferred, "origin link must be Preferred")
	assert.False(t, mirror.Preferred, "mirror link must NOT be Preferred")
	assert.True(t, originHit && mirrorHit, "both links must be reported as hits")

	// origin repo entry must carry role: origin; mirror role: mirror.
	var originRepo, mirrorRepo *projectfile.Repository
	for i := range p.Repositories {
		switch p.Repositories[i].URL {
		case "https://github.com/acme/proj.git":
			originRepo = &p.Repositories[i]
		case "https://codeberg.org/acme/proj.git":
			mirrorRepo = &p.Repositories[i]
		}
	}
	require.NotNil(t, originRepo)
	require.NotNil(t, mirrorRepo)
	assert.Equal(t, projectfile.RepositoryRoleOrigin, originRepo.Role)
	assert.Equal(t, projectfile.RepositoryRoleMirror, mirrorRepo.Role)
}

// TestRemotesScannerSoleRemotePreferred guards that a single (origin) remote
// still gets its link marked Preferred — the sole entry is implicitly origin.
func TestRemotesScannerSoleRemotePreferred(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	require.NoError(t, exec.Command("git", "-C", dir, "init", "--quiet").Run())
	require.NoError(t, exec.Command("git", "-C", dir, "remote", "add", "origin", "https://github.com/acme/proj.git").Run())

	p, _, err := remotesScanner{}.Scan(dir)
	require.NoError(t, err)
	var src *projectfile.Link
	for i := range p.Links {
		if p.Links[i].Type == projectfile.LinkSourceCode {
			src = &p.Links[i]
		}
	}
	require.NotNil(t, src, "source-code link must be emitted")
	assert.True(t, src.Preferred, "sole origin link must be Preferred")
}

// TestRemotesScannerProposesCapabilityTags guards the end-to-end proposal on
// the shape 132 of the 152 fleet projectfiles actually have: a Codeberg mirror
// and a GitHub mirror side by side. Both are crawlable, so both claim `badges`
// and document order decides — but each must carry its OWN shields route, or
// the `last-commit` badge queries a Gitea endpoint against a GitHub host.
func TestRemotesScannerProposesCapabilityTags(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	require.NoError(t, exec.Command("git", "-C", dir, "init", "--quiet").Run())
	require.NoError(t, exec.Command("git", "-C", dir, "remote", "add", "origin",
		"https://codeberg.org/acme/proj.git").Run())
	require.NoError(t, exec.Command("git", "-C", dir, "remote", "add", "gh",
		"https://github.com/acme/proj.git").Run())
	require.NoError(t, exec.Command("git", "-C", dir, "remote", "add", "internal",
		"https://gitlab.example.com/acme/proj.git").Run())

	p, hits, err := remotesScanner{}.Scan(dir)
	require.NoError(t, err)

	tagsFor := func(url string) []string {
		for _, l := range p.Links {
			if l.URL == url {
				return pfmodel.LinkTags(l)
			}
		}
		t.Fatalf("no source-code link for %q", url)
		return nil
	}
	assert.Equal(t, []string{pfmodel.TagPublic, pfmodel.TagBadges, pfmodel.TagBadgesGitea},
		tagsFor("https://codeberg.org/acme/proj"))
	assert.Equal(t, []string{pfmodel.TagPublic, pfmodel.TagBadges, pfmodel.TagBadgesGitHub},
		tagsFor("https://github.com/acme/proj"))
	assert.Empty(t, tagsFor("https://gitlab.example.com/acme/proj"),
		"a self-hosted instance must not be guessed public")

	var tagHit bool
	for _, h := range hits {
		if h.Field == "links[type=source-code].tags:origin" {
			tagHit = true
		}
	}
	assert.True(t, tagHit, "the proposal must be visible in --dry-run output")
}

// TestRemotesScannerDerivesIssuesLink guards that a links[type=bugs] entry is
// emitted beside each source-code one: the tracker URL the host rule resolves
// and a localized "Issues on {forge}" label, so a scan refreshes both entries
// a project's support surface needs in one pass.
func TestRemotesScannerDerivesIssuesLink(t *testing.T) {
	dir := remotesRepo(t)
	p, hits, err := remotesScanner{}.Scan(dir)
	require.NoError(t, err)

	bugsFor := func(url string) *projectfile.Link {
		for i := range p.Links {
			if p.Links[i].Type == projectfile.LinkBugs && p.Links[i].URL == url {
				return &p.Links[i]
			}
		}
		return nil
	}

	gh := bugsFor("https://github.com/acme/proj/issues")
	require.NotNil(t, gh, "github issues link must be emitted")
	require.NotNil(t, gh.Label, "issues link must carry a label")
	assert.Equal(t, "Issues on GitHub", gh.Label.Bare)

	cb := bugsFor("https://codeberg.org/acme/proj/issues")
	require.NotNil(t, cb, "codeberg issues link must be emitted")
	require.NotNil(t, cb.Label)
	assert.Equal(t, "Issues on Codeberg", cb.Label.Bare)

	var hit bool
	for _, h := range hits {
		if h.Field == "links[type=bugs]:origin" {
			hit = true
		}
	}
	assert.True(t, hit, "the issues derivation must be visible in --dry-run output")
}

// TestRemotesScannerDerivesIssuesLinkLocalized verifies the issues label is a
// Langs map when the project declares org.projectfile.i18n.languages — the
// multi-language parity with source-code links this change closes. A single-
// language project stays Bare (verified by TestRemotesScannerDerivesIssuesLink).
func TestRemotesScannerDerivesIssuesLinkLocalized(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	pfPath := filepath.Join(dir, "projectfile.yaml")
	require.NoError(t, os.WriteFile(pfPath, []byte("org:\n  projectfile:\n    i18n:\n      default-language: en\n      languages: [es]\n"), 0o644))
	require.NoError(t, exec.Command("git", "-C", dir, "init", "--quiet").Run())
	require.NoError(t, exec.Command("git", "-C", dir, "remote", "add", "origin", "https://github.com/acme/proj.git").Run())

	p, _, err := remotesScanner{}.Scan(dir)
	require.NoError(t, err)

	var bugs *projectfile.Link
	for i := range p.Links {
		if p.Links[i].Type == projectfile.LinkBugs {
			bugs = &p.Links[i]
		}
	}
	require.NotNil(t, bugs)
	require.NotNil(t, bugs.Label)
	assert.Empty(t, bugs.Label.Bare, "multi-language label must be a Langs map with Bare cleared")
	assert.Equal(t, map[string]string{
		"en": "Issues on GitHub",
		"es": "Incidencias en GitHub",
	}, bugs.Label.Langs)
}
