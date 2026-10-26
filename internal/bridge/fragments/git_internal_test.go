// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package fragments

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/netfetch"
)

// The git crossing is real in these tests: a fixture repository served over the
// file:// transport answers ls-remote and `git archive --remote` exactly as a
// forge does, so nothing is stubbed and the commands the fetcher composes are
// the ones that run.

// TestMain isolates the fixture repositories from the developer's own git:
// the global and system config are dropped, every inherited GIT_* variable
// goes, and the locale is pinned so a git diagnostic never arrives localized.
func TestMain(m *testing.M) {
	_ = os.Setenv("LC_ALL", "C")
	_ = os.Setenv("LANG", "C")
	_ = os.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	_ = os.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)
	for _, kv := range os.Environ() {
		if name, _, _ := strings.Cut(kv, "="); strings.HasPrefix(name, "GIT_") {
			_ = os.Unsetenv(name)
		}
	}
	os.Exit(m.Run())
}

// fixtureGit runs one git command in workDir with a self-contained identity:
// no global config is read, and signing is off so a tag never prompts.
func fixtureGit(t *testing.T, workDir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{
		"-c", "user.name=Tester", "-c", "user.email=tester@example.test",
		"-c", "commit.gpgsign=false", "-c", "tag.gpgsign=false",
		"-c", "init.defaultBranch=main", "-c", "advice.detachedHead=false",
	}, args...)...)
	cmd.Dir = workDir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v: %s", args, out)
}

// gitFixture builds one local repository serving the given files at the given
// release tags and returns the URL the fetcher is pointed at. The bare copy is
// what git reaches, and the URL is a plain path: a scheme URL is exactly the
// remote-helper form validateRepoURL refuses, since `scheme::` matches.
func gitFixture(t *testing.T, files map[string]string, tags ...string) string {
	t.Helper()
	work := t.TempDir()
	src := filepath.Join(work, "src")
	require.NoError(t, os.MkdirAll(src, 0o755))
	fixtureGit(t, src, "init", "--quiet")
	for name, body := range files {
		full := filepath.Join(src, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(body), 0o644))
	}
	fixtureGit(t, src, "add", "--all")
	fixtureGit(t, src, "commit", "--quiet", "--message", "fixture")
	for _, tag := range tags {
		fixtureGit(t, src, "tag", tag)
	}
	bare := filepath.Join(work, "kiota.ch", "b19", "ubuntu.git")
	require.NoError(t, os.MkdirAll(filepath.Dir(bare), 0o755))
	fixtureGit(t, work, "clone", "--quiet", "--bare", src, bare)
	return bare
}

// TestRunGitReturnsStdoutAndStderr covers one attempt: its stdout on success,
// and the two failure wraps — a command that exits with a diagnostic, and a
// start that never reaches git.
func TestRunGitReturnsStdoutAndStderr(t *testing.T) {
	out, err := runGit(context.Background(), "version")
	require.NoError(t, err)
	assert.Contains(t, out, "git version")

	_, err = runGit(context.Background(), "ls-remote", filepath.Join(t.TempDir(), "no-such-repository"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not appear to be a git repository")

	// A cancelled context never reaches the subprocess, so the error is the
	// context's own rather than an exit status.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = runGit(ctx, "version")
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "context canceled"), err.Error())
}

// TestRunGitRetriesATransportFailure pins the retry half of the policy: a
// transport failure is retried once under netfetch's schedule, and a git answer
// is not — the classifier decides, never the loop.
func TestRunGitRetriesATransportFailure(t *testing.T) {
	restore := netfetch.Attempts()
	netfetch.SetAttempts(2)
	t.Cleanup(func() { netfetch.SetAttempts(restore) })

	var log strings.Builder
	genlog.SetOutput(&log)
	genlog.SetVerbose(true)
	defer func() {
		genlog.SetVerbose(false)
		genlog.SetOutput(os.Stderr)
	}()

	_, err := runGit(context.Background(), "ls-remote", "https://127.0.0.1:1/no-such-forge")
	require.Error(t, err)
	assert.Contains(t, log.String(), "git retry", "a refused connection must be retried once")
	assert.Contains(t, log.String(), "ls-remote")

	_, err = runGit(context.Background(), "ls-remote", filepath.Join(t.TempDir(), "no-such-repository"))
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "connection refused")
}

// TestGitLsRemoteReadsTheSortedTagList verifies the one ls-remote form the
// floating version read uses: every option precedes the repository, so git
// orders the tags itself and nothing here has to know semver.
func TestGitLsRemoteReadsTheSortedTagList(t *testing.T) {
	url := gitFixture(t, map[string]string{outFeaturesName: "# Features\n"}, "v1.0.0", "v2.0.0", "v10.0.0")

	out, err := gitLsRemote(context.Background(), url, "--tags", "--refs", "--sort=-v:refname")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(out, fmt.Sprintf("%s\trefs/tags/", strings.Fields(out)[0])), out)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	require.Len(t, lines, 3)
	assert.Contains(t, lines[0], "refs/tags/v10.0.0", "git orders the versions, highest first")
}

// TestArchiveRefNamesTheRefToAskFor pins the ref sent to the server: the
// resolved name when a tag was resolved, HEAD for a branch read.
func TestArchiveRefNamesTheRefToAskFor(t *testing.T) {
	assert.Equal(t, "HEAD", archiveRef(""))
	assert.Equal(t, "v1.0.0", archiveRef("v1.0.0"))
}

