// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestEmbeddedNeedsNoPATH verifies the standalone binary lists and finds its subcommands with an empty PATH.
func TestEmbeddedNeedsNoPATH(t *testing.T) {
	t.Setenv("PATH", "")

	assert.True(t, installed(bridgeName))
	assert.True(t, installed(forgeName))
	assert.False(t, installed("absent"))
	assert.Contains(t, discover(), npmName)
	assert.IsIncreasing(t, discover())
}
