// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package security

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/projectfile"
	"kiota.ch/projectfile/core/v2/pkg/userconfig"
	"projectfile.org/projectfile/bridge/v4/internal/bridge/core"
	"projectfile.org/projectfile/bridge/v4/internal/pfmodel"
)

// Literals three or more test files repeat, hoisted so goconst stops counting them.
const (
	fxName    = "widget"
	fxTitle   = "Widget"
	fxContact = "security@example.org"
	fxMaint   = "maint@example.org"
	fxReport  = "https://example.org/report"
	fxBounty  = "https://example.org/bounty"
	fxWindow  = "90 days"
	fxVersion = "1.0"
	fxRange   = ">= 2.0"
	fxVuln    = "CVE-2026-0001"
	fxVulnRzn = "upstream fix pending"
	fxPGPKey  = "B64C122EE16C3746"
	fxPGPURL  = "https://example.org/pubkey.asc"
	fxFP      = "6F19 7084 3C9E 8406 AD70  0467 B64C 122E E16C 3746"

	nestedRel = "docs/SECURITY.md"
	keyPath   = "path"
	xdgApp    = "projectfile"
	cliFile   = "cli.json"
	xdgEnv    = "XDG_CONFIG_HOME"

	// Namespace map keys three or more fixtures spell, hoisted so goconst stops counting them.
	keyContact = "contact"
	keyReport  = "report-url"
	keyReason  = "reason"
)

// ── identity surface ────────────────────────────────────────────────────────

func TestBridgeIdentity(t *testing.T) {
	b := Bridge{}
	assert.Equal(t, "security", b.Name())
	label, provider := b.Labels()
	assert.Equal(t, core.FileSecurity, label)
	assert.Equal(t, "projectfile", provider)
	assert.True(t, b.Policy().Marker, "Markdown carries a comment, so the sentinel folds into the header")
}

func TestBridgeExists(t *testing.T) {
	dir := t.TempDir()
	pointUserConfigAt(t, nil)
	assert.False(t, Bridge{}.Exists(dir), "an empty directory holds no security policy")
	require.NoError(t, os.WriteFile(filepath.Join(dir, core.FileSecurity), []byte("# Policy"), 0o644))
	assert.True(t, Bridge{}.Exists(dir), "the default path is the probed one")
	require.NoError(t, os.Remove(filepath.Join(dir, core.FileSecurity)))
	assert.False(t, Bridge{}.Exists(dir), "a removed file reads absent again")
}

// The user-config path override relocates the probe, so a nested file is what Exists sees.
func TestBridgeExistsFollowsTheUserConfigPath(t *testing.T) {
	dir := t.TempDir()
	pointUserConfigAt(t, map[string]any{"generate": map[string]any{"defaults": map[string]any{core.FileSecurity: map[string]any{keyPath: nestedRel}}}})
	assert.False(t, Bridge{}.Exists(dir))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "docs"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, nestedRel), []byte("# Policy"), 0o644))
	assert.True(t, Bridge{}.Exists(dir), "the override path is the probed one")
}

func TestBridgeFullPathResolvesTheUserConfigPath(t *testing.T) {
	dir := t.TempDir()
	pointUserConfigAt(t, nil)
	assert.Equal(t, filepath.Join(dir, core.FileSecurity), Bridge{}.FullPath(dir, nil))
	pointUserConfigAt(t, map[string]any{"generate": map[string]any{"defaults": map[string]any{core.FileSecurity: map[string]any{keyPath: nestedRel}}}})
	assert.Equal(t, filepath.Join(dir, nestedRel), Bridge{}.FullPath(dir, nil), "the user config outranks the built-in default")
}

// ── contact resolution ──────────────────────────────────────────────────────

func TestContactEmailPrefersTheSecurityNamespace(t *testing.T) {
	email, src := contactEmail(securityDoc(), projectfile.RoleSecurity)
	assert.Equal(t, fxContact, email)
	assert.Equal(t, "["+pfmodel.SecurityExtensionNS+"].contact", src)
}

// With no security namespace contact the people roles decide: security, then maintainer, then any mailbox.
func TestContactEmailFallsBackThroughThePeopleRoles(t *testing.T) {
	doc := &projectfile.Document{}
	doc.People = []projectfile.Person{
		{Email: fxMaint, Roles: []string{projectfile.RoleMaintainer}},
		{Email: fxContact, Roles: []string{projectfile.RoleSecurity}},
	}
	email, src := contactEmail(doc, projectfile.RoleSecurity)
	assert.Equal(t, fxContact, email)
	assert.Contains(t, src, projectfile.RoleSecurity)

	doc.People = []projectfile.Person{{Email: fxMaint, Roles: []string{projectfile.RoleMaintainer}}, {Email: fxContact}}
	email, _ = contactEmail(doc, projectfile.RoleSecurity)
	assert.Equal(t, fxMaint, email, "a maintainer beats an unroled person")
}

func TestContactEmailIsEmptyWithoutAnyContact(t *testing.T) {
	email, src := contactEmail(&projectfile.Document{}, projectfile.RoleSecurity)
	assert.Empty(t, email)
	assert.Empty(t, src)
}

// ── namespace writer ────────────────────────────────────────────────────────

func TestExtensionToMapDropsEmptyFields(t *testing.T) {
	assert.Empty(t, extensionToMap(&pfmodel.SecurityExtension{}))
}

