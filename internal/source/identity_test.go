// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package source

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/userconfig"
)

// userConfigFile points userconfig.Load at a temp XDG tree and empties the process-wide load cache afterwards.
func userConfigFile(t *testing.T, body string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	if body != "" {
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "projectfile"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "projectfile", "cli.toml"), []byte(body), 0o644))
	}
	userconfig.SetIgnored(false)
	t.Cleanup(func() { userconfig.SetIgnored(true) })
	got := userconfig.Load().Identity.Email
	if body == "" {
		assert.Empty(t, got, "an absent config file yields no identity")
		return
	}
	assert.NotEmpty(t, got, "the fixture config must be what the loader reads")
}

func TestApplyUserIdentityNoConfig(t *testing.T) {
	userConfigFile(t, "")

	ApplyUserIdentity(nil)
	ApplyUserIdentity(&Partial{})
	ApplyUserIdentity(&Partial{People: []PersonEntry{{Email: fxEmailA}}})
	assert.True(t, true, "a missing user config detaches nothing and panics nowhere")
}

// TestApplyUserIdentityFillsEveryPersonField pins the person branch, filled gap-only on a case-insensitive email match.
func TestApplyUserIdentityFillsEveryPersonField(t *testing.T) {
	userConfigFile(t, `[identity]
email = "A@Example.TEST"
family-names = "Búho"
given-names = "Damián"
orcid = "`+fxOrcidUser+`"
url = "`+fxUserURL+`"
affiliation = "`+fxAffiliation+`"
`)

	p := &Partial{People: []PersonEntry{
		{Email: fxEmailA, Roles: []string{fxRoleAuthor}},
		{Email: fxEmailB, Roles: []string{fxRoleContrib}},
	}}
	ApplyUserIdentity(p)

	assert.Equal(t, "Búho", p.People[0].FamilyNames)
	assert.Equal(t, "Damián", p.People[0].GivenNames)
	assert.Equal(t, fxOrcidUser, p.People[0].Orcid)
	assert.Equal(t, fxUserURL, p.People[0].URL)
	assert.Equal(t, fxAffiliation, p.People[0].Affiliation)

	assert.Empty(t, p.People[1].FamilyNames, "a non-matching email is left untouched")
	assert.Empty(t, p.People[1].Orcid)
}

// TestApplyUserIdentityGapFillOnly pins that a value a source already produced is never replaced.
func TestApplyUserIdentityGapFillOnly(t *testing.T) {
	userConfigFile(t, `[identity]
email = "`+fxEmailA+`"
family-names = "Búho"
given-names = "Damián"
orcid = "`+fxOrcidUser+`"
url = "`+fxUserURL+`"
affiliation = "`+fxAffiliation+`"
`)

	p := &Partial{People: []PersonEntry{{
		Email:       fxEmailA,
		FamilyNames: fxFamilyLovelace,
		GivenNames:  fxGivenAda,
		Orcid:       fxOrcidBare,
		URL:         "https://ada.example.test",
		Affiliation: "Royal Society",
	}}}
	before := p.People[0]

	ApplyUserIdentity(p)
	assert.Equal(t, before, p.People[0], "every populated field survives the identity attach")
}

// TestApplyUserIdentityEntityBranch pins the organisation branch, where person-only fields stay suppressed.
func TestApplyUserIdentityEntityBranch(t *testing.T) {
	userConfigFile(t, `[identity]
is-entity = true
email = "`+fxEmailA+`"
name = "`+fxEntityName+`"
affiliation = "`+fxAffiliation+`"
orcid = "`+fxOrcidUser+`"
`)

	p := &Partial{People: []PersonEntry{{Email: fxEmailA}}}
	ApplyUserIdentity(p)

	assert.Equal(t, fxEntityName, p.People[0].Name)
	assert.Empty(t, p.People[0].FamilyNames, "an entity has no family-names")
	assert.Empty(t, p.People[0].GivenNames, "an entity has no given-names")
	assert.Empty(t, p.People[0].Affiliation, "affiliation is person-only per spec §5.5")
	assert.Equal(t, fxOrcidUser, p.People[0].Orcid, "orcid is still shared")
}

// TestApplyUserIdentityTrimsEmail pins the whitespace trim on both the config and the person entry.
func TestApplyUserIdentityTrimsEmail(t *testing.T) {
	userConfigFile(t, `[identity]
email = "  `+fxEmailA+`  "
family-names = "Búho"
`)

	p := &Partial{People: []PersonEntry{{Email: " " + fxEmailA + " "}}}
	ApplyUserIdentity(p)
	assert.Equal(t, "Búho", p.People[0].FamilyNames, "both sides are trimmed before comparing")
}
