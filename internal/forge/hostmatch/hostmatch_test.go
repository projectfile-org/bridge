// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package hostmatch

import "testing"

func TestResolve(t *testing.T) {
	cases := []struct {
		name     string
		url      string
		wantKind Kind
	}{
		{"github.com", "https://github.com/me/proj", KindGitHub},
		{"github trailing slash", "https://github.com/me/proj/", KindGitHub},
		{"gitlab.com", "https://gitlab.com/me/proj", KindGitLab},
		{"gitlab nested group", "https://gitlab.com/grp/sub/proj", KindGitLab},
		{"gitlab self-hosted", "https://gitlab.example.com/me/proj", KindGitLab},
		{"codeberg.org", "https://codeberg.org/me/proj", KindForgejo},
		{"gitea.com", "https://gitea.com/me/proj", KindForgejo},
		{"forgejo self-hosted", "https://forgejo.example.com/me/proj", KindForgejo},
		{"gitea self-hosted", "https://gitea.example.com/me/proj", KindForgejo},
		{"sourcehut", "https://git.sr.ht/~user/repo", KindSourcehut},
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
		{"https://github.com/me/proj", "https://github.com/me/proj/issues"},
		{"https://github.com/me/proj/", "https://github.com/me/proj/issues"},
		{"https://gitlab.com/me/proj", "https://gitlab.com/me/proj/-/issues"},
		{"https://gitlab.example.com/me/proj", "https://gitlab.example.com/me/proj/-/issues"},
		{"https://codeberg.org/me/proj", "https://codeberg.org/me/proj/issues"},
		{"https://git.sr.ht/~user/repo", "https://todo.sr.ht/~user/repo"},
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