func TestExtensionToMapCarriesEveryDeclaredField(t *testing.T) {
	m := extensionToMap(&pfmodel.SecurityExtension{
		Contact:           fxContact,
		ReportURL:         fxReport,
		SupportedVersions: []string{fxVersion},
		DisclosureWindow:  fxWindow,
		GPGKey:            "B64C122EE16C3746",
		GPGFingerprint:    "6F19 7084 3C9E 8406 AD70",
		BugBountyURL:      fxBounty,
	})
	assert.Equal(t, map[string]any{
		keyContact:           fxContact,
		keyReport:            fxReport,
		"supported-versions": []any{fxVersion},
		"disclosure-window":  fxWindow,
		"gpg-key":            "B64C122EE16C3746",
		"gpg-fingerprint":    "6F19 7084 3C9E 8406 AD70",
		"bug-bounty-url":     fxBounty,
	}, m)
}

// ── decision-trace helpers ──────────────────────────────────────────────────

func TestValueOrEmpty(t *testing.T) {
	assert.Equal(t, "(unset, omitted)", valueOrEmpty(""))
	assert.Equal(t, fxContact, valueOrEmpty(fxContact))
}

// A version range must not start a Markdown blockquote, so a leading comparison operator is escaped.
func TestMdSafeVersions(t *testing.T) {
	assert.Nil(t, mdSafeVersions(nil))
	assert.Equal(t, []string{`\` + fxRange, fxVersion, "1 <x"}, mdSafeVersions([]string{fxRange, fxVersion, "1 <x"}))
}

// ── acknowledged vulnerabilities ────────────────────────────────────────────

func TestAcknowledgedVulns(t *testing.T) {
	assert.Nil(t, acknowledgedVulns(&projectfile.Document{}), "a document with no vulnerabilities namespace names nothing")

	pf := &projectfile.Document{}
	projectfile.SetExtension(pf, pfmodel.VulnerabilitiesExtensionNS, map[string]any{"suppress": []any{
		fxVuln,
		map[string]any{"id": fxVuln, keyReason: fxVulnRzn},
		map[string]any{keyReason: "no id at all"},
		"",
	}})
	got := acknowledgedVulns(pf)
	require.Len(t, got, 2, "a bare id and a mapped pair render; a blank id is dropped")
	assert.Equal(t, fxVuln, got[0].ID)
	assert.Empty(t, got[0].Reason, "the string shorthand carries no reason")
	assert.Equal(t, fxVulnRzn, got[1].Reason)
}

// ── required fields ─────────────────────────────────────────────────────────

func TestRequiredFieldsIsSilentWhenAContactExists(t *testing.T) {
	assert.Nil(t, Bridge{}.RequiredFields(securityDoc()), "a declared contact closes the prompt")
}

func TestRequiredFieldsIsSilentOnTheUserConfigMailbox(t *testing.T) {
	pointUserConfigAt(t, map[string]any{"identity": map[string]any{"email": fxContact}})
	assert.Nil(t, Bridge{}.RequiredFields(&projectfile.Document{}), "the per-user mailbox answers for the project")
}

func TestRequiredFieldsPromptsForTheContact(t *testing.T) {
	pointUserConfigAt(t, nil)
	missing := Bridge{}.RequiredFields(&projectfile.Document{})
	require.Len(t, missing, 1)
	assert.Equal(t, "["+pfmodel.SecurityExtensionNS+"].contact", missing[0].Field)
	assert.Contains(t, missing[0].Hint, "security reports")
	require.NotNil(t, missing[0].Setter)
}

// The setter writes back through the extension surface, so the fill closes the prompt.
func TestRequiredFieldsSetterWritesTheContact(t *testing.T) {
	pointUserConfigAt(t, nil)
	pf := &projectfile.Document{}
	require.NoError(t, Bridge{}.RequiredFields(pf)[0].Setter(fxContact))
	assert.Nil(t, Bridge{}.RequiredFields(pf), "the filled contact closes the prompt")
	email, _ := contactEmail(pf, projectfile.RoleSecurity)
	assert.Equal(t, fxContact, email)
}

// A pre-existing namespace keeps its other fields when the setter fills the contact.
func TestRequiredFieldsSetterPreservesTheOtherFields(t *testing.T) {
	pointUserConfigAt(t, nil)
	pf := &projectfile.Document{}
	projectfile.SetExtension(pf, pfmodel.SecurityExtensionNS, map[string]any{keyReport: fxReport})
	require.NoError(t, Bridge{}.RequiredFields(pf)[0].Setter(fxContact))
	ext, err := pfmodel.GetSecurityExtension(pf)
	require.NoError(t, err)
	assert.Equal(t, fxContact, ext.Contact)
	assert.Equal(t, fxReport, ext.ReportURL, "the sibling field survives the rewrite")
}

func TestRequiredFieldsSetterRejectsABlankValue(t *testing.T) {
	pointUserConfigAt(t, nil)
	setter := Bridge{}.RequiredFields(&projectfile.Document{})[0].Setter
	err := setter("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "security contact is required")
}

// ── fixtures ────────────────────────────────────────────────────────────────

// securityDoc is a document whose contact arrives from the security namespace, the resolved-first tier.
func securityDoc() *projectfile.Document {
	pf := &projectfile.Document{Identity: projectfile.Identity{Name: fxName, Title: &projectfile.LocalizedString{Bare: fxTitle}}}
	projectfile.SetExtension(pf, pfmodel.SecurityExtensionNS, map[string]any{keyContact: fxContact})
	return pf
}

// pointUserConfigAt aims $XDG_CONFIG_HOME at a temp tree holding ext as cli.json and drops the load cache.
func pointUserConfigAt(t *testing.T, ext map[string]any) {
	t.Helper()
	dir := t.TempDir()
	if ext != nil {
		body, err := json.Marshal(ext)
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(filepath.Join(dir, xdgApp), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, xdgApp, cliFile), body, 0o644))
	}
	t.Setenv(xdgEnv, dir)
	userconfig.SetIgnored(false)
	t.Cleanup(func() { userconfig.SetIgnored(true) })
}
