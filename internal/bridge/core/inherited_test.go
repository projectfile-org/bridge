// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core

import "testing"

const ubuntu = "B19 / Ubuntu"

func TestInheritedParent(t *testing.T) {
	for heading, want := range map[string]string{
		"## Inherited from B19 / Ubuntu": ubuntu,
		"Heredado de B19 / Ubuntu":       "B19 / Ubuntu",
		"Успадковано від B19 / Ubuntu":   "B19 / Ubuntu",
		"Something else":                 "Something else",
	} {
		if got := InheritedParent(heading); got != want {
			t.Errorf("InheritedParent(%q) = %q, want %q", heading, got, want)
		}
	}
}
