// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package coc

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/v4/internal/pfmodel"
)

// Fixture literals the coc suite shares; goconst counts every repeat.
const (
	fxName     = "widget"
	fxTitle    = "Widget"
	fxContact  = "conduct@example.org"
	fxMaint    = "maint@example.org"
	fxGitEmail = "gituser@example.org"
	fxReport   = "https://example.org/report"
)

// bareDoc is the smallest document any bridge accepts: a name to display.
func bareDoc() *projectfile.Document {
	return &projectfile.Document{Identity: projectfile.Identity{Name: fxName}}
}

// conductDoc is a document whose contact arrives from the security namespace, the resolved-first tier.
func conductDoc() *projectfile.Document {
	pf := bareDoc()
	pf.Identity.Title = &projectfile.LocalizedString{Bare: fxTitle}
	projectfile.SetExtension(pf, pfmodel.SecurityExtensionNS, map[string]any{"contact": fxContact})
	return pf
}

// setConductNS attaches [org.projectfile.code-of-conduct], the namespace covenant and scope come from.
func setConductNS(pf *projectfile.Document, ext map[string]any) *projectfile.Document {
	projectfile.SetExtension(pf, pfmodel.CodeOfConductExtensionNS, ext)
	return pf
}

// withLangs declares i18n languages on the shared namespace.
func withLangs(pf *projectfile.Document, langs ...string) *projectfile.Document {
	items := make([]any, len(langs))
	for i, l := range langs {
		items[i] = l
	}
	projectfile.SetExtension(pf, pfmodel.I18NExtensionNS, map[string]any{"languages": items})
	return pf
}

// gitHome writes a HOME carrying user.email, the one value gitConfigEmail reads.
func gitHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(home, ".gitconfig"), []byte("[user]\n\temail = "+fxGitEmail+"\n"), 0o644))
	return home
}

// ── contact resolution ──────────────────────────────────────────────────────

func TestContactEmailPrefersTheSecurityNamespace(t *testing.T) {
	email, src := contactEmail(conductDoc())
	assert.Equal(t, fxContact, email)
	assert.Equal(t, "["+pfmodel.SecurityExtensionNS+"].contact", src)
}

// With no security namespace the community role decides, then the maintainer, then anyone.
func TestContactEmailFallsBackThroughThePeopleRoles(t *testing.T) {
	doc := &projectfile.Document{People: []projectfile.Person{
		{Email: fxMaint, Roles: []string{projectfile.RoleMaintainer}},
		{Email: fxContact, Roles: []string{projectfile.RoleCommunity}},
	}}
	email, src := contactEmail(doc)
	assert.Equal(t, fxContact, email)
	assert.Contains(t, src, projectfile.RoleCommunity)

	doc.People = []projectfile.Person{{Email: fxMaint, Roles: []string{projectfile.RoleMaintainer}}, {Email: fxContact}}
	email, _ = contactEmail(doc)
	assert.Equal(t, fxMaint, email, "a maintainer beats an unroled person")
}

func TestContactEmailReturnsEmptyWhenNobodyCarriesOne(t *testing.T) {
	email, _ := contactEmail(&projectfile.Document{})
	assert.Empty(t, email)
}

// ── git config suggestion ───────────────────────────────────────────────────

func TestGitConfigEmailReadsTheConfiguredAddress(t *testing.T) {
	t.Setenv("HOME", gitHome(t))
	assert.Equal(t, fxGitEmail, gitConfigEmail(t.TempDir()), "a repo dir binds the config lookup")
	assert.Equal(t, fxGitEmail, gitConfigEmail(""), "an empty dir leaves the process cwd in charge")
}

func TestGitConfigEmailIsEmptyWithoutGitConfig(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	assert.Empty(t, gitConfigEmail(t.TempDir()))
}

// ── security namespace writer ───────────────────────────────────────────────

func TestSecurityExtensionToMapDropsEmptyFields(t *testing.T) {
	assert.Empty(t, securityExtensionToMap(&pfmodel.SecurityExtension{}))
}

func TestSecurityExtensionToMapCarriesEveryDeclaredField(t *testing.T) {
	m := securityExtensionToMap(&pfmodel.SecurityExtension{
		Contact:           fxContact,
		ReportURL:         fxReport,
		SupportedVersions: []string{"1.0"},
		DisclosureWindow:  "90 days",
		GPGKey:            "-----BEGIN PGP PUBLIC KEY BLOCK-----",
		BugBountyURL:      "https://example.org/bounty",
	})
	assert.Equal(t, map[string]any{
		"contact":            fxContact,
		"report-url":         fxReport,
		"supported-versions": []any{"1.0"},
		"disclosure-window":  "90 days",
		"gpg-key":            "-----BEGIN PGP PUBLIC KEY BLOCK-----",
		"bug-bounty-url":     "https://example.org/bounty",
	}, m)
}

// ── required fields ─────────────────────────────────────────────────────────

func TestRequiredFieldsIsEmptyWhenTheNamespaceHasTheContact(t *testing.T) {
	assert.Nil(t, Bridge{}.RequiredFields(conductDoc()), "a resolved moderator closes the prompt")
}

func TestRequiredFieldsPromptsForTheModeratorMailbox(t *testing.T) {
	t.Setenv("HOME", gitHome(t))
	pointUserConfigAt(t, nil)
	missing := Bridge{}.RequiredFields(&projectfile.Document{})
	require.Len(t, missing, 1)
	assert.Equal(t, "["+pfmodel.SecurityExtensionNS+"].contact", missing[0].Field)
	assert.Contains(t, missing[0].Hint, "git suggests "+fxGitEmail)
	require.NotNil(t, missing[0].Setter)
}

// Without a git suggestion the hint carries a concrete example instead.
func TestRequiredFieldsHintWithoutGit(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	pointUserConfigAt(t, nil)
	missing := Bridge{}.RequiredFields(&projectfile.Document{})
	require.Len(t, missing, 1)
	assert.Equal(t, "moderator email address (e.g. conduct@example.org)", missing[0].Hint)
}

func TestRequiredFieldsSetterWritesTheContact(t *testing.T) {
	pointUserConfigAt(t, nil)
	pf := &projectfile.Document{}
	require.NoError(t, Bridge{}.RequiredFields(pf)[0].Setter(fxContact))
	assert.Nil(t, Bridge{}.RequiredFields(pf), "the filled contact closes the prompt")
	email, _ := contactEmail(pf)
	assert.Equal(t, fxContact, email)
}

// A pre-existing security namespace keeps its other fields when the setter fills the contact.
func TestRequiredFieldsSetterPreservesTheOtherFields(t *testing.T) {
	pointUserConfigAt(t, nil)
	pf := conductDoc()
	projectfile.SetExtension(pf, pfmodel.SecurityExtensionNS, map[string]any{"report-url": fxReport})
	require.NoError(t, Bridge{}.RequiredFields(pf)[0].Setter(fxContact))
	ext, err := pfmodel.GetSecurityExtension(pf)
	require.NoError(t, err)
	assert.Equal(t, fxContact, ext.Contact)
	assert.Equal(t, fxReport, ext.ReportURL)
}

func TestRequiredFieldsSetterRejectsABlankValue(t *testing.T) {
	pointUserConfigAt(t, nil)
	setter := Bridge{}.RequiredFields(&projectfile.Document{})[0].Setter
	err := setter("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "contact email is required")
}
