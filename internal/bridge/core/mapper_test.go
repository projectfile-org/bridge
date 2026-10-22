// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"projectfile.org/projectfile/bridge/v4/internal/bridge/core"
)

// Without force nothing clears: a default sync converges without destroying.
func TestClearExtWithoutForceIsInert(t *testing.T) {
	flag := true
	list := []string{"a"}
	m := map[string]string{"k": "v"}

	assert.Empty(t, core.ClearExt(false, &flag))
	assert.Empty(t, core.ClearExt(false, &list))
	assert.Empty(t, core.ClearExt(false, &m))
	assert.True(t, flag)
	assert.Equal(t, []string{"a"}, list)
	assert.Equal(t, map[string]string{"k": "v"}, m)
}

// With force a populated field is zeroed and reported removed.
func TestClearExtWithForceZeroesPopulatedFields(t *testing.T) {
	flag := true
	text := "value"
	list := []string{"a"}
	m := map[string]string{"k": "v"}

	assert.Equal(t, core.Removed, core.ClearExt(true, &flag))
	assert.Equal(t, core.Removed, core.ClearExt(true, &text))
	assert.Equal(t, core.Removed, core.ClearExt(true, &list))
	assert.Equal(t, core.Removed, core.ClearExt(true, &m))
	assert.False(t, flag)
	assert.Empty(t, text)
	assert.Empty(t, list)
	assert.Empty(t, m)
}

// Zero values and empty-but-non-nil slices/maps count as absent: clearing them manufactures a change.
func TestClearExtWithForceKeepsAbsentFields(t *testing.T) {
	flag := false
	text := ""
	list := []string{}
	m := map[string]string{}

	assert.Empty(t, core.ClearExt(true, &flag))
	assert.Empty(t, core.ClearExt(true, &text))
	assert.Empty(t, core.ClearExt(true, &list))
	assert.Empty(t, core.ClearExt(true, &m))
}
