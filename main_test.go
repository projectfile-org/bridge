// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// bridgeName is a stand-in bridge for the grammar cases below.
const bridgeName = "readme"

// Stand-in names shared across the discovery and grammar cases.
const (
	npmName   = "npm"
	forgeName = "forge"
)

func TestIsPlatformArtifact(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"npm-linux-amd64", true},
		{"npm-darwin-arm64", true},
		{"linux-amd64", true}, // the suffixed dispatcher copy
		{npmName, false},
		{"readme", false},
		{forgeName, false},
		{"codeowners", false},
		{"linux", false}, // goos alone is not a pair
		{"amd64", false}, // goarch alone is not a pair
		{"npm-linux", false},
		{"npm-amd64", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, isPlatformArtifact(tc.name))
		})
	}
}

func TestDiscoverSkipsPlatformArtifacts(t *testing.T) {
	dir := t.TempDir()
	for _, n := range []string{
		"pf-bridge", // the dispatcher itself
		"pf-bridge-npm",
		"pf-bridge-npm-linux-amd64",
		"pf-bridge-linux-amd64",
		"pf-bridge-forge",
		"pf-bridge-fragments-darwin-arm64",
		"unrelated",
	} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, n), nil, 0o755))
	}
	t.Setenv("PATH", dir)

	assert.Equal(t, []string{forgeName, npmName}, discover())
}

func TestRenderList(t *testing.T) {
	entries := []bridgeEntry{
		{name: npmName, desc: "package.json — two-way sync"},
		{name: "readme", desc: "README.md — one-way render"},
	}

	var plain strings.Builder
	renderList(&plain, entries, false)
	assert.Equal(t,
		"  npm     package.json — two-way sync\n"+
			"  readme  README.md — one-way render\n",
		plain.String())

	var bold strings.Builder
	renderList(&bold, entries, true)
	assert.Contains(t, bold.String(), "\x1b[1mnpm\x1b[22m")
	assert.NotContains(t, bold.String(), "\x1b[1mnpm \x1b[22m", "padding must stay outside the escape")
}

func TestWithDescriptions(t *testing.T) {
	dir := t.TempDir()
	// A script child answers the probe like a real pf-bridge-* would; a silent
	// one stands in for an older install that does not know the flag.
	script := "#!/bin/sh\necho 'package.json — two-way sync'\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pf-bridge-npm"), []byte(script), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pf-bridge-old"), nil, 0o755))
	t.Setenv("PATH", dir)

	entries := withDescriptions([]string{npmName, "old"})
	assert.Equal(t, "package.json — two-way sync", entries[0].desc)
	assert.Empty(t, entries[1].desc)
}

// The fan-out grammar, one case per shape the dispatcher has to tell apart.
// The first case is the regression that matters: a flag typed after `all` used
// to be dropped, which turned the read-only `pf-bridge all --check` into a
// write sweep over every derived file in the repository.
func TestSplitNamesFlags(t *testing.T) {
	cases := []struct {
		name      string
		args      []string
		wantNames []string
		wantFlags []string
	}{
		{
			name:      "flags after the fan-out verb reach the children",
			args:      []string{"all", "--check"},
			wantNames: nil,
			wantFlags: []string{"--check"},
		},
		{
			name:      "the verb itself is never a bridge name",
			args:      []string{"check", "--all"},
			wantNames: nil,
			wantFlags: nil,
		},
		{
			name:      "named bridges narrow the sweep",
			args:      []string{bridgeName, npmName},
			wantNames: []string{bridgeName, npmName},
			wantFlags: nil,
		},
		{
			name:      "names and flags mix in any order",
			args:      []string{"--no-diff", bridgeName, "--fail-on-drift"},
			wantNames: []string{bridgeName},
			wantFlags: []string{"--no-diff", "--fail-on-drift"},
		},
		{
			name:      "direction keywords address the dispatcher, not a child",
			args:      []string{"to", "all", "--dry-run"},
			wantNames: nil,
			wantFlags: []string{"--dry-run"},
		},
		{
			name:      "short flags forward too",
			args:      []string{"-n"},
			wantNames: nil,
			wantFlags: []string{"-n"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			names, flags := splitNamesFlags(tc.args)

			assert.Equal(t, tc.wantNames, names)
			assert.Equal(t, tc.wantFlags, flags)
		})
	}
}
