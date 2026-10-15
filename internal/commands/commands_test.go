// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package commands

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"projectfile.org/projectfile/bridge/internal/bridge"
)

// TestScopesPartitionTheLinkedBridges verifies each file-bridge command sees only its own bridges once every bridge is linked.
func TestScopesPartitionTheLinkedBridges(t *testing.T) {
	entries, err := os.ReadDir("../bridge")
	require.NoError(t, err)
	total, scoped := len(bridge.List()), 0
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "core" {
			continue
		}
		require.Contains(t, Mains, e.Name())
		bridge.Scope(e.Name())
		n := len(bridge.List())
		bridge.Scope("")
		assert.Positive(t, n, "command %s owns no bridge", e.Name())
		scoped += n
	}
	assert.Equal(t, total, scoped)
}
