// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// bridgeName is a stand-in bridge for the grammar cases below.
const bridgeName = "readme"

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
			args:      []string{bridgeName, "npm"},
			wantNames: []string{bridgeName, "npm"},
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
