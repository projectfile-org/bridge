// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package npm

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadRejectsMalformedJSON(t *testing.T) {
	dir := t.TempDir()
	writePkgJSON(t, dir, `{"name": oops}`)
	doc, err := Read(dir)
	require.Error(t, err, "malformed JSON must be reported, not silently half-read")
	assert.Nil(t, doc)
}

// TestReadRejectsANonObjectRoot pins that a null root reads as an empty Document yet is no object for the raw canvas.
func TestReadRejectsANonObjectRoot(t *testing.T) {
	dir := t.TempDir()
	writePkgJSON(t, dir, `null`)
	doc, err := Read(dir)
	require.Error(t, err, "a null root unmarshals into Document but never into the raw canvas")
	assert.Nil(t, doc)
}

// TestParsePersonEveryShape covers the three spellings a person can take inside
// package.json plus the nil pointer and the unknown-type fallthrough.
func TestParsePersonEveryShape(t *testing.T) {
	value := Person{Name: "Dana", Email: "dana@example.test", URL: "https://dana.test"}
	assert.Equal(t, value, ParsePerson(value), "a Person value passes through untouched")

	pointer := &Person{Name: "Dana", Email: "dana@example.test", URL: "https://dana.test"}
	assert.Equal(t, value, ParsePerson(pointer), "a *Person dereferences to its value")

	assert.Equal(t, Person{}, ParsePerson((*Person)(nil)), "a nil *Person is an empty person")
	assert.Equal(t, Person{}, ParsePerson(42), "an unknown type is an empty person")
	assert.Equal(t, "Eve", ParsePerson(map[string]any{npmKeyName: "Eve"}).Name,
		"a mapping without email/url still carries its name")
	assert.False(t, ParsePerson(map[string]any{npmKeyName: "Eve"}).IsEmpty())
}

// TestParsePersonStringEdge pins the two unterminated spellings parsePersonString
// tolerates: an email with no closing bracket and a URL with no closing paren
// both stay inside the name, because neither delimiter pair is complete.
func TestParsePersonStringEdge(t *testing.T) {
	openEmail := ParsePerson("Frank <frank@example.test")
	assert.Equal(t, "Frank <frank@example.test", openEmail.Name,
		"an unterminated email is not split out of the name")
	assert.Empty(t, openEmail.Email)

	openURL := ParsePerson("Fran (https://fran.test")
	assert.Equal(t, "Fran (https://fran.test", openURL.Name,
		"an unterminated URL is not split out of the name")
	assert.Empty(t, openURL.URL)

	closed := ParsePerson("Grace (https://grace.test)")
	assert.Equal(t, "Grace", closed.Name)
	assert.Equal(t, "https://grace.test", closed.URL)

	// The npm string form is "Name <email> (url)", but the email cut at "<"
	// removes the tail, so a URL typed after the email is not recovered.
	both := ParsePerson("Heidi <heidi@example.test> (https://heidi.test)")
	assert.Equal(t, "Heidi", both.Name)
	assert.Equal(t, "heidi@example.test", both.Email)
	assert.Empty(t, both.URL, "the URL is cut away with the email's tail")

	reversed := ParsePerson("Heidi (https://heidi.test) <heidi@example.test>")
	assert.Equal(t, "Heidi", reversed.Name)
	assert.Equal(t, "https://heidi.test", reversed.URL,
		"a URL written before the email survives both cuts")
	assert.Equal(t, "heidi@example.test", reversed.Email,
		"the later email cut still leaves the earlier URL in place")
}

func TestParsePersonListEveryItem(t *testing.T) {
	got := ParsePersonList([]any{
		"Judy <judy@example.test>",
		map[string]any{npmKeyName: "Ivan", fxKeyEmail: "ivan@example.test"},
		"nonsense",
		42,
	})
	require.Len(t, got, 4)
	assert.Equal(t, "Judy", got[0].Name)
	assert.Equal(t, "Ivan", got[1].Name)
	assert.Equal(t, "nonsense", got[2].Name, "a scalar with nothing in it parses to an empty person")
	assert.Equal(t, Person{}, got[3], "an unknown type is an empty person")
	assert.Len(t, ParsePersonList(nil), 0)
}

