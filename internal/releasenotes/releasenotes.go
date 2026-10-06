// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package releasenotes renders the human half of a release body from git alone.
package releasenotes

import (
	"context"
	"fmt"
	"os/exec"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/bridge/core"
	"projectfile.org/projectfile/bridge/internal/bridge/readme"
	"projectfile.org/projectfile/bridge/internal/derive"
)

// Options selects the range and the release the notes describe.
type Options struct {
	Dir     string
	Tag     string
	From    string
	To      string
	Prefix  string
	Note    string
	Timeout time.Duration
	Read    projectfile.ReadOptions
}

const (
	featuresDir     = "docs/features.d/"
	roadmapDir      = "docs/roadmap.d/"
	fragmentsDir    = "docs/release-notes.d/"
	vulnerabilities = ".projectfile/vulnerabilities.yaml"
	defaultTimeout  = 30 * time.Second
)

// sections are the shown commit types, in render order.
var sections = []struct{ kind, title string }{
	{"feat", "Features"},
	{"fix", "Bug fixes"},
	{"perf", "Performance"},
	{"revert", "Reverts"},
}

var (
	subjectRe  = regexp.MustCompile(`^([a-z]+)(?:\(([^)]*)\))?(!)?: (.+)$`)
	breakingRe = regexp.MustCompile(`(?m)^BREAKING[ -]CHANGE: ?`)
	trailerRe  = regexp.MustCompile(`^[A-Z][A-Za-z-]+: \S`)
	headingRe  = regexp.MustCompile(`^(#{1,4}) `)
	advisoryRe = regexp.MustCompile(`^(CVE|GHSA|GO|RUSTSEC|PYSEC)-`)
	linkRe     = regexp.MustCompile(`\]\(([^)\s]+)\)`)
	schemeRe   = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*:`)
)

type commit struct {
	hash, short, name, email, kind, scope, desc, breaking string
	bang                                                  bool
}

type git struct {
	dir     string
	timeout time.Duration
}

// run executes one git command bounded by the timeout.
func (g git) run(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), g.timeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, "git", append([]string{"-C", g.dir}, args...)...).Output() // #nosec G204 -- fixed git binary; refs are arguments, never a shell
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(ee.Stderr)))
		}
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return string(out), nil
}

// Render returns the Markdown body for the release in opts.
func Render(opts Options) (string, error) {
	if opts.To == "" {
		opts.To = "HEAD"
	}
	if opts.Timeout == 0 {
		opts.Timeout = defaultTimeout
	}
	g := git{dir: opts.Dir, timeout: opts.Timeout}
	if opts.From == "" {
		prev, err := previousTag(g, opts.To, opts.Tag, opts.Prefix)
		if err != nil {
			return "", err
		}
		opts.From = prev
	}
	genlog.Info("release notes range", "from", or(opts.From, "<root>"), "to", opts.To, "tag", or(opts.Tag, "<none>"))

	commits, err := readCommits(g, opts.From, opts.To)
	if err != nil {
		return "", err
	}
	base := opts.From
	if base == "" {
		if base, err = emptyTree(g); err != nil {
			return "", err
		}
	}

	var b strings.Builder
	summary, err := summaryLine(g, commits, opts.Dir, opts.From, opts.To)
	if err != nil {
		return "", err
	}
	b.WriteString(summary + "\n")
	if note := strings.TrimSpace(opts.Note); note != "" {
		b.WriteString("\n" + note + "\n")
	}
	writeBreaking(&b, commits)
	highlights, err := presentFragments(g, opts.To)
	if err != nil {
		return "", err
	}
	writeFragments(&b, "Highlights", highlights)
	added, err := changedFragments(g, base, opts.To, featuresDir, "A", opts.To)
	if err != nil {
		return "", err
	}
	if added, err = introduced(g, opts.From, opts.To, added); err != nil {
		return "", err
	}
	writeFragments(&b, "What’s new", added)
	delivered, err := changedFragments(g, base, opts.To, roadmapDir, "D", opts.From)
	if err != nil {
		return "", err
	}
	writeTitles(&b, "Roadmap delivered", delivered)
	install, err := pinnedBlocks(opts)
	if err != nil {
		return "", err
	}
	if install != "" {
		b.WriteString("\n" + install + "\n")
	}
	writeChanges(&b, commits)
	if err := writeSecurity(&b, g, opts.From, opts.To); err != nil {
		return "", err
	}
	if err := writeContributors(&b, g, commits, opts.From); err != nil {
		return "", err
	}
	return b.String(), nil
}

func or(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// previousTag picks the newest prefixed version below tag that this range follows.
func previousTag(g git, to, tag, prefix string) (string, error) {
	out, err := g.run("tag", "--merged", to, "--list", prefix+"[0-9]*")
	if err != nil {
		return "", err
	}
	cur, curOK := parseVersion(strings.TrimPrefix(tag, prefix))
	final := !curOK || cur.pre == ""
	best, bestName := version{}, ""
	for _, name := range strings.Fields(out) {
		v, ok := parseVersion(strings.TrimPrefix(name, prefix))
		switch {
		case !ok:
			genlog.Debug("tag skipped, not a version", "tag", name)
		case name == tag || (curOK && compare(v, cur) >= 0):
			genlog.Debug("tag skipped, not before the release", "tag", name, "release", tag)
		case final && v.pre != "":
			genlog.Debug("tag skipped, a final covers the previous final", "tag", name)
		case bestName == "" || compare(v, best) > 0:
			best, bestName = v, name
		}
	}
	genlog.Info("previous tag", "tag", or(bestName, "<none>"), "final", final)
	return bestName, nil
}

type version struct {
	core [3]int
	pre  string
}

func parseVersion(s string) (version, bool) {
	s, _, _ = strings.Cut(s, "+")
	main, pre, _ := strings.Cut(s, "-")
	parts := strings.Split(main, ".")
	if len(parts) != 3 {
		return version{}, false
	}
	var v version
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return version{}, false
		}
		v.core[i] = n
	}
	v.pre = pre
	return v, true
}

// compare orders versions per SemVer 2.0.0 §11.
func compare(a, b version) int {
	for i := range a.core {
		if a.core[i] != b.core[i] {
			return a.core[i] - b.core[i]
		}
	}
	switch {
	case a.pre == b.pre:
		return 0
	case a.pre == "":
		return 1
	case b.pre == "":
		return -1
	}
	ap, bp := strings.Split(a.pre, "."), strings.Split(b.pre, ".")
	for i := 0; i < len(ap) && i < len(bp); i++ {
		an, aErr := strconv.Atoi(ap[i])
		bn, bErr := strconv.Atoi(bp[i])
		switch {
		case aErr == nil && bErr == nil && an != bn:
			return an - bn
		case aErr == nil && bErr != nil:
			return -1
		case aErr != nil && bErr == nil:
			return 1
		case aErr != nil && ap[i] != bp[i]:
			return strings.Compare(ap[i], bp[i])
		}
	}
	return len(ap) - len(bp)
}

func emptyTree(g git) (string, error) {
	out, err := g.run("hash-object", "-t", "tree", "/dev/null")
	return strings.TrimSpace(out), err
}

func rangeArg(from, to string) string {
	if from == "" {
		return to
	}
	return from + ".." + to
}

func readCommits(g git, from, to string) ([]commit, error) {
	out, err := g.run("log", "--no-merges", "--reverse", "--format=%H%x1f%h%x1f%aN%x1f%aE%x1f%s%x1f%b%x1e", rangeArg(from, to))
	if err != nil {
		return nil, err
	}
	var commits []commit
	for _, rec := range strings.Split(out, "\x1e") {
		f := strings.Split(strings.TrimLeft(rec, "\n"), "\x1f")
		if len(f) != 6 {
			continue
		}
		c := commit{hash: f[0], short: f[1], name: f[2], email: f[3]}
		m := subjectRe.FindStringSubmatch(f[4])
		if m == nil {
			genlog.Warn("commit is not conventional, left out of the notes", "commit", c.short, "subject", f[4])
			commits = append(commits, c)
			continue
		}
		c.kind, c.scope, c.bang, c.desc = m[1], m[2], m[3] == "!", m[4]
		if loc := breakingRe.FindStringIndex(f[5]); loc != nil {
			c.breaking = stripTrailers(f[5][loc[1]:])
		}
		commits = append(commits, c)
	}
	genlog.Info("commits read", "range", rangeArg(from, to), "count", len(commits))
	return commits, nil
}

// stripTrailers drops the git trailer lines that close a footer.
func stripTrailers(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n "), "\n")
	for len(lines) > 0 && trailerRe.MatchString(lines[len(lines)-1]) {
		lines = lines[:len(lines)-1]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// elapsed is the time between two tags, whole in every unit the summary template may pick.
type elapsed struct{ Minutes, Hours, Days, Weeks, Months, Years int }

func newElapsed(seconds int64) elapsed {
	days := int(seconds / 86400)
	return elapsed{int(seconds / 60), int(seconds / 3600), days, days / 7, days / 30, days / 365}
}

func summaryLine(g git, commits []commit, dir, from, to string) (string, error) {
	people := map[string]bool{}
	for _, c := range commits {
		people[c.email] = true
	}
	data := struct {
		Commits, Contributors int
		From                  string
		Elapsed               elapsed
	}{Commits: len(commits), Contributors: len(people), From: from}
	if from != "" {
		out, err := g.run("log", "--max-count=1", "--format=%ct", from, "--")
		if err != nil {
			return "", err
		}
		start, _ := strconv.ParseInt(strings.TrimSpace(out), 10, 64)
		if out, err = g.run("log", "--max-count=1", "--format=%ct", to, "--"); err != nil {
			return "", err
		}
		end, _ := strconv.ParseInt(strings.TrimSpace(out), 10, 64)
		data.Elapsed = newElapsed(end - start)
		genlog.Debug("summary elapsed", "from", from, "to", to, "seconds", end-start)
	}
	out, err := core.Render(dir, summaryTemplate, data)
	return strings.TrimSpace(string(out)), err
}

func label(c commit) string {
	if c.scope == "" {
		return "- " + c.desc
	}
	return "- **" + c.scope + ":** " + c.desc
}

func item(c commit) string { return label(c) + " (" + c.short + ")" }

func writeBreaking(b *strings.Builder, commits []commit) {
	var lines []string
	for _, c := range commits {
		if !c.bang && c.breaking == "" {
			continue
		}
		genlog.Debug("breaking change", "commit", c.short, "footer", c.breaking != "")
		line := item(c)
		if c.breaking != "" {
			line += "\n\n" + indent(c.breaking, "  ")
		}
		lines = append(lines, line)
	}
	writeSection(b, "Upgrade notes", lines, "\n\n")
}

func indent(s, pad string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if l != "" {
			lines[i] = pad + l
		}
	}
	return strings.Join(lines, "\n")
}

func writeSection(b *strings.Builder, title string, lines []string, sep string) {
	if len(lines) == 0 {
		genlog.Debug("section omitted, empty", "section", title)
		return
	}
	b.WriteString("\n## " + title + "\n\n" + strings.Join(lines, sep) + "\n")
}

func writeChanges(b *strings.Builder, commits []commit) {
	deps := 0
	for _, c := range commits {
		if c.kind == "chore" && c.scope == "deps" {
			deps++
		}
	}
	for _, s := range sections {
		var labels []string
		hashes := map[string][]string{}
		for _, c := range commits {
			if c.kind != s.kind || c.bang || c.breaking != "" {
				continue
			}
			l := label(c)
			if hashes[l] == nil {
				labels = append(labels, l)
			}
			hashes[l] = append(hashes[l], c.short)
		}
		var lines []string
		for _, l := range labels {
			lines = append(lines, l+" ("+strings.Join(hashes[l], ", ")+")")
		}
		writeSection(b, s.title, lines, "\n")
	}
	if deps > 0 {
		writeSection(b, "Dependencies", []string{"- " + core.Plural(deps, "dependency update", "dependency updates")}, "\n")
	}
}

type fragment struct{ path, text string }

// changedFragments reads the dir/*.md files the range adds (A) or deletes (D), at ref.
func changedFragments(g git, from, to, dir, filter, at string) ([]fragment, error) {
	out, err := g.run("diff", "--name-only", "--no-renames", "--diff-filter="+filter, from, to, "--", dir)
	if err != nil {
		return nil, err
	}
	var own []string
	for _, p := range strings.Fields(out) {
		if path.Dir(p)+"/" != dir {
			genlog.Debug("fragment skipped, nested", "path", p)
			continue
		}
		own = append(own, p)
	}
	return readFragments(g, at, own)
}

// introduced keeps the fragments a feat commit added; any other type documents an existing feature.
func introduced(g git, from, to string, frags []fragment) ([]fragment, error) {
	var kept []fragment
	for _, f := range frags {
		subject, err := g.run("log", "--diff-filter=A", "--max-count=1", "--format=%s", rangeArg(from, to), "--", f.path)
		if err != nil {
			return nil, err
		}
		m := subjectRe.FindStringSubmatch(strings.TrimSpace(subject))
		if m == nil || m[1] != "feat" {
			genlog.Debug("fragment skipped, not added by a feat commit", "path", f.path, "subject", strings.TrimSpace(subject))
			continue
		}
		kept = append(kept, f)
	}
	return kept, nil
}

func presentFragments(g git, to string) ([]fragment, error) {
	out, err := g.run("ls-tree", "--name-only", to, fragmentsDir)
	if err != nil {
		return nil, err
	}
	return readFragments(g, to, strings.Fields(out))
}

func readFragments(g git, ref string, paths []string) ([]fragment, error) {
	var frags []fragment
	slices.Sort(paths)
	for _, p := range paths {
		if !strings.HasSuffix(p, ".md") {
			genlog.Debug("fragment skipped, not Markdown", "path", p)
			continue
		}
		text, err := g.run("show", ref+":"+p)
		if err != nil {
			return nil, err
		}
		genlog.Debug("fragment read", "path", p, "ref", ref)
		frags = append(frags, fragment{p, demote(text, path.Dir(p))})
	}
	return frags, nil
}

// demote strips the licence comment, lowers headings two levels and roots relative links, outside fences.
func demote(text, dir string) string {
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "<!--") {
		if _, rest, ok := strings.Cut(text, "-->"); ok {
			text = strings.TrimSpace(rest)
		}
	}
	lines := strings.Split(text, "\n")
	fenced := false
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "```") {
			fenced = !fenced
		}
		if !fenced {
			lines[i] = linkRe.ReplaceAllStringFunc(headingRe.ReplaceAllString(l, "$1## "), func(m string) string {
				return "](" + rootLink(linkRe.FindStringSubmatch(m)[1], dir) + ")"
			})
		}
	}
	return strings.Join(lines, "\n")
}

