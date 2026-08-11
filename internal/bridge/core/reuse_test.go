// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
)

// copyrightTag is the REUSE copyright tag assembled from fragments, for the
// same reason spdxTag is (see marker_test.go): a source literal spelling the
// whole tag would be read as this file's own licence declaration.
const copyrightTag = "SPDX-FileCopyright"

func copyrightLine(holder string) string { return copyrightTag + "Text: " + holder }

// headerDoc is the minimal document the header composers need. With no
// copyright-role holder and no author, the holder falls back to the display
// name — so the whole header is pinned by the fixture, year included.
func headerDoc() *projectfile.Document {
	return &projectfile.Document{
		Identity:  projectfile.Identity{Name: "p"},
		License:   &projectfile.License{Spdx: "MIT"},
		Copyright: &projectfile.Copyright{Year: 2026},
	}
}

// holder is the copyright line headerDoc resolves to.
const holder = "2026 p"

// TestREUSEHeaderHashSeparatesTagGroups pins the bare `#` between the two tag
// groups — `reuse annotate` writes it for every line-comment dialect, so a
// generated file must carry the same header a hand-annotated source file does.
func TestREUSEHeaderHashSeparatesTagGroups(t *testing.T) {
	t.Parallel()

	got := REUSEHeader(headerDoc(), StyleHash)

	assert.Equal(t,
		"# "+copyrightLine(holder)+"\n#\n# "+spdxLicenseLine("MIT")+"\n\n",
		got)
}

// TestREUSEHeaderHTMLIsOneCompactComment pins the Markdown form: one comment,
// adjacent tag lines. Inside a block comment the tags need no separator, and a
// blank line there would split the header visually for no gain.
func TestREUSEHeaderHTMLIsOneCompactComment(t *testing.T) {
	t.Parallel()

	got := REUSEHeader(headerDoc(), StyleHTML)

	assert.Equal(t,
		"<!--\n"+copyrightLine(holder)+"\n"+spdxLicenseLine("MIT")+"\n-->\n\n",
		got)
}

// TestManagedREUSEHeaderFoldsSentinel pins the one-comment Markdown header for
// a Marker-policy artefact: the sentinel is the comment's last line, and the
// result still reads as managed.
func TestManagedREUSEHeaderFoldsSentinel(t *testing.T) {
	t.Parallel()

	got := ManagedREUSEHeader(headerDoc())

	assert.Equal(t,
		"<!--\n"+copyrightLine(holder)+"\n"+spdxLicenseLine("MIT")+"\n"+MarkerInner+"\n-->\n\n",
		got)
	assert.False(t, strings.Contains(got, MarkerHTML),
		"the folded form must not also emit the standalone marker comment")
	assert.True(t, HasMarker([]byte(got)), "the folded header must read as managed")
}
