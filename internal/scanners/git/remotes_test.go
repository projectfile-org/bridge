// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package git

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
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
	require.Len(t, p.Links, 1)
	assert.Equal(t, projectfile.LinkSourceCode, p.Links[0].Type)
	assert.True(t, p.Links[0].Preferred, "sole origin link must be Preferred")
}