// rootLink rewrites a link relative to dir as relative to the repository root.
func rootLink(target, dir string) string {
	if target == "" || strings.HasPrefix(target, "#") || strings.HasPrefix(target, "/") || schemeRe.MatchString(target) {
		return target
	}
	file, anchor, hasAnchor := strings.Cut(target, "#")
	rooted := path.Join(dir, file)
	if strings.HasPrefix(rooted, "../") {
		genlog.Debug("link left alone, outside the repository", "target", target, "dir", dir)
		return target
	}
	if hasAnchor {
		rooted += "#" + anchor
	}
	return rooted
}

func writeFragments(b *strings.Builder, title string, frags []fragment) {
	var parts []string
	for _, f := range frags {
		parts = append(parts, f.text)
	}
	writeSection(b, title, parts, "\n\n")
}

// writeTitles lists each fragment by its heading alone.
func writeTitles(b *strings.Builder, title string, frags []fragment) {
	var lines []string
	for _, f := range frags {
		heading, _, _ := strings.Cut(f.text, "\n")
		lines = append(lines, "- "+strings.TrimLeft(heading, "# "))
	}
	writeSection(b, title, lines, "\n")
}

func suppressed(g git, ref string) (map[string]bool, error) {
	ids := map[string]bool{}
	if ref == "" {
		return ids, nil
	}
	listed, err := g.run("ls-tree", "--name-only", ref, "--", vulnerabilities)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(listed) == "" {
		genlog.Debug("no suppression file", "ref", ref, "path", vulnerabilities)
		return ids, nil
	}
	text, err := g.run("show", ref+":"+vulnerabilities)
	if err != nil {
		return nil, err
	}
	var doc struct {
		Org struct {
			Projectfile struct {
				Vulnerabilities struct {
					Suppress []struct {
						ID string `yaml:"id"`
					} `yaml:"suppress"`
				} `yaml:"vulnerabilities"`
			} `yaml:"projectfile"`
		} `yaml:"org"`
	}
	if err := yaml.Unmarshal([]byte(text), &doc); err != nil {
		return nil, fmt.Errorf("parse %s at %s: %w", vulnerabilities, ref, err)
	}
	for _, s := range doc.Org.Projectfile.Vulnerabilities.Suppress {
		if !advisoryRe.MatchString(s.ID) {
			genlog.Debug("suppression skipped, not an advisory", "id", s.ID, "ref", ref)
			continue
		}
		ids[s.ID] = true
	}
	return ids, nil
}

