// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core

import "testing"

func TestResolveToken_PerHostBeatsCanonical(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "canonical-value")
	t.Setenv("PF_FORGE_TOKEN_GITHUB_COM", "per-host-value")
	tok, src := ResolveToken("github", CanonicalGitHub)
	if tok != "per-host-value" {
		t.Fatalf("got %q, want per-host-value", tok)
	}
	if src != "PF_FORGE_TOKEN_GITHUB_COM" {
		t.Fatalf("got source %q, want PF_FORGE_TOKEN_GITHUB_COM", src)
	}
}

func TestResolveToken_CanonicalFallback(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "canonical-value")
	t.Setenv("PF_FORGE_TOKEN_GITHUB_COM", "")
	tok, src := ResolveToken("github", CanonicalGitHub)
	if tok != "canonical-value" {
		t.Fatalf("got %q, want canonical-value", tok)
	}
	if src != "GITHUB_TOKEN" {
		t.Fatalf("got source %q, want GITHUB_TOKEN", src)
	}
}

func TestResolveToken_ForgejoGiteaFallback(t *testing.T) {
	t.Setenv("FORGEJO_TOKEN", "")
	t.Setenv("GITEA_TOKEN", "gitea-value")
	tok, src := ResolveToken("forgejo", "codeberg.org")
	if tok != "gitea-value" {
		t.Fatalf("got %q, want gitea-value", tok)
	}
	if src != "GITEA_TOKEN" {
		t.Fatalf("got source %q, want GITEA_TOKEN", src)
	}
}

func TestResolveToken_MissingReturnsEmpty(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "")
	t.Setenv("PF_FORGE_TOKEN_GITLAB_EXAMPLE_COM", "")
	tok, src := ResolveToken("gitlab", "gitlab.example.com")
	if tok != "" {
		t.Fatalf("got %q, want empty", tok)
	}
	if src != "" {
		t.Fatalf("got source %q, want empty", src)
	}
}

func TestHostOverrideEnvVar(t *testing.T) {
	cases := []struct {
		host, want string
	}{
		{CanonicalGitHub, "PF_FORGE_TOKEN_GITHUB_COM"},
		{"gitlab.example.com", "PF_FORGE_TOKEN_GITLAB_EXAMPLE_COM"},
		{"code.foo-bar.dev", "PF_FORGE_TOKEN_CODE_FOO_BAR_DEV"},
	}
	for _, c := range cases {
		got := HostOverrideEnvVar(c.host)
		if got != c.want {
			t.Errorf("HostOverrideEnvVar(%q) = %q, want %q", c.host, got, c.want)
		}
	}
}
