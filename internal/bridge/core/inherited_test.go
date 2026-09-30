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

func TestLocalizedFilename(t *testing.T) {
	const usage = "docs/USAGE.md"
	for _, c := range []struct{ base, lang, want string }{
		{FileContributing, "", FileContributing},
		{FileContributing, "es", "docs/es/CONTRIBUTING.md"},
		{usage, "", usage},
		{usage, "uk", "docs/uk/USAGE.md"},
	} {
		if got := LocalizedFilename(c.base, c.lang); got != c.want {
			t.Errorf("LocalizedFilename(%q, %q) = %q, want %q", c.base, c.lang, got, c.want)
		}
	}
}
