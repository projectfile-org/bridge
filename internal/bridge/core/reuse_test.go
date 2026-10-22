// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/v4/internal/pfmodel"
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

// nameAda and familyLovelace are the person the contributor-tier tests declare.
const (
	nameAda        = "Ada"
	familyLovelace = "Lovelace"
	adaEmail       = "ada@example.test"
)

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

// TestREUSEHeaderSlashSeparatesTagGroups pins the `//` dialect (JSONC) the
// same way TestREUSEHeaderHashSeparatesTagGroups pins `#`.
func TestREUSEHeaderSlashSeparatesTagGroups(t *testing.T) {
	t.Parallel()

	got := REUSEHeader(headerDoc(), StyleSlash)

	assert.Equal(t,
		"// "+copyrightLine(holder)+"\n//\n// "+spdxLicenseLine("MIT")+"\n\n",
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

// A copyright-role people entry is the first rung: SPDX lines minus the "Copyright " prefix.
func TestReuseCopyrightLinesPreferExplicitHolders(t *testing.T) {
	doc := &projectfile.Document{
		Copyright: &projectfile.Copyright{Year: 2026},
		People: []projectfile.Person{
			{GivenNames: nameAda, FamilyNames: familyLovelace, Email: adaEmail, Roles: []string{pfmodel.RoleCopyright}},
		},
	}
	assert.Equal(t, []string{"2026 Ada Lovelace <ada@example.test>"}, ReuseCopyrightLines(doc))
	assert.Equal(t, []string{"Ada Lovelace"}, ReuseCopyrightHolderNames(doc))
}

// With no copyright-role holder the first author/maintainer fills in; a nameless entry is skipped.
func TestReuseCopyrightLinesFallBackToContributorThenDisplayName(t *testing.T) {
	author := &projectfile.Document{
		Copyright: &projectfile.Copyright{Year: 2026},
		People: []projectfile.Person{
			{GivenNames: nameAda, FamilyNames: familyLovelace, Email: adaEmail, Roles: []string{"author"}},
		},
	}
	assert.Equal(t, []string{"2026 Ada Lovelace <ada@example.test>"}, ReuseCopyrightLines(author))
	assert.Equal(t, []string{"Ada Lovelace"}, ReuseCopyrightHolderNames(author))

	noEmail := &projectfile.Document{
		Copyright: &projectfile.Copyright{Year: 2026},
		People: []projectfile.Person{
			{Roles: []string{"author"}},
			{GivenNames: "Grace", FamilyNames: "Hopper", Roles: []string{"maintainer"}},
		},
	}
	assert.Equal(t, []string{"2026 Grace Hopper"}, ReuseCopyrightLines(noEmail))
	assert.Equal(t, []string{"Grace Hopper"}, ReuseCopyrightHolderNames(noEmail))

	none := &projectfile.Document{Identity: projectfile.Identity{Name: "p"}, Copyright: &projectfile.Copyright{Year: 2026}}
	assert.Equal(t, []string{holder}, ReuseCopyrightLines(none))
	assert.Equal(t, []string{"p"}, ReuseCopyrightHolderNames(none))
}

// The pinned copyright year wins; an absent copyright namespace takes the current year.
func TestReuseCopyrightYearAndLicenseFallbacks(t *testing.T) {
	thisYear := time.Now().Year()
	assert.Equal(t, 2026, resolveCopyrightYear(&projectfile.Document{Copyright: &projectfile.Copyright{Year: 2026}}))
	assert.Equal(t, thisYear, resolveCopyrightYear(&projectfile.Document{}))
	assert.Equal(t, thisYear, resolveCopyrightYear(nil))

	assert.Equal(t, "Apache-2.0", reuseLicenseID(&projectfile.Document{License: &projectfile.License{Spdx: "Apache-2.0"}}))
	assert.Equal(t, defaultSPDX, reuseLicenseID(&projectfile.Document{License: &projectfile.License{Spdx: "  "}}))
	assert.Equal(t, defaultSPDX, reuseLicenseID(&projectfile.Document{}))
	assert.Equal(t, defaultSPDX, reuseLicenseID(nil))
}

// contributorHolder accepts author and maintainer roles only; a nil document has none.
func TestContributorHolderReadsAuthorAndMaintainer(t *testing.T) {
	name, email, ok := contributorHolder(&projectfile.Document{
		People: []projectfile.Person{
			{GivenNames: nameAda, FamilyNames: familyLovelace, Email: adaEmail, Roles: []string{"maintainer"}},
		},
	})
	assert.True(t, ok)
	assert.Equal(t, "Ada Lovelace", name)
	assert.Equal(t, "ada@example.test", email)

	_, _, ok = contributorHolder(&projectfile.Document{People: []projectfile.Person{{Roles: []string{"sponsor"}}}})
	assert.False(t, ok, "a sponsor is not a rights-holder")

	_, _, ok = contributorHolder(nil)
	assert.False(t, ok)
}
