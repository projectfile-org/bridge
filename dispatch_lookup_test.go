// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

//go:build lookup

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func TestLookupFindsSiblingsOnPATH(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pf-bridge-npm"), nil, 0o755))
	t.Setenv("PATH", dir)

	assert.True(t, installed(npmName))
	assert.False(t, installed("absent"))
	assert.Contains(t, discover(), npmName)
}
