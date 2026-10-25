// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package security

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"projectfile.org/projectfile/bridge/v4/internal/bridge/core"
	"projectfile.org/projectfile/bridge/v4/internal/pfmodel"
)

// ── contact channel ─────────────────────────────────────────────────────────

// A security-role person is the resolved contact, so the policy names the mailbox.
func TestRenderResolvesTheContactFromThePeopleRoles(t *testing.T) {
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: fxName, Title: &projectfile.LocalizedString{Bare: fxTitle}}}
	pf.People = []projectfile.Person{{GivenNames: "Sec", FamilyNames: "Officer", Email: fxContact, Roles: []string{projectfile.RoleSecurity}}}
	pointUserConfigAt(t, nil)
	out, err := Bridge{}.Render(pf, core.Options{})
	require.NoError(t, err)
	assert.Contains(t, string(out.Files[core.FileSecurity]), "**<"+fxContact+">**")
}

// With no contact anywhere in the document the per-user mailbox answers for the project.
func TestRenderFallsBackToTheUserConfigMailbox(t *testing.T) {
	pointUserConfigAt(t, map[string]any{"identity": map[string]any{"email": fxContact}})
	out, err := Bridge{}.Render(&projectfile.Document{Identity: projectfile.Identity{Name: fxName}}, core.Options{})
	require.NoError(t, err)
	assert.Contains(t, string(out.Files[core.FileSecurity]), "**<"+fxContact+">**")
}

// A URL alone is a channel, so a project with no mailbox still renders its private disclosure form.
func TestRenderReportsTheURLOnlyChannel(t *testing.T) {
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: fxName}}
	projectfile.SetExtension(pf, pfmodel.SecurityExtensionNS, map[string]any{keyReport: fxReport})
	pointUserConfigAt(t, nil)
	out, err := Bridge{}.Render(pf, core.Options{})
	require.NoError(t, err)
	body := string(out.Files[core.FileSecurity])
	assert.Contains(t, body, "Report them through our private disclosure channel:")
	assert.Contains(t, body, " <"+fxReport+">")
	assert.NotContains(t, body, "Report them by emailing", "the email branch is off")
}

func TestRenderFailsWithoutAnyChannel(t *testing.T) {
	pointUserConfigAt(t, nil)
	_, err := Bridge{}.Render(&projectfile.Document{Identity: projectfile.Identity{Name: fxName}}, core.Options{})
	require.Error(t, err, "a channel nobody declared is not invented")
	assert.Contains(t, err.Error(), "no contact channel")
}

// ── declared sections ───────────────────────────────────────────────────────

// Every declared field renders its section: versions, window, bounty, fingerprint, acknowledged findings.
func TestRenderRendersEveryDeclaredSection(t *testing.T) {
	pf := fullDoc()
	pointUserConfigAt(t, nil)
	out, err := Bridge{}.Render(pf, core.Options{})
	require.NoError(t, err)
	body := string(out.Files[core.FileSecurity])
	assert.Contains(t, body, "## Supported Versions")
	assert.Contains(t, body, "- "+fxVersion)
	assert.Contains(t, body, "- \\"+fxRange)
	assert.Contains(t, body, "acknowledge reports within "+fxWindow)
	assert.Contains(t, body, "## Acknowledged Vulnerabilities")
	assert.Contains(t, body, "| "+fxVuln+" | "+fxVulnRzn+" |")
	assert.Contains(t, body, "## Bug Bounty")
	assert.Contains(t, body, fxBounty)
	assert.Contains(t, body, "gpg --keyserver keys.openpgp.org --recv-keys "+fxPGPKey)
	assert.Contains(t, body, "gpg --import pubkey.asc   # downloaded from "+fxPGPURL)
	assert.Contains(t, body, fxFP)
	assert.Contains(t, body, core.MarkerInner, "every generated file carries the sentinel")
}

// A support section the namespace never declared is not rendered.
func TestRenderOmitsUnsetSections(t *testing.T) {
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: fxName, Title: &projectfile.LocalizedString{Bare: fxTitle}}}
	projectfile.SetExtension(pf, pfmodel.SecurityExtensionNS, map[string]any{keyReport: fxReport})
	pointUserConfigAt(t, nil)
	out, err := Bridge{}.Render(pf, core.Options{})
	require.NoError(t, err)
	body := string(out.Files[core.FileSecurity])
	assert.NotContains(t, body, "## Supported Versions", "no versions declared, no list")
	assert.NotContains(t, body, "## Acknowledged Vulnerabilities", "nothing suppressed, no table")
	assert.NotContains(t, body, "## Encrypting a Report", "no key declared, no pgp block")
	assert.Contains(t, body, "does not currently run a bug bounty programme")
	assert.Contains(t, body, "within 30 days", "the window falls back to the template default")
}

// ── fingerprint derive ──────────────────────────────────────────────────────