func writeSecurity(b *strings.Builder, g git, from, to string) error {
	before, err := suppressed(g, from)
	if err != nil {
		return err
	}
	after, err := suppressed(g, to)
	if err != nil {
		return err
	}
	var lines []string
	for _, s := range []struct {
		label string
		have  map[string]bool
		lack  map[string]bool
	}{
		{"No longer suppressed", before, after},
		{"Newly suppressed", after, before},
	} {
		var ids []string
		for id := range s.have {
			if !s.lack[id] {
				ids = append(ids, id)
			}
		}
		slices.Sort(ids)
		genlog.Debug("suppression delta", "label", s.label, "count", len(ids))
		if len(ids) > 0 {
			lines = append(lines, "- "+s.label+": "+strings.Join(ids, ", "))
		}
	}
	writeSection(b, "Security", lines, "\n")
	return nil
}

func writeContributors(b *strings.Builder, g git, commits []commit, from string) error {
	if from == "" {
		genlog.Debug("new contributors omitted, first release")
		return nil
	}
	out, err := g.run("log", "--format=%aE", from)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, e := range strings.Fields(out) {
		seen[e] = true
	}
	var lines []string
	for _, c := range commits {
		if seen[c.email] {
			continue
		}
		seen[c.email] = true
		lines = append(lines, "- "+c.name+" ("+c.short+")")
	}
	writeSection(b, "New contributors", lines, "\n")
	return nil
}

