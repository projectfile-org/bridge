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
)

// gitCmd runs a git invocation in dir with deterministic identity + dates so
// the test is reproducible regardless of the host git config.
func gitCmd(t *testing.T, dir, authorDate string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test", "GIT_COMMITTER_EMAIL=test@example.com",
		"GIT_AUTHOR_DATE="+authorDate, "GIT_COMMITTER_DATE="+authorDate,
	)
	require.NoError(t, cmd.Run(), "git %v", args)
}

// initRepo builds a two-commit repo: root commit on 2020-01-02, HEAD on
// 2023-06-07. Created should track the root, Modified the HEAD.
func initRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	gitCmd(t, dir, "2020-01-02T10:00:00", "init")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0o644))
	gitCmd(t, dir, "2020-01-02T10:00:00", "add", "a.txt")
	gitCmd(t, dir, "2020-01-02T10:00:00", "commit", "-m", "root")
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0o644))
	gitCmd(t, dir, "2023-06-07T12:00:00", "add", "b.txt")
	gitCmd(t, dir, "2023-06-07T12:00:00", "commit", "-m", "second")
	return dir
}

func TestScanPopulatesCreatedAndModified(t *testing.T) {
	dir := initRepo(t)
	p, _, err := Scanner{}.Scan(dir)
	require.NoError(t, err)
	require.NotNil(t, p.Created)
	require.NotNil(t, p.Modified)
	assert.Equal(t, "2020-01-02", *p.Created, "created tracks the root commit")
	assert.Equal(t, "2023-06-07", *p.Modified, "modified tracks HEAD")
}

// lastCommitDate mirrors firstCommitDate but reports HEAD's date.
func TestLastCommitDate(t *testing.T) {
	dir := initRepo(t)
	assert.Equal(t, "2023-06-07", lastCommitDate(dir))
	assert.Equal(t, "2020-01-02", firstCommitDate(dir))
}

func TestLastCommitDateEmptyRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	gitCmd(t, dir, "2020-01-02T10:00:00", "init")
	assert.Empty(t, lastCommitDate(dir), "empty repo yields no modified date")
}
