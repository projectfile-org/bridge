// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package hostmatch

import (
	"testing"

	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// Repository URLs reused across the case tables, one per rule under test.
const (
	urlGitHub     = "https://github.com/me/proj"
	urlGitLab     = "https://gitlab.com/me/proj"
	urlGitLabSelf = "https://gitlab.example.com/me/proj"
	urlCodeberg   = "https://codeberg.org/me/proj"
	urlSourcehut  = "https://git.sr.ht/~user/repo"
)

func TestResolve(t *testing.T) {
	cases := []struct {
		name     string
		url      string
		wantKind Kind
	}{
		{"github.com", urlGitHub, KindGitHub},
		{"github trailing slash", "https://github.com/me/proj/", KindGitHub},
		{"gitlab.com", urlGitLab, KindGitLab},
		{"gitlab nested group", "https://gitlab.com/grp/sub/proj", KindGitLab},
		{"gitlab self-hosted", urlGitLabSelf, KindGitLab},
		{"codeberg.org", urlCodeberg, KindForgejo},
		{"gitea.com", "https://gitea.com/me/proj", KindForgejo},
		{"forgejo self-hosted", "https://forgejo.example.com/me/proj", KindForgejo},
		{"gitea self-hosted", "https://gitea.example.com/me/proj", KindForgejo},
		{"sourcehut", urlSourcehut, KindSourcehut},
		{"unknown host", "https://example.com/me/proj", KindUnknown},
		{"empty url", "", KindUnknown},
		{"bad url", "://not-a-url", KindUnknown},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := ResolveKind(c.url)
			if got != c.wantKind {
				t.Fatalf("ResolveKind(%q) = %q, want %q", c.url, got, c.wantKind)
			}
		})
	}
}

func TestResolveTracker(t *testing.T) {
	cases := []struct {
		url  string
		want string
	}{
		{urlGitHub, "https://github.com/me/proj/issues"},
		{"https://github.com/me/proj/", "https://github.com/me/proj/issues"},
		{urlGitLab, "https://gitlab.com/me/proj/-/issues"},
		{urlGitLabSelf, "https://gitlab.example.com/me/proj/-/issues"},
		{urlCodeberg, "https://codeberg.org/me/proj/issues"},
		{urlSourcehut, "https://todo.sr.ht/~user/repo"},
	}
	for _, c := range cases {
		rule, _ := Resolve(c.url)
		if rule == nil {
			t.Fatalf("Resolve(%q) returned nil", c.url)
		}
		got := rule.Tracker(c.url)
		if got != c.want {
			t.Fatalf("tracker(%q) = %q, want %q", c.url, got, c.want)
		}
	}
}

func TestMatchesHost(t *testing.T) {
	cases := []struct {
		host, pattern string
		want          bool
	}{
		{HostGitHub, HostGitHub, true},
		{"www.github.com", HostGitHub, true},
		{"github.example.com", HostGitHub, false},      // not a subdomain of github.com
		{"gitlab.example.com", HostGitLabPrefix, true}, // prefix-with-dot match
		{HostGitLab, HostGitLabPrefix, true},           // also matches the prefix; Rules order ensures the exact rule wins first
		{"forgejo.foo.bar", HostForgejoPrefix, true},
	}
	for _, c := range cases {
		got := MatchesHost(c.host, c.pattern)
		if got != c.want {
			t.Errorf("MatchesHost(%q, %q) = %v, want %v", c.host, c.pattern, got, c.want)
		}
	}
}

// TestCapabilities pins the host→capability proposal the scanner writes into
// links[].tags. The interesting rows are the empty ones: a self-hosted
// instance and an unknown host must yield nothing, because a wrong `public`
// tag survives in the projectfile and renders a broken badge forever.
func TestCapabilities(t *testing.T) {
	cases := []struct {
		name string
		url  string
		want []string
	}{
		{"github.com", urlGitHub, crawlable},
		{"gitlab.com", urlGitLab, crawlable},
		{"codeberg.org", urlCodeberg, crawlable},
		{"gitea.com", "https://gitea.com/me/proj", crawlable},
		{"sourcehut is public but uncrawlable", urlSourcehut, []string{"public"}},
		{"gitlab self-hosted proposes nothing", urlGitLabSelf, nil},
		{"gitea self-hosted proposes nothing", "https://gitea.example.com/me/proj", nil},
		{"forgejo self-hosted proposes nothing", "https://forgejo.example.com/me/proj", nil},
		{"unknown host proposes nothing", "https://kiota.ch/me/proj", nil},
		{"empty url", "", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Capabilities(c.url)
			if len(got) != len(c.want) {
				t.Fatalf("Capabilities(%q) = %v, want %v", c.url, got, c.want)
			}
			for i := range c.want {
				if got[i] != c.want[i] {
					t.Fatalf("Capabilities(%q) = %v, want %v", c.url, got, c.want)
				}
			}
		})
	}
}

// TestCapabilitiesReturnsCopy guards the rule table against a caller that
// appends to what it got back. Rules is package state read by every scan.
func TestCapabilitiesReturnsCopy(t *testing.T) {
	got := Capabilities(urlGitHub)
	got[0] = "mutated"
	if again := Capabilities(urlGitHub); again[0] != pfmodel.TagPublic {
		t.Fatalf("rule table was mutated through a returned slice: %v", again)
	}
}
