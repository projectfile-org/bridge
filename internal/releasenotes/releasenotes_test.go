// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package releasenotes

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// isolate drops every inherited GIT_* variable so a hook's GIT_DIR never reaches the fixture.
func isolate(t *testing.T) {
	t.Helper()
	for _, kv := range os.Environ() {
		if k, _, _ := strings.Cut(kv, "="); strings.HasPrefix(k, "GIT_") {
			t.Setenv(k, "")
			require.NoError(t, os.Unsetenv(k))
		}
	}
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
}

type repo struct {
	t   *testing.T
	dir string
	day int
}

func newRepo(t *testing.T) *repo {
	isolate(t)
	r := &repo{t: t, dir: t.TempDir()}
	r.git("Ann", "init", "--quiet", "--initial-branch=main")
	return r
}

// git runs one command as author, one fixture day after the previous commit.
func (r *repo) git(author string, args ...string) {
	r.t.Helper()
	date := "2026-01-01T00:00:00Z"
	if args[0] == "commit" {
		r.day++
		date = fmt.Sprintf("2026-01-%02dT00:00:00Z", r.day)
	}
	cmd := exec.Command("git", append([]string{"-c", "commit.gpgsign=false", "-c", "tag.gpgsign=false"}, args...)...)
	cmd.Dir = r.dir
	email := strings.ToLower(author) + "@example.org"
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME="+author, "GIT_AUTHOR_EMAIL="+email,
		"GIT_COMMITTER_NAME="+author, "GIT_COMMITTER_EMAIL="+email,
		"GIT_AUTHOR_DATE="+date, "GIT_COMMITTER_DATE="+date)
	out, err := cmd.CombinedOutput()
	require.NoError(r.t, err, "git %v: %s", args, out)
}

