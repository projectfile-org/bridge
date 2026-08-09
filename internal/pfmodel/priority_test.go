// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package pfmodel_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/internal/pfmodel"
)

// keyPriority is the §139 advisory priority key, declared once for the
// pfmodel test package so goconst sees one canonical home (mirroring the
// production const in links.go). linkTypeHomepage is the link type reused by
// the fixtures below.
const (
	keyPriority      = "priority"
	linkTypeHomepage = "homepage"
	urlExample       = "https://example.test"
)

// LinkPriority reads the advisory `priority` key a link carries via the spec's
// §139 additional-key channel (Link.Extra) — the same channel the `related`
// tag rides on. An absent key returns PriorityDefault so a stable sort keeps
// the link in declaration order.
func TestLinkPriorityDefaultWhenAbsent(t *testing.T) {
	l := projectfile.Link{Type: linkTypeHomepage, URL: urlExample}
	assert.Equal(t, pfmodel.PriorityDefault, pfmodel.LinkPriority(l),
		"absent priority returns the default")
}

// A YAML/JSON/TOML decoder hands numeric values to Extra as int, int64, or
// float64 depending on the format. LinkPriority coerces all three so the value
// set in any projectfile encoding reaches the sort as the intended integer.
func TestLinkPriorityCoercesNumericShapes(t *testing.T) {
	cases := []struct {
		name string
		raw  any
		want int
	}{
		{"int", 300, 300},
		{"int64", int64(200), 200},
		{"float64", float64(100), 100},
		{"negative", -5, -5},
		{"zero is honoured, not treated as absent", 0, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			l := projectfile.Link{
				Type:  linkTypeHomepage,
				URL:   urlExample,
				Extra: map[string]any{keyPriority: tc.raw},
			}
			assert.Equal(t, tc.want, pfmodel.LinkPriority(l))
		})
	}
}

// A mistyped priority (string, list, map) is not a number, so it falls back to
// PriorityDefault rather than panicking or rendering a broken order.
func TestLinkPriorityFallsBackForNonNumeric(t *testing.T) {
	for _, raw := range []any{"100", []any{1}, map[string]any{"x": 1}, true} {
		l := projectfile.Link{
			Type:  linkTypeHomepage,
			URL:   urlExample,
			Extra: map[string]any{keyPriority: raw},
		}
		assert.Equal(t, pfmodel.PriorityDefault, pfmodel.LinkPriority(l),
			"non-numeric priority %v returns the default", raw)
	}
}

// ByPriorityDesc is the single comparator every priority-aware sort uses, so
// the direction lives in one place. Higher renders FIRST: it returns a
// negative result when a has the higher priority (a sorts before b).
func TestByPriorityDescHigherFirst(t *testing.T) {
	assert.Negative(t, pfmodel.ByPriorityDesc(300, 100), "300 sorts before 100")
	assert.Positive(t, pfmodel.ByPriorityDesc(50, 200), "50 sorts after 200")
	assert.Zero(t, pfmodel.ByPriorityDesc(100, 100), "equal priorities tie")
}