// TestParseRepositoryEveryShape walks all four shortcut prefixes, the three URL
// spellings, the plain fallback and the mapping with a directory.
func TestParseRepositoryEveryShape(t *testing.T) {
	cases := []struct {
		in       any
		url      string
		repoType string
	}{
		{in: "gitlab:acme/pkg", url: "https://gitlab.com/acme/pkg", repoType: repoTypeGit},
		{in: "gist:deadbeef", url: "https://gist.github.com/deadbeef", repoType: repoTypeGit},
		{in: "git+https://example.test/pkg.git", url: fxPkgURL, repoType: repoTypeGit},
		{in: "git://example.test/pkg.git", url: fxPkgURL, repoType: repoTypeGit},
		{in: "example.test/pkg.git", url: "example.test/pkg.git", repoType: repoTypeGit},
		{in: 42, url: "", repoType: ""},
	}
	for _, tc := range cases {
		got := ParseRepository(tc.in)
		assert.Equal(t, tc.url, got.URL, "%v", tc.in)
		assert.Equal(t, tc.repoType, got.Type, "%v", tc.in)
	}

	typed := Repository{URL: fxPkgURL, Type: repoTypeGit}
	assert.Equal(t, typed, ParseRepository(typed), "a typed Repository passes through")

	mapped := ParseRepository(map[string]any{
		fxKeyURL: fxPkgURL, fxKeyType: repoTypeGit, "directory": "packages/npm",
	})
	assert.Equal(t, "packages/npm", mapped.Directory, "a mapping's directory is read")
	assert.Equal(t, repoTypeGit, mapped.Type)
	assert.Equal(t, Repository{}, ParseRepository(map[string]any{fxKeyURL: 42}),
		"a non-string url is dropped, not coerced")
}

// TestParseBugsEveryShape covers the typed, string, mapping and fallthrough
// spellings, with and without the email alternative.
func TestParseBugsEveryShape(t *testing.T) {
	assert.Equal(t, Bugs{URL: fxIssuesURL}, ParseBugs(fxIssuesURL))

	typed := Bugs{URL: fxIssuesURL, Email: "bugs@example.test"}
	assert.Equal(t, typed, ParseBugs(typed), "a typed Bugs passes through")

	mapped := ParseBugs(map[string]any{fxKeyURL: fxIssuesURL, fxKeyEmail: "bugs@example.test"})
	assert.Equal(t, typed, mapped)
	assert.Equal(t, Bugs{}, ParseBugs(map[string]any{fxKeyURL: 42}), "a non-string url is dropped")
	assert.Equal(t, Bugs{}, ParseBugs(42))
}

// TestParseFundingEveryShape covers the scalar, mapping, list and fallthrough
// spellings, including an entry that carries nothing.
func TestParseFundingEveryShape(t *testing.T) {
	assert.Equal(t, []FundingEntry{{URL: fxFundURL}}, ParseFunding(fxFundURL))

	single := ParseFunding(map[string]any{fxKeyURL: fxFundURL, fxKeyType: fxValueCustom})
	require.Len(t, single, 1)
	assert.Equal(t, fxFundURL, single[0].URL)
	assert.Equal(t, fxValueCustom, single[0].Type)

	list := ParseFunding([]any{
		"https://example.test/a",
		map[string]any{fxKeyURL: "https://example.test/b"},
		map[string]any{},
	})
	require.Len(t, list, 3, "every entry of a list expands to one FundingEntry")
	assert.Equal(t, "https://example.test/a", list[0].URL)
	assert.Equal(t, "https://example.test/b", list[1].URL)

	urlless := ParseFunding(map[string]any{fxKeyType: fxValueCustom})
	require.Len(t, urlless, 1, "a typeless mapping still yields one entry")
	assert.Empty(t, urlless[0].URL)
	assert.Nil(t, ParseFunding(42), "an unknown type yields no entry")
}

// TestWriteMergeDelegates pins that the legacy entry point still reaches Write.
func TestWriteMergeDelegates(t *testing.T) {
	dir := t.TempDir()
	writePkgJSON(t, dir, `{"name":"legacy","version":"1.0.0"}`)
	doc, err := Read(dir)
	require.NoError(t, err)
	doc.Description = "through the legacy entry point"
	require.NoError(t, WriteMerge(dir, doc))

	got, err := Read(dir)
	require.NoError(t, err)
	assert.Equal(t, "through the legacy entry point", got.Description)
}

func TestFullPathAbsAndClean(t *testing.T) {
	dir := t.TempDir()
	assert.Equal(t, filepath.Join(dir, "package.json"), FullPath(dir))
	assert.Equal(t, filepath.Join(dir, "sub", "package.json"), FullPath(filepath.Join(dir, "sub", ".", "..", "sub")),
		"a path with redundant components is cleaned to the same file")
}
