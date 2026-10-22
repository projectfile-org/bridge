// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"projectfile.org/projectfile/bridge/v4/internal/bridge/core"
)

// The user-config path override wins over the built-in default, joined under the caller's dir.
func TestPathOrDefaultUsesUserConfigOverride(t *testing.T) {
	dir := t.TempDir()
	pointUserConfigAt(t, map[string]any{"generate": map[string]any{"defaults": map[string]any{
		"FUNDING.yml": map[string]any{"path": "assets/funding"},
	}}})

	assert.Equal(t, filepath.Join(dir, "assets", "funding"),
		core.PathOrDefault(dir, "FUNDING.yml", "FUNDING.yml"))
}

// No entry for the filename keeps the renderer's own default.
func TestPathOrDefaultFallsBackToDefault(t *testing.T) {
	dir := t.TempDir()
	pointUserConfigAt(t, nil)

	assert.Equal(t, filepath.Join(dir, "FUNDING.yml"),
		core.PathOrDefault(dir, "FUNDING.yml", "FUNDING.yml"))
}