// TestOwnerRepoSingleSegmentNamesTheWholeURL verifies a URL carrying no
// owner/repo pair names itself whole rather than losing its tail.
func TestOwnerRepoSingleSegmentNamesTheWholeURL(t *testing.T) {
	assert.Equal(t, "standalone", ownerRepo("standalone"))
	assert.Equal(t, "standalone", ownerRepo("standalone.git"))
}

// TestRunGitFailsOnRefusedRepository: a URL nothing listens on reports the
// transport refusal the classifier retries — the error text git itself writes.
func TestRunGitFailsOnRefusedRepository(t *testing.T) {
	restore := netfetch.Attempts()
	netfetch.SetAttempts(1)
	t.Cleanup(func() { netfetch.SetAttempts(restore) })

	_, err := runGit(context.Background(), "ls-remote", "https://127.0.0.1:1/no-such-forge")
	require.Error(t, err)
	assert.True(t, transientGit(err), "the refusal is a transport failure: %v", err)
}

// TestRunGitStopsRetryingWhenTheContextEnds: the backoff wait is bounded by the
// caller's context, so a run cancelled mid-retry gives up rather than blocking
// on the schedule.
func TestRunGitStopsRetryingWhenTheContextEnds(t *testing.T) {
	restore := netfetch.Attempts()
	netfetch.SetAttempts(2)
	t.Cleanup(func() { netfetch.SetAttempts(restore) })

	// The refusal answers in milliseconds while the backoff wait half a second,
	// so the deadline lands between the attempt and its retry.
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	_, err := runGit(ctx, "ls-remote", "https://127.0.0.1:1/no-such-forge")
	require.Error(t, err)
	assert.True(t, transientGit(err), err.Error())
}

// tarMember is one entry a fixture archive carries.
type tarMember struct {
	name string
	body string
	size int // declared body size, or -1 for the body's own length
	dir  bool
}

// tarStream builds one tar archive from the given members.
func tarStream(t *testing.T, members ...tarMember) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := tar.NewWriter(&buf)
	for _, m := range members {
		size := m.size
		if size < 0 {
			size = len(m.body)
		}
		flag := byte(tar.TypeReg)
		if m.dir {
			flag = tar.TypeDir
		}
		require.NoError(t, w.WriteHeader(&tar.Header{
			Name: m.name, Mode: 0o644, Size: int64(size), Typeflag: flag,
		}))
		if !m.dir {
			_, err := w.Write([]byte(m.body))
			require.NoError(t, err)
		}
	}
	require.NoError(t, w.Close())
	return buf.Bytes()
}

// tarBlock is the one header block tar writes before every member body.
const tarBlock = 512

// Archive-member fixture literals (goconst).
const (
	outReadmeName    = "README.md"
	archiveCanonical = "canonical"
	archiveUnasked   = "unasked"
)

// cutBody truncates a one-member archive inside its body: a body that ends
// where its header said it would not, the read error both tar readers must
// surface. tar writes one 512-byte header block before the body, so the cut
// point is the header plus the bytes delivered.
func cutBody(t *testing.T, body string, written int) []byte {
	t.Helper()
	stream := tarStream(t, tarMember{name: outFeaturesName, body: body, size: -1})
	return stream[:tarBlock+written]
}

// TestTarStreamReadersHandleOddArchives covers the stream shapes a server can
// hand back beyond the well-formed one: a directory member, a member no request
// asked for, a duplicate of one already read, a stream that is not tar at all,
// and a body that ends where its header said it would not.
func TestTarStreamReadersHandleOddArchives(t *testing.T) {
	stream := tarStream(
		t,
		tarMember{name: "docs/es", dir: true},
		tarMember{name: outFeaturesName, body: archiveCanonical, size: -1},
		tarMember{name: "other/FEATURES.md", body: "duplicate", size: -1},
		tarMember{name: outReadmeName, body: archiveUnasked, size: -1},
	)

	got, err := tarEntries(stream, []string{outFeaturesName})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{outFeaturesName: "canonical"}, got,
		"the first member of a name wins and an unasked one is skipped")

	_, err = tarEntries([]byte("this is not a tar stream"), []string{outFeaturesName})
	require.Error(t, err)

	_, err = tarEntries(cutBody(t, "hello world", 3), []string{outFeaturesName})
	require.Error(t, err)
	assert.True(t, errors.Is(err, io.ErrUnexpectedEOF), err.Error())

	_, err = tarEntry([]byte("this is not a tar stream"), outFeaturesName)
	require.Error(t, err)

	_, err = tarEntry(cutBody(t, "hello world", 3), outFeaturesName)
	require.Error(t, err)
	assert.True(t, errors.Is(err, io.ErrUnexpectedEOF), err.Error())
}

// TestNormalizeInheritedHandlesTildeFences pins the second fence dialect: a
// parent that fences its examples with ~~~ keeps them whole.
func TestNormalizeInheritedHandlesTildeFences(t *testing.T) {
	_, body := splitSPDX(upstreamDoc("### Entry", "", "~~~sh", "# install the thing", "## not a heading either", "make install", "~~~"))
	got := normalizeInherited(body)
	assert.Contains(t, got, "# install the thing")
	assert.Contains(t, got, "## not a heading either")
	assert.Contains(t, got, "### Entry")
}
