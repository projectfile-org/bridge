// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFirstHeading pins the label source: the first ATX heading of any level, empty on no heading and on a read error.
func TestFirstHeading(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.md"), []byte("preamble\n## Second Section\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "b.md"), []byte("no heading here\n"), 0o600))
	assert.Equal(t, "Second Section", FirstHeading(dir, "a.md"))
	assert.Equal(t, "", FirstHeading(dir, "b.md"), "no heading → empty")
	assert.Equal(t, "", FirstHeading(dir, "missing.md"), "unreadable → empty")
}

// TestParseATXHeading pins level detection, the required space after the marker, and the 1–6 CommonMark bound.
func TestParseATXHeading(t *testing.T) {
	cases := []struct {
		line  string
		level int
		text  string
		ok    bool
	}{
		{"# Title", 1, "Title", true},
		{"#\tTab", 1, "Tab", true},
		{"###  Trimmed  ", 3, "Trimmed", true},
		{"###### Deep", 6, "Deep", true},
		{"####### Too many", 0, "", false},
		{"NoMarker", 0, "", false},
		{"#NoSpace", 0, "", false},
		{"#", 0, "", false},
		{"", 0, "", false},
	}
	for _, c := range cases {
		level, text, ok := ParseATXHeading(c.line)
		assert.Equalf(t, c.ok, ok, "ok mismatch for %q", c.line)
		assert.Equalf(t, c.level, level, "level mismatch for %q", c.line)
		assert.Equalf(t, c.text, text, "text mismatch for %q", c.line)
	}
}
