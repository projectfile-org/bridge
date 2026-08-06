// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package git derives projectfile metadata from a git working tree:
// authors (from git log), origin URL + default branch (from remote), and
// project lifecycle dates (from first/last commit). Shells out to the host
// `git` binary via os/exec — no new Go dependencies, and every container we
// ship already includes git per the workspace conventions.
package git

import (
	"bytes"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/forge/hostmatch"
	"projectfile.org/projectfile/bridge/internal/scanners/core"
	"projectfile.org/projectfile/bridge/internal/source"
)

// Scanner is the composite git scanner. It delegates to the three
// sub-scanners (authors, remotes, dates) and merges their results.
// Not registered in the core registry — the command layer expands "git"
// to the individual sub-scanner names.
type Scanner struct{}

func (Scanner) Name() string { return "git" }

func (Scanner) Detect(root string) bool { return detectGit(root) }

// Scan runs all three sub-scanners and merges their output.
func (Scanner) Scan(root string) (*source.Partial, []core.Hit, error) {
	subs := []interface {
		Scan(root string) (*source.Partial, []core.Hit, error)
	}{
		authorsScanner{},
		remotesScanner{},
		datesScanner{},
	}
	merged := &source.Partial{}
	var allHits []core.Hit
	for _, s := range subs {
		p, h, err := s.Scan(root)
		if err != nil {
			continue
		}
		if p != nil {
			merged = source.MergePartials(merged, p)
		}
		allHits = append(allHits, h...)
	}
	return merged, allHits, nil
}

// detectGit returns true iff <root>/.git exists. The entry can be either a
// directory (regular repo) or a file (worktree / submodule pointer file),
// so a plain Stat is correct — the type doesn't matter.
func detectGit(root string) bool {
	_, err := os.Stat(filepath.Join(root, ".git"))
	return err == nil
}

// run executes a pre-built git command from `root`, capturing stdout and
// swallowing stderr (we never need it — failure is signalled by a non-nil
// error). The caller constructs the command with literal arguments only, so
// no caller-derived value flows into exec.Command — there is no injection
// surface for git to misinterpret as an option.
func run(cmd *exec.Cmd, root string) (string, error) {
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimRight(out.String(), "\n"), nil
}

// forgePageURL converts a git clone URL to the matching http forge landing
// page for hosts hostmatch recognises. Handles three real-world shapes:
//
//	git@github.com:acme/repo.git    → https://github.com/acme/repo
//	git+ssh://github.com/acme/repo  → https://github.com/acme/repo
//	https://github.com/acme/repo.git → https://github.com/acme/repo
//
// Returns "" when the host is unknown to hostmatch or the URL can't be
// parsed. kinds is the optional user-defined host→kind map.
func forgePageURL(remote string, kinds map[string]string) string {
	host, path := parseForgeLocator(remote)
	if host == "" || path == "" {
		return ""
	}
	if hostmatch.ResolveKindWithKinds("https://"+host, kinds) == hostmatch.KindUnknown {
		return ""
	}
	path = strings.TrimSuffix(path, ".git")
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		return ""
	}
	return "https://" + host + "/" + path
}

// parseForgeLocator extracts (host, path) from a git clone URL.
func parseForgeLocator(remote string) (host, path string) {
	if strings.HasPrefix(remote, "git@") || (!strings.Contains(remote, "://") && strings.Contains(remote, ":")) {
		rest := strings.TrimPrefix(remote, "git@")
		i := strings.Index(rest, ":")
		if i < 0 {
			return "", ""
		}
		return rest[:i], strings.TrimPrefix(rest[i+1:], "/")
	}
	u, err := url.Parse(remote)
	if err != nil || u.Host == "" {
		return "", ""
	}
	return u.Host, strings.TrimPrefix(u.Path, "/")
}

// primaryRepoIndex returns the index of the entry marked origin (role == origin), or -1.
func primaryRepoIndex(repos []projectfile.Repository) int {
	for i, r := range repos {
		if r.Role == projectfile.RepositoryRoleOrigin {
			return i
		}
	}
	return -1
}

// remoteEntry pairs a remote name with its fetch URL.
type remoteEntry struct {
	Name string
	URL  string
}

// remoteEntries lists the configured remotes with their fetch URLs in one
// literal-argument invocation (`git remote -v`). A single command replaces
// the previous per-remote `git remote get-url <name>` loop, so no remote
// name — which is data, not a literal — ever reaches exec.Command. Each
// remote appears twice in `git remote -v` output (fetch + push); the first
// occurrence (fetch) wins, matching `git remote get-url`'s default.
func remoteEntries(root string) []remoteEntry {
	out, err := run(exec.Command("git", "remote", "-v"), root)
	if err != nil || out == "" {
		return nil
	}
	seen := map[string]bool{}
	var entries []remoteEntry
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || seen[fields[0]] {
			continue
		}
		seen[fields[0]] = true
		entries = append(entries, remoteEntry{Name: fields[0], URL: fields[1]})
	}
	return entries
}

// defaultBranch resolves the symbolic ref for origin's HEAD.
func defaultBranch(root string) string {
	out, err := run(exec.Command("git", "symbolic-ref", "--short", "refs/remotes/origin/HEAD"), root)
	if err != nil {
		return ""
	}
	const prefix = "origin/"
	return strings.TrimPrefix(out, prefix)
}

// firstCommitDate returns YYYY-MM-DD of the chronologically earliest commit.
func firstCommitDate(root string) string {
	out, err := run(exec.Command("git", "log", "--max-parents=0", "--format=%aI", "HEAD"), root)
	if err != nil || out == "" {
		return ""
	}
	earliest := ""
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		if earliest == "" || line < earliest {
			earliest = line
		}
	}
	return isoDate(earliest)
}

// lastCommitDate returns YYYY-MM-DD of HEAD.
func lastCommitDate(root string) string {
	out, err := run(exec.Command("git", "log", "-1", "--format=%aI", "HEAD"), root)
	if err != nil || out == "" {
		return ""
	}
	return isoDate(strings.TrimSpace(out))
}

// isoDate slices the YYYY-MM-DD prefix off a %aI timestamp.
func isoDate(s string) string {
	if len(s) < 10 {
		return ""
	}
	return s[:10]
}

// hostFromURL extracts the host from an https URL.
func hostFromURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return ""
	}
	return u.Host
}

// displayName picks the most useful label for logging a person hit.
func displayName(p source.PersonEntry) string {
	switch {
	case p.GivenNames != "" && p.FamilyNames != "":
		return p.GivenNames + " " + p.FamilyNames
	case p.Name != "":
		return p.Name
	default:
		return p.Email
	}
}