// commit writes files (an empty value deletes one) and commits them as author.
func (r *repo) commit(author, message string, files map[string]string) {
	r.t.Helper()
	for path, body := range files {
		full := filepath.Join(r.dir, path)
		if body == "" {
			require.NoError(r.t, os.Remove(full))
			continue
		}
		require.NoError(r.t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(r.t, os.WriteFile(full, []byte(body), 0o644))
	}
	r.git(author, "add", "--all")
	r.git(author, "commit", "--quiet", "--allow-empty", "--message", message)
}

const licence = "<!--\nSPDX-License-Identifier: MIT\n-->\n\n"

const first = "1.0.0"

const suppress = "org:\n  projectfile:\n    vulnerabilities:\n      suppress:\n"

func TestRenderFirstRelease(t *testing.T) {
	r := newRepo(t)
	r.commit("Ann", "feat: first light", map[string]string{"docs/features.d/one.md": licence + "# Fast\n\n- Starts at once.\n"})
	r.commit("Ann", "ci: wire the pipeline", nil)

	got, err := Render(Options{Dir: r.dir, Tag: "0.1.0"})
	require.NoError(t, err)
	assert.Equal(t, "2 commits, 1 contributor.\n"+
		"\n## What’s new\n\n### Fast\n\n- Starts at once.\n"+
		"\n## Features\n\n- first light (", got[:strings.LastIndex(got, "(")+1])
	assert.NotContains(t, got, "New contributors")
	assert.NotContains(t, got, "pipeline")
}

func TestRenderRange(t *testing.T) {
	r := newRepo(t)
	r.commit("Ann", "feat: base", map[string]string{
		"docs/roadmap.d/later.md":           licence + "# Later\n\n- Soon.\n",
		".projectfile/vulnerabilities.yaml": suppress + "        - id: CVE-2026-1\n        - id: CVE-2026-2\n",
	})
	r.git("Ann", "tag", first)
	r.commit("Ann", "feat(cli)!: drop the old flag\n\nBREAKING CHANGE: use the new flag:\n\n```sh\ntool --new\n```\n\nSigned-off-by: Ann <ann@example.org>", nil)
	r.commit("Bob", "fix(api): stop leaking sockets", map[string]string{
		"docs/roadmap.d/later.md":           "",
		"docs/features.d/later.md":          licence + "# Later\n\n- Here.\n",
		".projectfile/vulnerabilities.yaml": suppress + "        - id: CVE-2026-2\n        - id: CVE-2026-3\n",
	})
	r.commit("Ann", "docs: refresh help", nil)
	r.commit("Ann", "docs: refresh help", nil)
	r.commit("Ann", "chore(deps): bump a", nil)
	r.commit("Ann", "chore(deps): bump b", nil)
	r.commit("Ann", "test: cover it", nil)
	r.commit("Ann", "not conventional", map[string]string{"docs/release-notes.d/10-thanks.md": licence + "# Thanks\n\nTo everyone.\n"})

	got, err := Render(Options{Dir: r.dir, Tag: "2.0.0", Note: "Smaller and faster."})
	require.NoError(t, err)
	for _, want := range []string{
		"8 commits, 2 contributors, 8 days since 1.0.0.\n\nSmaller and faster.\n",
		"## Upgrade notes\n\n- **cli:** drop the old flag (",
		"\n\n  use the new flag:\n\n  ```sh\n  tool --new\n  ```\n",
		"## What’s new\n\n### Later\n\n- Here.\n",
		"## Roadmap delivered\n\n### Later\n\n- Soon.\n",
		"## Highlights\n\n### Thanks\n\nTo everyone.\n",
		"## Bug fixes\n\n- **api:** stop leaking sockets (",
		"## Dependencies\n\n- 2 dependency updates\n",
		"## Security\n\n- No longer suppressed: CVE-2026-1\n- Newly suppressed: CVE-2026-3\n",
		"## New contributors\n\n- Bob (",
	} {
		assert.Contains(t, got, want)
	}
	assert.Regexp(t, `- refresh help \([0-9a-f]+, [0-9a-f]+\)`, got)
	assert.NotContains(t, got, "## Features")
	assert.NotContains(t, got, "Signed-off-by")
	assert.NotContains(t, got, "cover it")
	assert.Less(t, strings.Index(got, "Upgrade notes"), strings.Index(got, "What’s new"))
}

func TestPreviousTag(t *testing.T) {
	r := newRepo(t)
	const rc1 = "1.1.0-rc.1"
	for _, tag := range []string{first, "1.1.0-rc.0", rc1, "v9.9.9"} {
		r.commit("Ann", "feat: "+tag, nil)
		r.git("Ann", "tag", tag)
	}
	g := git{dir: r.dir, timeout: defaultTimeout}
	for tag, want := range map[string]string{
		"1.1.0-rc.2": rc1,
		"1.1.0":      first,
		rc1:          "1.1.0-rc.0",
		first:        "",
		"":           first,
	} {
		got, err := previousTag(g, "HEAD", tag, "")
		require.NoError(t, err)
		assert.Equal(t, want, got, "tag=%s", tag)
	}
	got, err := previousTag(g, "HEAD", "v10.0.0", "v")
	require.NoError(t, err)
	assert.Equal(t, "v9.9.9", got)
}

func TestCompare(t *testing.T) {
	order := []string{"1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-alpha.beta", "1.0.0-beta", "1.0.0-beta.2", "1.0.0-beta.11", "1.0.0-rc.1", first, "1.0.1", "1.10.0"}
	for i := 1; i < len(order); i++ {
		a, _ := parseVersion(order[i-1])
		b, _ := parseVersion(order[i])
		assert.Negative(t, compare(a, b), "%s < %s", order[i-1], order[i])
	}
}

func TestDemoteKeepsFences(t *testing.T) {
	assert.Equal(t, "### Title\n\n```sh\n# comment\n```\n\n#### Sub", demote(licence+"# Title\n\n```sh\n# comment\n```\n\n## Sub\n"))
}
