// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

// Package advisory maps vulnerability IDs to their database pages, orders them, and reads their titles from osv.dev.
package advisory

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"kiota.ch/projectfile/core/v2/pkg/genlog"
	"kiota.ch/projectfile/core/v2/pkg/netfetch"
)

// osvEndpoint is the OSV REST path a vulnerability ID is appended to.
const osvEndpoint = "https://api.osv.dev/v1/vulns/"

// maxNameRunes caps a title so it stays one readable comment line.
const maxNameRunes = 120

// maxBody bounds an OSV record read.
const maxBody = 4 << 20

// idPattern accepts an advisory ID shape (CVE-…, GHSA-…, GO-…, RUSTSEC-…), never a bare package name.
var idPattern = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*-[A-Za-z0-9._-]+$`)

// databases lists the official page per ID prefix; any other advisory ID falls back to osv.dev.
var databases = []struct{ prefix, page string }{
	{"CVE-", "https://nvd.nist.gov/vuln/detail/"},
	{"GHSA-", "https://github.com/advisories/"},
	{"GO-", "https://pkg.go.dev/vuln/"},
}

// Key returns the advisory ID of a suppress entry, dropping an audit-ci `|path` suffix; "" when it names no advisory.
func Key(id string) string {
	id, _, _ = strings.Cut(id, "|")
	if !idPattern.MatchString(id) {
		return ""
	}
	return id
}

// URL returns the official database page of an advisory ID, or "" when the ID names no advisory.
func URL(id string) string {
	key := Key(id)
	if key == "" {
		return ""
	}
	for _, db := range databases {
		if strings.HasPrefix(key, db.prefix) {
			return db.page + key
		}
	}
	return "https://osv.dev/vulnerability/" + key
}

// Compare orders IDs alphanumerically: digit runs compare by value, so CVE-2026-9999 sorts before CVE-2026-10000.
func Compare(a, b string) int {
	x, y := a, b
	for x != "" && y != "" {
		dx, dy := digitRun(x), digitRun(y)
		if dx > 0 && dy > 0 {
			nx, ny := strings.TrimLeft(x[:dx], "0"), strings.TrimLeft(y[:dy], "0")
			if c := cmp.Or(cmp.Compare(len(nx), len(ny)), strings.Compare(nx, ny)); c != 0 {
				return c
			}
			x, y = x[dx:], y[dy:]
			continue
		}
		if x[0] != y[0] {
			return cmp.Compare(x[0], y[0])
		}
		x, y = x[1:], y[1:]
	}
	return cmp.Or(cmp.Compare(len(x), len(y)), strings.Compare(a, b))
}

// digitRun returns the length of the leading ASCII digit run of s.
func digitRun(s string) int {
	n := 0
	for n < len(s) && s[n] >= '0' && s[n] <= '9' {
		n++
	}
	return n
}

// OneLine folds whitespace and caps the length so a title fits on a single comment line.
func OneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if utf8.RuneCountInString(s) <= maxNameRunes {
		return s
	}
	r := []rune(s)
	return strings.TrimSpace(string(r[:maxNameRunes-1])) + "…"
}

// Name reads an advisory’s title from the local cache, else from osv.dev unless offline; a record OSV does not hold yields "" and no error.
func Name(id string, offline bool) (string, error) {
	key := Key(id)
	if key == "" {
		return "", nil
	}
	if name := loadCached(key); name != "" {
		genlog.Debug("advisory name from cache", "id", key, "name", name)
		return name, nil
	}
	if offline {
		genlog.Debug("advisory name not cached; offline, skipping osv.dev", "id", key)
		return "", nil
	}
	name, err := fetchName(osvEndpoint, key, &http.Client{})
	if err == nil && name != "" {
		storeCached(key, name)
	}
	return name, err
}

// CacheDir is the osv title slot every pf-* binary shares: $XDG_CACHE_HOME/pf/osv.
func CacheDir() string {
	base := os.Getenv("XDG_CACHE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			home = "."
		}
		base = filepath.Join(home, ".cache")
	}
	return filepath.Join(base, "pf", "osv")
}

// loadCached returns the cached title of key, or "" on a miss.
func loadCached(key string) string {
	b, err := os.ReadFile(filepath.Join(CacheDir(), key+".txt")) // #nosec G304 -- key matches idPattern, dir is XDG-derived
	if err != nil {
		return ""
	}
	return OneLine(string(b))
}

// storeCached writes the title through a temp file and a rename, so a parallel reader never sees half an entry.
func storeCached(key, name string) {
	dir := CacheDir()
	if err := os.MkdirAll(dir, 0o750); err != nil { // #nosec G301 -- owner-only cache dir under XDG
		genlog.Warn("advisory cache write skipped", "dir", dir, "error", err.Error())
		return
	}
	tmp, err := os.CreateTemp(dir, key+".*.tmp")
	if err != nil {
		genlog.Warn("advisory cache write skipped", "id", key, "error", err.Error())
		return
	}
	_, werr := tmp.WriteString(name + "\n")
	cerr := tmp.Close()
	if werr != nil || cerr != nil {
		_ = os.Remove(tmp.Name())
		genlog.Warn("advisory cache write failed", "id", key, "error", errors.Join(werr, cerr).Error())
		return
	}
	if err := os.Rename(tmp.Name(), filepath.Join(dir, key+".txt")); err != nil {
		_ = os.Remove(tmp.Name())
		genlog.Warn("advisory cache rename failed", "id", key, "error", err.Error())
	}
}

// fetchName GETs endpoint+id through netfetch and derives the title from the record’s summary, else its details.
func fetchName(endpoint, id string, client *http.Client) (string, error) {
	key := Key(id)
	if key == "" {
		genlog.Debug("advisory name skipped: not an advisory ID", "id", id)
		return "", nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), netfetch.Timeout()*time.Duration(netfetch.Attempts()))
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+url.PathEscape(key), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "projectfile")
	resp, err := netfetch.Do(client, req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		genlog.Debug("advisory name unknown to osv.dev", "id", key)
		return "", nil
	}
	if resp.StatusCode/100 != 2 {
		return "", fmt.Errorf("osv.dev %s: HTTP %d", key, resp.StatusCode)
	}
	var rec struct {
		Summary string `json:"summary"`
		Details string `json:"details"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(&rec); err != nil {
		return "", fmt.Errorf("osv.dev %s: decode: %w", key, err)
	}
	name := rec.Summary
	if strings.TrimSpace(name) == "" {
		name = firstSentence(rec.Details)
	}
	genlog.Debug("advisory name resolved", "id", key, "name", OneLine(name))
	return OneLine(name), nil
}

// firstSentence returns the first line of s, cut after its first full stop.
func firstSentence(s string) string {
	s, _, _ = strings.Cut(strings.TrimSpace(s), "\n")
	if i := strings.Index(s, ". "); i >= 0 {
		return s[:i+1]
	}
	return s
}
