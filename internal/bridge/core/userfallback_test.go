// SPDX-FileCopyrightText: 2026 Damián Búho <damian.buho@proton.me>
//
// SPDX-License-Identifier: MIT

package core_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"kiota.ch/projectfile/core/v2/pkg/userconfig"
	"projectfile.org/projectfile/bridge/v4/internal/bridge/core"
	"projectfile.org/projectfile/bridge/v4/internal/pfmodel"
)

// pointUserConfigAt writes ext as cli.json under a temp $XDG_CONFIG_HOME and drops the cache.
func pointUserConfigAt(t *testing.T, ext map[string]any) {
	t.Helper()
	dir := t.TempDir()
	if ext != nil {
		body, err := json.Marshal(ext)
		require.NoError(t, err)
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "projectfile"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "projectfile", "cli.json"), body, 0o644))
	}
	t.Setenv("XDG_CONFIG_HOME", dir)
	userconfig.SetIgnored(false)
	t.Cleanup(func() { userconfig.SetIgnored(true) })
}

// fixture funding URLs, each named once so goconst sees a single literal.
const keySupport = "support"

const (
	fxPatreon    = "https://patreon.test/fixture"
	fxKoFi       = "https://ko-fi.test/fixture"
	fxLiberapay  = "https://liberapay.test/fixture"
	fxTidelift   = "https://tidelift.test/fixture"
	fxBridge     = "https://bridge.test/fixture"
	fxIssueHunt  = "https://issuehunt.test/fixture"
	fxCollective = "https://collective.test/fixture"
	fxLFX        = "https://lfx.test/fixture"
	fxPolar      = "https://polar.test/fixture"
	fxCoffee     = "https://coffee.test/fixture"
	fxThanksDev  = "https://thanks.test/fixture"
	fxCustom     = "https://custom.test/fixture"
	fxGitHub     = "fixture-user"
)

// All five fallbacks are safe on a nil extension.
func TestUserFallbacksIgnoreNilExtensions(t *testing.T) {
	pointUserConfigAt(t, nil)

	assert.NotPanics(t, func() {
		core.ApplyUserFundingFallback(nil)
		core.ApplyUserSecurityFallback(nil)
		core.ApplyUserContributingFallback(nil)
		core.ApplyUserConventionsFallback(nil)
		core.ApplyUserSupportFallback(nil)
	})
}

// The funding fallback fills every channel field from the user config, lists included.
func TestUserFundingFallbackGapFillsEveryChannel(t *testing.T) {
	pointUserConfigAt(t, map[string]any{"funding": map[string]any{
		"github":           []string{fxGitHub},
		"patreon":          fxPatreon,
		"ko-fi":            fxKoFi,
		"liberapay":        fxLiberapay,
		"tidelift":         fxTidelift,
		"community-bridge": fxBridge,
		"issuehunt":        fxIssueHunt,
		"open-collective":  fxCollective,
		"lfx-crowdfunding": fxLFX,
		"polar":            fxPolar,
		"buy-me-a-coffee":  fxCoffee,
		"thanks-dev":       fxThanksDev,
		"custom":           []string{fxCustom},
	}})

	ext := &pfmodel.FundingExtension{}
	core.ApplyUserFundingFallback(ext)

	assert.Equal(t, []string{fxGitHub}, ext.GitHub)
	assert.Equal(t, fxPatreon, ext.Patreon)
	assert.Equal(t, fxKoFi, ext.KoFi)
	assert.Equal(t, fxLiberapay, ext.Liberapay)
	assert.Equal(t, fxTidelift, ext.Tidelift)
	assert.Equal(t, fxBridge, ext.CommunityBridge)
	assert.Equal(t, fxIssueHunt, ext.IssueHunt)
	assert.Equal(t, fxCollective, ext.OpenCollective)
	assert.Equal(t, fxLFX, ext.LFXCrowdfunding)
	assert.Equal(t, fxPolar, ext.Polar)
	assert.Equal(t, fxCoffee, ext.BuyMeACoffee)
	assert.Equal(t, fxThanksDev, ext.ThanksDev)
	assert.Equal(t, []string{fxCustom}, ext.Custom)
}

// A populated in-pf value always wins: the fallback gap-fills, it never overwrites what the project declared.
func TestUserFundingFallbackNeverOverwritesDeclaredValues(t *testing.T) {
	pointUserConfigAt(t, map[string]any{"funding": map[string]any{
		"github":  []string{fxGitHub},
		"patreon": fxPatreon,
		"custom":  []string{fxCustom},
	}})

	ext := &pfmodel.FundingExtension{
		GitHub:  []string{"project-user"},
		Patreon: "https://patreon.test/project",
		Custom:  []string{"https://custom.test/project"},
	}
	core.ApplyUserFundingFallback(ext)

	assert.Equal(t, []string{"project-user"}, ext.GitHub)
	assert.Equal(t, "https://patreon.test/project", ext.Patreon)
	assert.Equal(t, []string{"https://custom.test/project"}, ext.Custom)
}

// The security fallback crosses sections: contact and GPG key come from [identity].
func TestUserSecurityFallbackCrossFillsFromIdentityAndSecurity(t *testing.T) {
	pointUserConfigAt(t, map[string]any{
		"identity": map[string]any{"email": "fixture@example.test", "gpg-key": "FIXTUREKEY"},
		"security": map[string]any{
			"report-url":        "https://report.test",
			"disclosure-window": "90 days",
			"bug-bounty-url":    "https://bounty.test",
		},
	})

	ext := &pfmodel.SecurityExtension{}
	core.ApplyUserSecurityFallback(ext)

	assert.Equal(t, "fixture@example.test", ext.Contact)
	assert.Equal(t, "FIXTUREKEY", ext.GPGKey)
	assert.Equal(t, "https://report.test", ext.ReportURL)
	assert.Equal(t, "90 days", ext.DisclosureWindow)
	assert.Equal(t, "https://bounty.test", ext.BugBountyURL)

	ext.Contact = "security@example.test"
	core.ApplyUserSecurityFallback(ext)
	assert.Equal(t, "security@example.test", ext.Contact, "a declared contact must survive")
}

// Contributing, conventions and support gap-fill their per-user URLs; per-project shape has none.
func TestUserFallbacksFillTheirOwnSections(t *testing.T) {
	pointUserConfigAt(t, map[string]any{
		"contributing": map[string]any{"cla-url": "https://cla.test", "chat-url": "https://chat.test"},
		"conventions": map[string]any{
			"commit-style":    "conventional",
			"workflow":        "trunk",
			"style-guide-url": "https://style.test",
		},
		keySupport: map[string]any{"response-time": "48 hours"},
	})

	contrib := &pfmodel.ContributingExtension{}
	core.ApplyUserContributingFallback(contrib)
	assert.Equal(t, "https://cla.test", contrib.CLAURL)
	assert.Equal(t, "https://chat.test", contrib.ChatURL)

	conv := &pfmodel.ConventionsExtension{}
	core.ApplyUserConventionsFallback(conv)
	assert.Equal(t, "conventional", conv.CommitStyle)
	assert.Equal(t, "trunk", conv.Workflow)
	assert.Equal(t, "https://style.test", conv.StyleGuideURL)

	support := &pfmodel.SupportExtension{}
	core.ApplyUserSupportFallback(support)
	assert.Equal(t, "48 hours", support.ResponseTime)
}