// Offline with an empty cache derives nothing, and the render still succeeds: the verify block is omitted.
func TestRenderDerivesNoFingerprintOffline(t *testing.T) {
	pf := fullDoc()
	setFingerprint(t, pf, "")
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	pointUserConfigAt(t, nil)
	out, err := Bridge{}.Render(pf, core.Options{Offline: true})
	require.NoError(t, err)
	assert.NotContains(t, string(out.Files[core.FileSecurity]), "Verify the fingerprint matches",
		"the derive found nothing offline, so the verify block is omitted")
	assert.Contains(t, string(out.Files[core.FileSecurity]), "## Encrypting a Report", "the declared key still renders")
}

// ── error and failure paths ─────────────────────────────────────────────────

// A scalar namespace is not a map, and the getter's error reaches the caller.
func TestRenderFailsOnAScalarSecurityNamespace(t *testing.T) {
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: fxName}}
	projectfile.SetExtension(pf, pfmodel.SecurityExtensionNS, "not a map")
	pointUserConfigAt(t, nil)
	_, err := Bridge{}.Render(pf, core.Options{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), pfmodel.SecurityExtensionNS)
}

// ── localized variants ──────────────────────────────────────────────────────

// es and uk ship embedded, so each declared language renders its own prose and rebased links.
func TestRenderLocalizedVariants(t *testing.T) {
	pointUserConfigAt(t, nil)
	out, err := Bridge{}.Render(withLangs(securityDoc(), "es", "uk"), core.Options{})
	require.NoError(t, err)
	assert.Contains(t, out.Files, core.FileSecurity)
	assert.Contains(t, string(out.Files["docs/es/SECURITY.md"]), "# Política de seguridad")
	assert.Contains(t, string(out.Files["docs/es/SECURITY.md"]), "**<"+fxContact+">**")
	assert.Contains(t, string(out.Files["docs/uk/SECURITY.md"]), "# Політика безпеки")
	assert.Contains(t, string(out.Files["docs/uk/SECURITY.md"]), "[English](../../SECURITY.md)")
}

// The See also section is entirely project-authored: one link tagged security, no others.
func TestRenderAppendsTheTaggedSeeAlsoLinks(t *testing.T) {
	pf := securityDoc()
	pf.Links = []projectfile.Link{{
		Type:  projectfile.LinkDocumentation,
		URL:   "https://example.org/disclosure",
		Label: &projectfile.LocalizedString{Bare: "Disclosure log"},
		Extra: map[string]any{"tags": []any{"security"}},
	}}
	pointUserConfigAt(t, nil)
	out, err := Bridge{}.Render(pf, core.Options{})
	require.NoError(t, err)
	body := string(out.Files[core.FileSecurity])
	assert.Contains(t, body, "## See also")
	assert.Contains(t, body, "- [Disclosure log](https://example.org/disclosure)")
}

// An untagged document grows no orphan heading, even with links[] declared.
func TestRenderOmitsSeeAlsoWithoutATaggedLink(t *testing.T) {
	pf := securityDoc()
	pf.Links = []projectfile.Link{{Type: projectfile.LinkHomepage, URL: "https://example.org"}}
	pointUserConfigAt(t, nil)
	out, err := Bridge{}.Render(pf, core.Options{})
	require.NoError(t, err)
	assert.NotContains(t, string(out.Files[core.FileSecurity]), "## See also")
}

// ── fixtures ────────────────────────────────────────────────────────────────

// fullDoc declares every optional field the namespace carries, so one document reaches the whole render.
func fullDoc() *projectfile.Document {
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: fxName, Title: &projectfile.LocalizedString{Bare: fxTitle}}}
	projectfile.SetExtension(pf, pfmodel.SecurityExtensionNS, map[string]any{
		keyContact:           fxContact,
		keyReport:            fxReport,
		"supported-versions": []any{fxVersion, fxRange},
		"disclosure-window":  fxWindow,
		"gpg-key":            fxPGPKey,
		"gpg-fingerprint":    fxFP,
		"bug-bounty-url":     fxBounty,
	})
	projectfile.SetExtension(pf, pfmodel.VulnerabilitiesExtensionNS, map[string]any{
		"suppress": []any{map[string]any{"id": fxVuln, keyReason: fxVulnRzn}},
	})
	pf.Links = []projectfile.Link{{Type: "pgp-key", URL: fxPGPURL}}
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

// setFingerprint rewrites the namespace's gpg-fingerprint in place, leaving every other field alone.
func setFingerprint(t *testing.T, pf *projectfile.Document, fp string) {
	t.Helper()
	ext, err := pfmodel.GetSecurityExtension(pf)
	require.NoError(t, err)
	ext.GPGFingerprint = fp
	projectfile.SetExtension(pf, pfmodel.SecurityExtensionNS, extensionToMap(ext))
}
