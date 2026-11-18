// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/v4/internal/pfmodel"
)

// The badge the document-routing tests move between documents.
const (
	badgeCommits    = "commits"
	badgeCommitsImg = "https://img.test/" + badgeCommits
	badgeLicense    = "license"
)

// TestBadgesRenderOnTheirDocument: a shield with no documents stays on the readme, one naming contributing leaves it.
func TestBadgesRenderOnTheirDocument(t *testing.T) {
	shields := []pfmodel.Shield{
		{Name: "license", Img: "https://img.test/license"},
		{Name: badgeCommits, Img: badgeCommitsImg, Documents: []string{"contributing"}},
	}
	assert.Equal(t, "![license](https://img.test/license)", BadgeMarkdown(BadgeRows(nil, shields, DocumentReadme, "")))
	assert.Equal(t, "![commits](https://img.test/commits)", BadgeMarkdown(BadgeRows(nil, shields, "contributing", "")))
}

// TestBadgeEncodesSpaces: an SPDX expression in a badge URL stays one markdown link destination.
func TestBadgeEncodesSpaces(t *testing.T) {
	shields := []pfmodel.Shield{{Name: "license", Img: "https://img.test/?message=MIT OR CC-BY-4.0", Href: "https://l.test/a b"}}
	assert.Equal(t, "[![license](https://img.test/?message=MIT%20OR%20CC-BY-4.0)](https://l.test/a%20b)", BadgeMarkdown(BadgeRows(nil, shields, DocumentReadme, "")))
}

// TestRedeclaredBadgeMovesDocument: redeclaring an inherited name with documents moves the badge, it does not copy it.
func TestRedeclaredBadgeMovesDocument(t *testing.T) {
	shields := []pfmodel.Shield{
		{Name: badgeCommits, Img: badgeCommitsImg},
		{Name: badgeCommits, Img: badgeCommitsImg, Href: "https://cc.test", Documents: []string{"contributing"}},
	}
	assert.Empty(t, Badges(nil, shields, DocumentReadme, ""))
	assert.Equal(t, "[![commits](https://img.test/commits)](https://cc.test)", BadgeMarkdown(BadgeRows(nil, shields, "contributing", "")))
}

// A shield with no image resolves to nothing and is dropped, never rendered as an empty cell.
func TestBadgesDropsShieldsWithoutImage(t *testing.T) {
	shields := []pfmodel.Shield{
		{Name: badgeLicense, Img: "", Href: "https://l.test"},
		{Name: badgeCommits, Img: badgeCommitsImg},
	}
	got := BadgeMarkdown(BadgeRows(nil, shields, DocumentReadme, ""))
	assert.Equal(t, "!["+badgeCommits+"]("+badgeCommitsImg+")", got)
}

// An unresolved ${…} placeholder never reaches the badge: the shield is dropped instead.
func TestBadgesDropsUnresolvedPlaceholders(t *testing.T) {
	shields := []pfmodel.Shield{
		{Name: "bad", Img: "https://img.test/${org.projectfile.nope.missing}"},
		{Name: "good", Img: "https://img.test/ok"},
	}
	assert.Len(t, Badges(nil, shields, DocumentReadme, ""), 1)
	assert.Equal(t, "good", Badges(nil, shields, DocumentReadme, "")[0].Alt)
}

// A shield naming a declared CI matrix axis is dropped: per-cell fan-out is the CI plane’s job.
func TestBadgesDropsShieldsNamingAMatrixAxis(t *testing.T) {
	doc := &projectfile.Document{}
	projectfile.SetExtension(doc, ciNS, map[string]any{"matrix": map[string]any{"axes": map[string]any{
		"arch": []any{"amd64", "arm64"},
	}}})
	shields := []pfmodel.Shield{
		{Name: "axis", Img: "https://img.test/{arch}.svg"},
		{Name: "plain", Img: "https://img.test/plain.svg"},
	}
	assert.Len(t, Badges(doc, shields, DocumentReadme, ""), 1)
}

// Alt falls back to the shield name so every badge carries description text.
func TestBadgeAltFallsBackToName(t *testing.T) {
	shields := []pfmodel.Shield{{Name: badgeCommits, Img: badgeCommitsImg}}
	assert.Equal(t, badgeCommits, Badges(nil, shields, DocumentReadme, "")[0].Alt)
}

// hrefForLang honours the per-language override for that language only.
func TestHrefForLangResolvesPerLanguage(t *testing.T) {
	s := pfmodel.Shield{Href: "https://default.test", HrefByLang: map[string]string{"es": "https://es.test"}}
	assert.Equal(t, "https://es.test", hrefForLang(s, "es"))
	assert.Equal(t, "https://default.test", hrefForLang(s, "uk"))
	assert.Equal(t, "https://default.test", hrefForLang(s, ""))
}

// The unnamed row gets a label for the decision trace; a named row keeps it.
func TestBadgeRowLabelNamesTheUnnamedRow(t *testing.T) {
	assert.Equal(t, "(unnamed)", BadgeRowLabel(""))
	assert.Equal(t, "top", BadgeRowLabel("top"))
}

// TestBadgeHrefRebasedPerLanguage: a repo-relative href resolves from docs/<lang>/, a localized sibling stays in its language, absolute ones are untouched.
func TestBadgeHrefRebasedPerLanguage(t *testing.T) {
	shields := []pfmodel.Shield{
		{Name: badgeLicense, Img: "https://img.test/l", Href: "LICENSE"},
		{Name: "prs", Img: "https://img.test/p", Href: FileContributing},
		{Name: "cite", Img: "https://img.test/c", Href: "CITATION.cff", Documents: []string{DocumentContributing}},
		{Name: "web", Img: "https://img.test/w", Href: "https://w.test/"},
		{Name: "top", Img: "https://img.test/t", Href: "#top"},
	}
	assert.Equal(t, "[![license](https://img.test/l)](LICENSE) [![prs](https://img.test/p)](CONTRIBUTING.md) [![web](https://img.test/w)](https://w.test/) [![top](https://img.test/t)](#top)",
		BadgeMarkdown(BadgeRows(nil, shields, DocumentReadme, "")), "the root readme keeps every href")
	assert.Equal(t, "[![license](https://img.test/l)](LICENSE) [![prs](https://img.test/p)](CONTRIBUTING.md) [![web](https://img.test/w)](https://w.test/) [![top](https://img.test/t)](#top)",
		BadgeMarkdown(BadgeRows(nil, shields, DocumentReadme, "en")), "the default language is the root readme")
	assert.Equal(t, "[![license](https://img.test/l)](../../LICENSE) [![prs](https://img.test/p)](CONTRIBUTING.md) [![web](https://img.test/w)](https://w.test/) [![top](https://img.test/t)](#top)",
		BadgeMarkdown(BadgeRows(nil, shields, DocumentReadme, "es")))
	assert.Equal(t, "[![cite](https://img.test/c)](../../CITATION.cff)", BadgeMarkdown(BadgeRows(nil, shields, DocumentContributing, "es")))
	other := []pfmodel.Shield{{Name: "cite", Img: "https://img.test/c", Href: "CITATION.cff", Documents: []string{"security"}}}
	assert.Equal(t, "[![cite](https://img.test/c)](CITATION.cff)", BadgeMarkdown(BadgeRows(nil, other, "security", "es")), "a document without a known file is never rebased")
}
