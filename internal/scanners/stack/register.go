// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package stack

import (
	"projectfile.org/projectfile/bridge/internal/scanners/core"
	"projectfile.org/projectfile/bridge/internal/source"
)

// Scanner adapts the package's existing Scan walker to the core.Scanner
// interface. Detect is unconditionally true: every directory is a candidate
// for stack detection (in contrast with git, which needs .git present).
type Scanner struct{}

func (Scanner) Name() string { return "stacks" }

func (Scanner) Detect(_ string) bool { return true }

// Scan delegates to the package-level Scan and packs the resulting tag set
// into a Partial.Stack slice plus a Hit per fired rule for logging.
func (Scanner) Scan(root string) (*source.Partial, []core.Hit, error) {
	tags, hits, err := Scan(root)
	if err != nil {
		return nil, nil, err
	}
	p := &source.Partial{}
	if len(tags) > 0 {
		p.Stack = tags
	}
	out := make([]core.Hit, 0, len(hits))
	for _, h := range hits {
		out = append(out, core.Hit{
			Source: "stacks",
			Field:  h.Rule,
			Path:   h.Path,
		})
	}
	return p, out, nil
}

func init() {
	core.Register(Scanner{})
}