// pinnedBlocks renders the README installation and usage blocks with every latest pinned to this release.
func pinnedBlocks(opts Options) (string, error) {
	if opts.Tag == "" {
		genlog.Debug("install blocks omitted, no release tag")
		return "", nil
	}
	pf, _, err := projectfile.ReadWithOptions(opts.Dir, opts.Read)
	if err != nil {
		genlog.Warn("install blocks omitted, projectfile unreadable", "dir", opts.Dir, "err", err.Error())
		return "", nil
	}
	version := strings.TrimPrefix(opts.Tag, opts.Prefix)
	for _, pin := range []struct{ value, path string }{
		{version, "org.projectfile.image.tag"},
		{opts.Tag, "org.projectfile.readme.tag"},
		{"download/" + opts.Tag, "org.projectfile.readme.download"},
	} {
		setPath(pf.Extensions, pin.value, strings.Split(pin.path, ".")...)
		genlog.Debug("install pinned", "path", pin.path, "value", pin.value)
	}
	derive.AddVirtual(pf)
	return readme.Blocks(pf, opts.Dir, "installation", "usage")
}

// setPath stores value at the nested map path, creating the maps it lacks.
func setPath(m map[string]any, value string, path ...string) {
	for _, key := range path[:len(path)-1] {
		next, ok := m[key].(map[string]any)
		if !ok {
			next = map[string]any{}
			m[key] = next
		}
		m = next
	}
	m[path[len(path)-1]] = value
}
