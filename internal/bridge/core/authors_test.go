// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"kiota.ch/projectfile/core/v2/pkg/projectfile"
)

const (
	roleAuthor     = "author"
	roleMaintainer = "maintainer"
)

func TestSplitAuthor(t *testing.T) {
	name, email := SplitAuthor(" Ada Lovelace <ada@example.com> ")
	assert.Equal(t, "Ada Lovelace", name)
	assert.Equal(t, "ada@example.com", email)
	name, email = SplitAuthor("Solo <unclosed")
	assert.Equal(t, "Solo <unclosed", name)
	assert.Empty(t, email)
}

func TestAuthorPersonAndFormatRoundTrip(t *testing.T) {
	p := AuthorPerson("Ada Lovelace", "ada@example.com")
	assert.Equal(t, "Ada", p.GivenNames)
	assert.Equal(t, "Lovelace", p.FamilyNames)
	assert.Equal(t, "Ada Lovelace <ada@example.com>", FormatAuthor(p))
	assert.Equal(t, "Mono", FormatAuthor(AuthorPerson("Mono", "")))
	assert.Equal(t, "x@example.com", FormatAuthor(AuthorPerson("", "x@example.com")))
	full := projectfile.Person{GivenNames: "Ludwig", NameParticle: "van", FamilyNames: "Beethoven", NameSuffix: "II"}
	assert.Equal(t, "Ludwig van Beethoven II", PersonDisplayName(full))
	assert.Empty(t, PersonDisplayName(projectfile.Person{GivenNames: "Only"}))
}

func TestAuthorStringsSelectsAuthorRole(t *testing.T) {
	people := []projectfile.Person{
		{DisplayName: "A", Roles: []string{roleMaintainer, roleAuthor}},
		{DisplayName: "M", Roles: []string{roleMaintainer}},
		{Roles: []string{roleAuthor}},
	}
	assert.Equal(t, []string{"A"}, AuthorStrings(people))
	assert.Len(t, PeopleWithRole(people, roleMaintainer), 2)
}

func TestMergeAuthorStrings(t *testing.T) {
	pf := &projectfile.Document{}
	assert.Empty(t, MergeAuthorStrings(pf, []string{" ", "<>"}), "entries with neither name nor email are skipped")
	assert.Equal(t, "1 author(s) merged", MergeAuthorStrings(pf, []string{"Ada Lovelace <ada@example.com>"}))
	assert.Empty(t, MergeAuthorStrings(pf, []string{"Ada Lovelace <ada@example.com>"}), "a known author changes nothing")
	assert.Len(t, pf.People, 1)
}

func TestGapFillREUSEHeader(t *testing.T) {
	header := "# SPDX-License-Identifier: MIT\n\n"
	assert.Equal(t, header+"a = 1\n", string(GapFillREUSEHeader([]byte("a = 1\n"), header)))
	tagged := "# note\n\n# SPDX-FileCopyrightText: X\na = 1\n"
	assert.Equal(t, tagged, string(GapFillREUSEHeader([]byte(tagged), header)), "a present header wins")
	late := "a = 1\n# SPDX-License-Identifier: MIT\n"
	assert.Equal(t, header+late, string(GapFillREUSEHeader([]byte(late), header)), "only the leading block counts")
	assert.Equal(t, "a = 1\n", string(GapFillREUSEHeader([]byte("a = 1\n"), "")))
}
